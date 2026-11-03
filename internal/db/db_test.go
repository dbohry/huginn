package db

import (
	"database/sql"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func openTemp(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestMigrateIsIdempotent(t *testing.T) {
	d := openTemp(t)
	if err := Migrate(d); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(d); err != nil {
		t.Fatalf("second run: %v", err)
	}

	var n int
	if err := d.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 7 {
		t.Fatalf("applied %d migrations, want 7", n)
	}
}

func TestItemsAreUniquePerFeedAndCascade(t *testing.T) {
	d := openTemp(t)
	if err := Migrate(d); err != nil {
		t.Fatal(err)
	}

	if _, err := d.Exec(`INSERT INTO users (id, username, password_hash, created_at) VALUES (1, 'a', 'x', 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`INSERT INTO feeds (id, user_id, url) VALUES (1, 1, 'https://example.com/feed.xml')`); err != nil {
		t.Fatal(err)
	}
	insert := `INSERT INTO items (feed_id, guid, title) VALUES (1, 'a', 'First')`
	if _, err := d.Exec(insert); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(insert); err == nil {
		t.Fatal("a duplicate (feed_id, guid) was accepted")
	}

	if _, err := d.Exec(`DELETE FROM feeds WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := d.QueryRow(`SELECT COUNT(*) FROM items`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("%d items left after deleting their feed", n)
	}
}

func TestInvalidMigrationNames(t *testing.T) {
	cases := map[string]string{
		"no number":    "init.sql",
		"zero version": "0000_init.sql",
	}
	for name, file := range cases {
		t.Run(name, func(t *testing.T) {
			files := fstest.MapFS{"migrations/" + file: {Data: []byte("SELECT 1;")}}
			if _, err := load(files); err == nil {
				t.Fatal("expected an error")
			}
		})
	}

	dup := fstest.MapFS{
		"migrations/0001_a.sql": {Data: []byte("SELECT 1;")},
		"migrations/0001_b.sql": {Data: []byte("SELECT 1;")},
	}
	if _, err := load(dup); err == nil {
		t.Fatal("duplicate versions were accepted")
	}
}

func TestFailedMigrationLeavesNoTrace(t *testing.T) {
	d := openTemp(t)
	files := fstest.MapFS{
		"migrations/0001_ok.sql":  {Data: []byte("CREATE TABLE ok (id INTEGER);")},
		"migrations/0002_bad.sql": {Data: []byte("CREATE TABLE half (id INTEGER); NOT SQL;")},
	}
	if err := migrate(d, files); err == nil {
		t.Fatal("expected the bad migration to fail")
	}

	var n int
	if err := d.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("%d migrations recorded, want 1", n)
	}
	if _, err := d.Exec(`SELECT * FROM half`); err == nil {
		t.Fatal("table from the failed migration still exists")
	}
}

// A database that already holds a user, a feed and an article is upgraded from the schema
// before the full-text columns: nothing is lost, and the new columns start at their defaults.
func TestUpgradeKeepsExistingData(t *testing.T) {
	d := openTemp(t)

	// The first four migrations only: the schema of the previous release.
	older := fstest.MapFS{}
	for _, name := range []string{"0001_init.sql", "0002_feeds_items.sql", "0003_users_sessions.sql", "0004_feeds_owner.sql"} {
		body, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		older["migrations/"+name] = &fstest.MapFile{Data: body}
	}
	if err := migrate(d, older); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO users (id, username, password_hash, created_at) VALUES (1, 'alex', 'hash', 0)`,
		`INSERT INTO feeds (id, user_id, url, title) VALUES (7, 1, 'https://example.com/feed', 'Example')`,
		`INSERT INTO items (id, feed_id, guid, title, summary, content_html, is_read) VALUES (42, 7, 'g', 'An article', 'Summary', '<p>Body</p>', 1)`,
	} {
		if _, err := d.Exec(q); err != nil {
			t.Fatal(err)
		}
	}

	if err := Migrate(d); err != nil { // up to the current schema
		t.Fatalf("upgrade: %v", err)
	}

	var title, summary, content, full string
	var read, feedFull int
	err := d.QueryRow(`SELECT title, summary, content_html, full_html, is_read FROM items WHERE id = 42`).Scan(&title, &summary, &content, &full, &read)
	if err != nil {
		t.Fatal(err)
	}
	if title != "An article" || summary != "Summary" || content != "<p>Body</p>" || read != 1 {
		t.Errorf("the article changed: %q %q %q read=%d", title, summary, content, read)
	}
	if full != "" {
		t.Errorf("full_html should start empty, got %q", full)
	}
	if err := d.QueryRow(`SELECT title, full_text FROM feeds WHERE id = 7`).Scan(&title, &feedFull); err != nil {
		t.Fatal(err)
	}
	if title != "Example" || feedFull != 0 {
		t.Errorf("the feed changed: %q full_text=%d", title, feedFull)
	}
	// The new column accepts only 0 and 1.
	if _, err := d.Exec(`UPDATE feeds SET full_text = 2 WHERE id = 7`); err == nil {
		t.Error("full_text accepted a value other than 0 or 1")
	}
}
