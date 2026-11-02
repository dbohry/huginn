package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dbohry/huginn/internal/feed"
	"github.com/dbohry/huginn/internal/store"
)

// addFeedTimeout bounds how long adding a feed waits for the first fetch. It is
// shorter than the server's write timeout.
const addFeedTimeout = 25 * time.Second

// feedJSON is a feed as the interface sees it.
type feedJSON struct {
	ID      int64  `json:"id"`
	URL     string `json:"url"`
	Title   string `json:"title"`
	SiteURL string `json:"siteUrl"`
	Unread  int    `json:"unread"`
	// LastFetchedAt is Unix seconds, or null if the feed was never fetched.
	LastFetchedAt *int64 `json:"lastFetchedAt"`
	// Error is the last fetch error, or empty when the last fetch worked.
	Error string `json:"error"`
}

func toFeedJSON(f store.FeedSummary) feedJSON {
	out := feedJSON{ID: f.ID, URL: f.URL, Title: feedName(f.Title, f.URL), SiteURL: f.SiteURL, Unread: f.Unread, Error: f.LastError}
	if f.LastFetchedAt != nil {
		t := f.LastFetchedAt.Unix()
		out.LastFetchedAt = &t
	}
	return out
}

// feedName is the title to show for a feed: its own title, else its host.
func feedName(title, rawURL string) string {
	if title = strings.TrimSpace(title); title != "" {
		return title
	}
	if u, err := url.Parse(rawURL); err == nil && u.Host != "" {
		return u.Host
	}
	return rawURL
}

func (s *server) listFeeds(w http.ResponseWriter, r *http.Request, user store.User, _ store.Session) {
	feeds, err := s.store.ListFeeds(r.Context(), user.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	out := make([]feedJSON, len(feeds))
	for i, f := range feeds {
		out[i] = toFeedJSON(f)
	}
	writeJSON(w, http.StatusOK, map[string][]feedJSON{"feeds": out})
}

// addFeed subscribes the user to a feed. It fetches the feed first, so a bad
// address is refused with a reason instead of being added and left failing.
func (s *server) addFeed(w http.ResponseWriter, r *http.Request, user store.User, _ store.Session) {
	var in struct {
		URL string `json:"url"`
	}
	if !decode(w, r, &in) {
		return
	}
	address := strings.TrimSpace(in.URL)
	if address == "" {
		writeError(w, http.StatusBadRequest, "Enter the address of a feed.")
		return
	}
	if !strings.Contains(address, "://") {
		address = "https://" + address
	}
	if err := feed.ValidateURL(address); err != nil {
		writeError(w, http.StatusBadRequest, "That is not a web address.")
		return
	}
	if ok, retry := s.refreshByUser.Allow(strconv.FormatInt(user.ID, 10)); !ok {
		tooManyAttempts(w, retry)
		return
	}
	s.refreshByUser.Fail(strconv.FormatInt(user.ID, 10))

	ctx, cancel := context.WithTimeout(r.Context(), addFeedTimeout)
	defer cancel()
	id, err := s.subscribe(ctx, user.ID, address)
	if errors.Is(err, feed.ErrNotAFeed) {
		// A web page rather than a feed: use the feed it advertises, if any.
		if alt, derr := s.refresh.Fetcher.Discover(ctx, address); derr == nil && alt != "" && alt != address {
			id, err = s.subscribe(ctx, user.ID, alt)
		}
	}
	switch {
	case errors.Is(err, store.ErrExists):
		writeError(w, http.StatusConflict, "You already follow that feed.")
		return
	case err != nil && isFetchProblem(err):
		writeError(w, http.StatusUnprocessableEntity, "Could not add that feed. "+fetchMessage(err))
		return
	case err != nil:
		serverError(w, err)
		return
	}

	f, err := s.feedSummary(r.Context(), user.ID, id)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]feedJSON{"feed": toFeedJSON(f)})
}

