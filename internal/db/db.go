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
//   - the default rollback journal, not WAL: WAL needs shared memory and reliable
//     locks between processes, which a network filesystem such as NFS cannot give,
//     and on one a database call can block forever
//   - a busy timeout, so a second writer waits instead of failing
//   - foreign keys on
//   - immediate write transactions, which avoids lock-upgrade deadlocks
func Open(path string) (*sql.DB, error) {
	q := url.Values{}
	q.Add("_pragma", "journal_mode(DELETE)")
	q.Add("_pragma", "busy_timeout(10000)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "synchronous(FULL)")
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
