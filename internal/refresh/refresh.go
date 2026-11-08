// Package refresh keeps feeds up to date in the background.
package refresh

import (
	"context"
	"log/slog"
	"net/url"
	"sync"
	"time"

	"github.com/dbohry/huginn/internal/feed"
	"github.com/dbohry/huginn/internal/store"
)

const (
	// DefaultInterval is how often each feed is fetched.
	DefaultInterval = 30 * time.Minute
	// MaxBackoff is the longest wait after repeated failures.
	MaxBackoff = 12 * time.Hour

	defaultWorkers = 4
	batchSize      = 200
	maxErrorLength = 300
)

// Service fetches feeds and stores their articles.
type Service struct {
	Store    *store.Store
	Fetcher  *feed.Fetcher
	Interval time.Duration
	Workers  int
	Log      *slog.Logger
	Now      func() time.Time // replaced in tests

	mu    sync.Mutex
	hosts map[string]*sync.Mutex
	// How many fetches of each feed are running, and how many are waiting
	// their turn in a refresh of every feed, so the interface can show them.
	fetching map[int64]int
	queued   map[int64]int
}

// Feed refresh states, as Status reports them.
const (
	StatusIdle     = ""
	StatusQueued   = "queued"
	StatusFetching = "fetching"
)

// Status says whether a feed is being fetched right now, is waiting to be
// fetched in a refresh of every feed, or neither.
func (s *Service) Status(id int64) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.fetching[id] > 0:
		return StatusFetching
	case s.queued[id] > 0:
		return StatusQueued
	}
	return StatusIdle
}

// adjust adds d to a feed's count in m, forgetting the feed at zero.
func (s *Service) adjust(m *map[int64]int, id int64, d int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if *m == nil {
		*m = map[int64]int{}
	}
	if (*m)[id] += d; (*m)[id] <= 0 {
		delete(*m, id)
	}
}

// RefreshFeeds fetches the given feeds one after another. They all count as
// queued from the start, so the interface can show the ones still to come.
// Failures are recorded on each feed.
func (s *Service) RefreshFeeds(ctx context.Context, ids []int64) {
	for _, id := range ids {
		s.adjust(&s.queued, id, 1)
	}
	for _, id := range ids {
		if ctx.Err() == nil {
			s.RefreshFeed(ctx, id)
		}
		s.adjust(&s.queued, id, -1)
	}
}

// New returns a Service with the default interval.
func New(st *store.Store, f *feed.Fetcher, log *slog.Logger) *Service {
	return &Service{Store: st, Fetcher: f, Interval: DefaultInterval, Workers: defaultWorkers, Log: log, Now: time.Now}
}

// Backoff is how long to wait before retrying a feed that has failed errors
// times in a row: the interval doubled for each failure, up to MaxBackoff.
func Backoff(interval time.Duration, errors int) time.Duration {
	d := interval
	for i := 0; i < errors && d < MaxBackoff; i++ {
		d *= 2
	}
	if d > MaxBackoff {
		d = MaxBackoff
	}
	return d
}

// RefreshFeed fetches one feed now and stores the result. A fetch failure is
// recorded on the feed and also returned.
func (s *Service) RefreshFeed(ctx context.Context, id int64) error {
	s.adjust(&s.fetching, id, 1)
	defer s.adjust(&s.fetching, id, -1)
	f, err := s.Store.GetFeed(ctx, id)
	if err != nil {
		return err
	}

	unlock := s.lockHost(f.URL)
	res, fetchErr := s.Fetcher.Fetch(ctx, f.URL, f.ETag, f.LastModified)
	unlock()

	now := s.Now()
	if fetchErr != nil {
		msg := fetchErr.Error()
		if len(msg) > maxErrorLength {
			msg = msg[:maxErrorLength]
		}
		next := now.Add(Backoff(s.Interval, f.ErrorCount+1))
		if err := s.Store.RecordFailure(ctx, id, now, next, msg); err != nil {
			return err
		}
		return fetchErr
	}

	if res.NotModified {
		return s.Store.RecordSuccess(ctx, id, now, now.Add(s.Interval), res.ETag, res.LastModified, "", "")
	}
	if err := s.Store.UpsertItems(ctx, id, res.Feed.Items, now); err != nil {
		return err
	}
	return s.Store.RecordSuccess(ctx, id, now, now.Add(s.Interval), res.ETag, res.LastModified, res.Feed.Title, res.Feed.SiteURL)
}

// RefreshDue fetches every feed that is due, a few at a time, never two at once
// from the same host. It returns how many feeds it tried.
func (s *Service) RefreshDue(ctx context.Context) int {
	due, err := s.Store.DueFeeds(ctx, s.Now(), batchSize)
	if err != nil {
		if ctx.Err() == nil {
			s.Log.Error("list due feeds", "err", err)
		}
		return 0
	}

	workers := s.Workers
	if workers < 1 {
		workers = 1
	}
	jobs := make(chan store.Feed)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range jobs {
				if err := s.RefreshFeed(ctx, f.ID); err != nil && ctx.Err() == nil {
					s.Log.Warn("refresh feed", "feed", f.URL, "err", err)
				}
			}
		}()
	}
	for _, f := range due {
		if ctx.Err() != nil {
			break
		}
		jobs <- f
	}
	close(jobs)
	wg.Wait()
	return len(due)
}

// Run checks for due feeds once a minute until ctx ends.
func (s *Service) Run(ctx context.Context) {
	s.RefreshDue(ctx)
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			s.RefreshDue(ctx)
		}
	}
}

// lockHost serializes fetches to one host so a site with many feeds is not hit
// in parallel. It returns the function that releases the lock.
func (s *Service) lockHost(rawURL string) func() {
	host := rawURL
	if u, err := url.Parse(rawURL); err == nil {
		host = u.Host
	}
	s.mu.Lock()
	if s.hosts == nil {
		s.hosts = map[string]*sync.Mutex{}
	}
	m, ok := s.hosts[host]
	if !ok {
		m = &sync.Mutex{}
		s.hosts[host] = m
	}
	s.mu.Unlock()

	m.Lock()
	return m.Unlock
}