// subscribe adds the feed, fetches it once and, if that fails, takes it out
// again. It returns the new feed's id.
func (s *server) subscribe(ctx context.Context, userID int64, address string) (int64, error) {
	id, err := s.store.CreateFeed(ctx, userID, address)
	if err != nil {
		return 0, err
	}
	if err := s.refresh.RefreshFeed(ctx, id); err != nil {
		// Use a fresh context: the request's may be the reason the fetch failed.
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.store.DeleteFeed(cleanup, userID, id)
		return 0, err
	}
	return id, nil
}

func (s *server) feedSummary(ctx context.Context, userID, id int64) (store.FeedSummary, error) {
	feeds, err := s.store.ListFeeds(ctx, userID)
	if err != nil {
		return store.FeedSummary{}, err
	}
	for _, f := range feeds {
		if f.ID == id {
			return f, nil
		}
	}
	return store.FeedSummary{}, store.ErrNotFound
}

func (s *server) deleteFeed(w http.ResponseWriter, r *http.Request, user store.User, _ store.Session) {
	id, ok := pathID(w, r, "Feed not found.")
	if !ok {
		return
	}
	switch err := s.store.DeleteFeed(r.Context(), user.ID, id); {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "Feed not found.")
	case err != nil:
		serverError(w, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// refreshFeed fetches one feed now.
func (s *server) refreshFeed(w http.ResponseWriter, r *http.Request, user store.User, _ store.Session) {
	id, ok := pathID(w, r, "Feed not found.")
	if !ok {
		return
	}
	if _, err := s.store.GetUserFeed(r.Context(), user.ID, id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Feed not found.")
		return
	} else if err != nil {
		serverError(w, err)
		return
	}
	if !s.allowRefresh(w, user.ID) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), addFeedTimeout)
	defer cancel()
	switch err := s.refresh.RefreshFeed(ctx, id); {
	case err != nil && isFetchProblem(err):
		writeError(w, http.StatusUnprocessableEntity, fetchMessage(err))
	case err != nil:
		serverError(w, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// refreshAll starts fetching all of the user's feeds in the background and
// returns at once. The interface reloads the feed list to see the result.
func (s *server) refreshAll(w http.ResponseWriter, r *http.Request, user store.User, _ store.Session) {
	if !s.allowRefresh(w, user.ID) {
		return
	}
	ids, err := s.store.UserFeedIDs(r.Context(), user.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	s.background(func(ctx context.Context) {
		for _, id := range ids {
			if ctx.Err() != nil {
				return
			}
			s.refresh.RefreshFeed(ctx, id) // failures are recorded on the feed
		}
	})
	writeJSON(w, http.StatusAccepted, map[string]int{"feeds": len(ids)})
}

func (s *server) allowRefresh(w http.ResponseWriter, userID int64) bool {
	key := strconv.FormatInt(userID, 10)
	if ok, retry := s.refreshByUser.Allow(key); !ok {
		tooManyAttempts(w, retry)
		return false
	}
	s.refreshByUser.Fail(key)
	return true
}

// background runs fn on its own goroutine with a generous time limit. Tests wait
// for it with s.bg.
func (s *server) background(fn func(ctx context.Context)) {
	s.bg.Add(1)
	go func() {
		defer s.bg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		fn(ctx)
	}()
}

// isFetchProblem is true for failures caused by the feed or its server rather
// than by Huginn.
func isFetchProblem(err error) bool {
	var status *feed.StatusError
	var netErr net.Error
	var urlErr *url.Error
	return errors.Is(err, feed.ErrNotAFeed) || errors.Is(err, feed.ErrTooLarge) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.As(err, &status) || errors.As(err, &netErr) || errors.As(err, &urlErr)
}

// fetchMessage explains a fetch failure without exposing internal detail.
func fetchMessage(err error) string {
	var status *feed.StatusError
	var netErr net.Error
	switch {
	case errors.Is(err, feed.ErrNotAFeed):
		return "That address is not an RSS, Atom or JSON feed."
	case errors.Is(err, feed.ErrTooLarge):
		return "That feed is too large."
	case errors.As(err, &status):
		return "The site answered \"" + status.Status + "\"."
	case errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()):
		return "The site took too long to answer."
	default:
		return "The site could not be reached."
	}
}

func pathID(w http.ResponseWriter, r *http.Request, notFound string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusNotFound, notFound)
		return 0, false
	}
	return id, true
}
