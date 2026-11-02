// Package db opens the SQLite database and applies schema migrations.
package db

import (
	"database/sql"
	"fmt"
	"net/url"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, registers "sqlite"
)

// Open opens (creating it if needed) the SQLite database at path.
//
// The connection settings are applied to every pooled connection:
//   - WAL journal mode, so readers do not block the single writer
//   - a busy timeout, so a second writer waits instead of failing
//   - foreign keys on
//   - immediate write transactions, which avoids lock-upgrade deadlocks
func Open(path string) (*sql.DB, error) {
	q := url.Values{}
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "synchronous(NORMAL)")
	q.Set("_txlock", "immediate")

	conn, err := sql.Open("sqlite", "file:"+path+"?"+q.Encode())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("open database: %w", err)
	}
	return conn, nil
}
