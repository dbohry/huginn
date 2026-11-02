// Package store is Huginn's data access layer over SQLite.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/dbohry/huginn/internal/feed"
)

// ErrNotFound is returned when a row does not exist (or is not visible to the
// caller).
var ErrNotFound = errors.New("not found")

// ErrExists is returned when a unique row already exists.
var ErrExists = errors.New("already exists")

// MaxItemsPerFeed is how many articles are kept for each feed. The oldest are
// removed after a refresh.
const MaxItemsPerFeed = 1000

// Store reads and writes Huginn's data.
type Store struct {
	db *sql.DB
}

// New returns a Store backed by db.
func New(db *sql.DB) *Store { return &Store{db: db} }

// Feed is a subscribed feed and its fetch state.
type Feed struct {
	ID            int64
	URL           string
	Title         string
	SiteURL       string
	ETag          string
	LastModified  string
	LastFetchedAt *time.Time
	LastError     string
	ErrorCount    int
	NextFetchAt   *time.Time
}

const feedColumns = `id, url, title, site_url, etag, last_modified, last_fetched_at, last_error, error_count, next_fetch_at`

type scanner interface{ Scan(dest ...any) error }

func scanFeed(row scanner) (Feed, error) {
	var f Feed
	var fetched, next sql.NullString
	err := row.Scan(&f.ID, &f.URL, &f.Title, &f.SiteURL, &f.ETag, &f.LastModified,
		&fetched, &f.LastError, &f.ErrorCount, &next)
	if err != nil {
		return Feed{}, err
	}
	f.LastFetchedAt = parseTime(fetched)
	f.NextFetchAt = parseTime(next)
	return f, nil
}

// GetFeed returns one feed by id.
func (s *Store) GetFeed(ctx context.Context, id int64) (Feed, error) {
	f, err := scanFeed(s.db.QueryRowContext(ctx, `SELECT `+feedColumns+` FROM feeds WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Feed{}, ErrNotFound
	}
	return f, err
}

// DueFeeds returns feeds that should be fetched at now: never fetched, or whose
// next fetch time has passed. The most overdue come first.
func (s *Store) DueFeeds(ctx context.Context, now time.Time, limit int) ([]Feed, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+feedColumns+` FROM feeds
		WHERE next_fetch_at IS NULL OR next_fetch_at <= ?
		ORDER BY next_fetch_at IS NOT NULL, next_fetch_at, id
		LIMIT ?`, formatTime(now), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Feed
	for rows.Next() {
		f, err := scanFeed(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// RecordSuccess stores the result of a fetch that worked (including "not
// modified"): the validators, the feed's title and site, and when to fetch next.
// An empty title or siteURL leaves the stored value alone.
func (s *Store) RecordSuccess(ctx context.Context, id int64, now, next time.Time, etag, lastModified, title, siteURL string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE feeds SET
		etag = ?, last_modified = ?,
		title = CASE WHEN ? != '' THEN ? ELSE title END,
		site_url = CASE WHEN ? != '' THEN ? ELSE site_url END,
		last_fetched_at = ?, last_error = '', error_count = 0, next_fetch_at = ?
		WHERE id = ?`,
		etag, lastModified, title, title, siteURL, siteURL, formatTime(now), formatTime(next), id)
	return err
}

// RecordFailure stores a failed fetch and when to try again.
func (s *Store) RecordFailure(ctx context.Context, id int64, now, next time.Time, message string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE feeds SET
		last_fetched_at = ?, last_error = ?, error_count = error_count + 1, next_fetch_at = ?
		WHERE id = ?`, formatTime(now), message, formatTime(next), id)
	return err
}

// UpsertItems adds new articles and updates ones the feed has edited. Read state
// is never touched. An article whose feed gives no date keeps the date it was
// first seen. Afterwards the feed is trimmed to MaxItemsPerFeed articles.
func (s *Store) UpsertItems(ctx context.Context, feedID int64, items []feed.Item, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	withDate, err := tx.PrepareContext(ctx, `INSERT INTO items
		(feed_id, guid, title, url, author, published_at, summary, content_html)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (feed_id, guid) DO UPDATE SET
			title = excluded.title, url = excluded.url, author = excluded.author,
			published_at = excluded.published_at,
			summary = excluded.summary, content_html = excluded.content_html`)
	if err != nil {
		return err
	}
	defer withDate.Close()
	noDate, err := tx.PrepareContext(ctx, `INSERT INTO items
		(feed_id, guid, title, url, author, published_at, summary, content_html)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (feed_id, guid) DO UPDATE SET
			title = excluded.title, url = excluded.url, author = excluded.author,
			summary = excluded.summary, content_html = excluded.content_html`)
	if err != nil {
		return err
	}
	defer noDate.Close()

	for _, it := range items {
		stmt, published := noDate, formatTime(now)
		if it.Published != nil {
			stmt, published = withDate, formatTime(*it.Published)
		}
		if _, err := stmt.ExecContext(ctx, feedID, it.GUID, it.Title, it.URL, it.Author, published, it.Summary, it.ContentHTML); err != nil {
			return fmt.Errorf("save item %q: %w", it.GUID, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM items WHERE feed_id = ? AND id NOT IN
		(SELECT id FROM items WHERE feed_id = ? ORDER BY published_at DESC, id DESC LIMIT ?)`,
		feedID, feedID, MaxItemsPerFeed); err != nil {
		return err
	}
	return tx.Commit()
}

// CreateFeed subscribes to a feed address and returns its id. It returns
// ErrExists if the address is already subscribed.
func (s *Store) CreateFeed(ctx context.Context, url string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO feeds (url) VALUES (?)`, url)
	if err != nil {
		if isUnique(err) {
			return 0, ErrExists
		}
		return 0, err
	}
	return res.LastInsertId()
}
