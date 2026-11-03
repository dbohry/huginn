package store

import (
	"context"
	"database/sql"
	"errors"
)

// StarterFeed is a feed suggested to new users.
type StarterFeed struct {
	ID       int64
	Category string
	Title    string
	URL      string
}

// ListStarterFeeds returns the suggested feeds in the order they are shown:
// topics in the order they were first added, feeds in the order given.
func (s *Store) ListStarterFeeds(ctx context.Context) ([]StarterFeed, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, category, title, url FROM starter_feeds ORDER BY position, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []StarterFeed{}
	for rows.Next() {
		var f StarterFeed
		if err := rows.Scan(&f.ID, &f.Category, &f.Title, &f.URL); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// AddStarterFeed adds a suggestion at the end of the list. It returns ErrExists
// if the address is already suggested.
func (s *Store) AddStarterFeed(ctx context.Context, category, title, url string) (StarterFeed, error) {
	f := StarterFeed{Category: category, Title: title, URL: url}
	err := s.db.QueryRowContext(ctx, `INSERT INTO starter_feeds (category, title, url, position)
		VALUES (?, ?, ?, COALESCE((SELECT MAX(position) + 1 FROM starter_feeds), 0)) RETURNING id`,
		category, title, url).Scan(&f.ID)
	if err != nil {
		if isUnique(err) {
			return StarterFeed{}, ErrExists
		}
		return StarterFeed{}, err
	}
	return f, nil
}

// DeleteStarterFeed removes one suggestion.
func (s *Store) DeleteStarterFeed(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM starter_feeds WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ReplaceStarterFeeds swaps the whole list for feeds, in one transaction.
func (s *Store) ReplaceStarterFeeds(ctx context.Context, feeds []StarterFeed) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := replaceStarter(ctx, tx, feeds); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO settings (key, value) VALUES ('starter_seeded', '1')
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`); err != nil {
		return err
	}
	return tx.Commit()
}

// SeedStarterFeeds fills the list with feeds the first time it is called on a
// database, and does nothing afterwards, so an administrator who empties the
// list is not given the built-in one back.
func (s *Store) SeedStarterFeeds(ctx context.Context, feeds []StarterFeed) error {
	var seeded string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = 'starter_seeded'`).Scan(&seeded)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return s.ReplaceStarterFeeds(ctx, feeds)
}

func replaceStarter(ctx context.Context, tx *sql.Tx, feeds []StarterFeed) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM starter_feeds`); err != nil {
		return err
	}
	for i, f := range feeds {
		// A repeated address keeps its first entry.
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO starter_feeds (category, title, url, position) VALUES (?, ?, ?, ?)`,
			f.Category, f.Title, f.URL, i); err != nil {
			return err
		}
	}
	return nil
}

// FollowedURLs returns the set of addresses the user follows.
func (s *Store) FollowedURLs(ctx context.Context, userID int64) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT url FROM feeds WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		out[u] = true
	}
	return out, rows.Err()
}
