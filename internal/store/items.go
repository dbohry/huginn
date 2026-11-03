package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Item is an article together with its feed's name.
type Item struct {
	ID          int64
	FeedID      int64
	FeedTitle   string
	FeedURL     string
	Title       string
	URL         string
	Author      string
	PublishedAt time.Time
	// Summary is cut short when listing. ContentHTML and FullHTML are only set by GetItem.
	Summary     string
	ContentHTML string
	// FullHTML is the article loaded from its own page, or "" if that has not been done.
	FullHTML string
	IsRead   bool
}

// Cursor marks where the next page of articles starts. Articles are sorted
// newest first, by publication time and then id.
type Cursor struct {
	Published string
	ID        int64
}

// ItemFilter selects articles for ListItems.
type ItemFilter struct {
	UserID     int64
	FeedID     int64 // 0 means every feed
	UnreadOnly bool
	After      *Cursor
	Limit      int
}

// summaryListLimit bounds how much summary text a list query reads per item.
const summaryListLimit = 2000

const itemFrom = `FROM items i JOIN feeds f ON f.id = i.feed_id`

func scanItem(row scanner, withContent bool) (Item, error) {
	var it Item
	var published sql.NullString
	dest := []any{&it.ID, &it.FeedID, &it.FeedTitle, &it.FeedURL, &it.Title, &it.URL, &it.Author,
		&published, &it.Summary, &it.IsRead}
	if withContent {
		dest = append(dest, &it.ContentHTML, &it.FullHTML)
	}
	if err := row.Scan(dest...); err != nil {
		return Item{}, err
	}
	if p := parseTime(published); p != nil {
		it.PublishedAt = *p
	}
	return it, nil
}

// ListItems returns one page of a user's articles, newest first, and the cursor
// for the next page (nil when this is the last page).
func (s *Store) ListItems(ctx context.Context, f ItemFilter) ([]Item, *Cursor, error) {
	if f.Limit < 1 {
		f.Limit = 50
	}
	query := `SELECT i.id, i.feed_id, f.title, f.url, i.title, i.url, i.author,
			i.published_at, substr(i.summary, 1, ?), i.is_read ` + itemFrom + ` WHERE f.user_id = ?`
	if f.FeedID == 0 && !f.UnreadOnly {
		// Walk articles newest first (items_published) and look up each feed's owner,
		// so a page costs a page of rows however many articles there are.
		query = strings.Replace(query, " JOIN feeds f ON", " CROSS JOIN feeds f ON", 1)
	}
	args := []any{summaryListLimit, f.UserID}
	if f.FeedID != 0 {
		query += ` AND i.feed_id = ?`
		args = append(args, f.FeedID)
	}
	if f.UnreadOnly {
		query += ` AND i.is_read = 0`
	}
	if f.After != nil {
		query += ` AND (i.published_at < ? OR (i.published_at = ? AND i.id < ?))`
		args = append(args, f.After.Published, f.After.Published, f.After.ID)
	}
	query += ` ORDER BY i.published_at DESC, i.id DESC LIMIT ?`
	args = append(args, f.Limit+1) // one extra row says whether there is a next page

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items := []Item{}
	for rows.Next() {
		it, err := scanItem(rows, false)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	var next *Cursor
	if len(items) > f.Limit {
		items = items[:f.Limit]
		last := items[len(items)-1]
		next = &Cursor{Published: formatTime(last.PublishedAt), ID: last.ID}
	}
	return items, next, nil
}

// GetItem returns one of the user's articles with its full content.
func (s *Store) GetItem(ctx context.Context, userID, id int64) (Item, error) {
	it, err := scanItem(s.db.QueryRowContext(ctx, `SELECT i.id, i.feed_id, f.title, f.url, i.title, i.url, i.author,
		i.published_at, i.summary, i.is_read, i.content_html, i.full_html `+itemFrom+` WHERE i.id = ? AND f.user_id = ?`, id, userID), true)
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	return it, err
}

// SetRead marks one of the user's articles read or unread.
func (s *Store) SetRead(ctx context.Context, userID, id int64, read bool) error {
	res, err := s.db.ExecContext(ctx, `UPDATE items SET is_read = ?
		WHERE id = ? AND feed_id IN (SELECT id FROM feeds WHERE user_id = ?)`, read, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkAllRead marks the user's unread articles read, in one feed or (feedID 0)
// in all of them. It returns how many changed.
func (s *Store) MarkAllRead(ctx context.Context, userID, feedID int64) (int64, error) {
	query := `UPDATE items SET is_read = 1 WHERE is_read = 0 AND feed_id IN (SELECT id FROM feeds WHERE user_id = ?`
	args := []any{userID}
	if feedID != 0 {
		query += ` AND id = ?`
		args = append(args, feedID)
	}
	res, err := s.db.ExecContext(ctx, query+`)`, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// GetUserFeed returns one of the user's feeds.
func (s *Store) GetUserFeed(ctx context.Context, userID, id int64) (Feed, error) {
	f, err := s.GetFeed(ctx, id)
	if err != nil {
		return Feed{}, err
	}
	if f.UserID != userID {
		return Feed{}, ErrNotFound
	}
	return f, nil
}

// DeleteFeed unsubscribes the user from a feed and deletes its articles.
func (s *Store) DeleteFeed(ctx context.Context, userID, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM feeds WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// UserFeedIDs returns the ids of all of a user's feeds.
func (s *Store) UserFeedIDs(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM feeds WHERE user_id = ? ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// SaveFullText keeps the full article loaded from an article's own page.
func (s *Store) SaveFullText(ctx context.Context, userID, id int64, fullHTML string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE items SET full_html = ?
		WHERE id = ? AND feed_id IN (SELECT id FROM feeds WHERE user_id = ?)`, fullHTML, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetFeedFullText turns loading full articles automatically on or off for one of the user's feeds.
func (s *Store) SetFeedFullText(ctx context.Context, userID, feedID int64, on bool) error {
	res, err := s.db.ExecContext(ctx, `UPDATE feeds SET full_text = ? WHERE id = ? AND user_id = ?`, on, feedID, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
