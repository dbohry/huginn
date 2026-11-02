package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/dbohry/huginn/internal/db"
	"github.com/dbohry/huginn/internal/feed"
)

func newStore(t *testing.T) (*Store, *sql.DB) {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := db.Migrate(conn); err != nil {
		t.Fatal(err)
	}
	return New(conn), conn
}

func ptr(t time.Time) *time.Time { return &t }

func TestCreateFeedRejectsDuplicates(t *testing.T) {
	st, _ := newStore(t)
	ctx := context.Background()

	id, err := st.CreateFeed(ctx, "https://example.com/feed")
	if err != nil || id == 0 {
		t.Fatalf("create: %d, %v", id, err)
	}
	if _, err := st.CreateFeed(ctx, "https://example.com/feed"); err != ErrExists {
		t.Fatalf("duplicate: err = %v, want ErrExists", err)
	}
	if _, err := st.GetFeed(ctx, 999); err != ErrNotFound {
		t.Fatalf("missing: err = %v, want ErrNotFound", err)
	}
}

func TestDueFeedsOrderingAndBackoff(t *testing.T) {
	st, _ := newStore(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)

	fresh, _ := st.CreateFeed(ctx, "https://a.example/feed")   // never fetched: due
	later, _ := st.CreateFeed(ctx, "https://b.example/feed")   // next fetch in the future
	overdue, _ := st.CreateFeed(ctx, "https://c.example/feed") // next fetch passed

	if err := st.RecordSuccess(ctx, later, now, now.Add(time.Hour), "", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordFailure(ctx, overdue, now.Add(-2*time.Hour), now.Add(-time.Hour), "boom"); err != nil {
		t.Fatal(err)
	}

	due, err := st.DueFeeds(ctx, now, 10)
	if err != nil {
		t.Fatal(err)
	}
	// Never-fetched feeds come first, then the most overdue.
	if len(due) != 2 || due[0].ID != fresh || due[1].ID != overdue {
		t.Fatalf("due = %+v", due)
	}
	if due[1].ErrorCount != 1 || due[1].LastError != "boom" {
		t.Fatalf("failure not recorded: %+v", due[1])
	}

	if err := st.RecordSuccess(ctx, overdue, now, now.Add(time.Hour), `"e"`, "lm", "Title", "https://c.example/"); err != nil {
		t.Fatal(err)
	}
	f, _ := st.GetFeed(ctx, overdue)
	if f.ErrorCount != 0 || f.LastError != "" || f.Title != "Title" || f.ETag != `"e"` {
		t.Fatalf("success should clear the error: %+v", f)
	}
	// An empty title on a later success must not erase the stored one.
	if err := st.RecordSuccess(ctx, overdue, now, now.Add(time.Hour), "", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if f, _ = st.GetFeed(ctx, overdue); f.Title != "Title" || f.SiteURL != "https://c.example/" {
		t.Fatalf("title/site erased: %+v", f)
	}
}

func TestUpsertItemsKeepsReadStateAndDates(t *testing.T) {
	st, conn := newStore(t)
	ctx := context.Background()
	id, _ := st.CreateFeed(ctx, "https://example.com/feed")
	t1 := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	first := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)

	items := []feed.Item{
		{GUID: "a", Title: "Dated", Published: ptr(t1), Summary: "v1"},
		{GUID: "b", Title: "Undated"},
	}
	if err := st.UpsertItems(ctx, id, items, first); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`UPDATE items SET is_read = 1 WHERE guid = 'a'`); err != nil {
		t.Fatal(err)
	}

	// The feed is fetched again later: "a" was edited, "b" still has no date.
	items[0].Summary = "v2"
	items[0].Title = "Dated (edited)"
	if err := st.UpsertItems(ctx, id, items, first.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}

	var n int
	conn.QueryRow(`SELECT COUNT(*) FROM items`).Scan(&n)
	if n != 2 {
		t.Fatalf("%d items, want 2 (no duplicates)", n)
	}
	var title, summary, published string
	var read int
	conn.QueryRow(`SELECT title, summary, is_read FROM items WHERE guid = 'a'`).Scan(&title, &summary, &read)
	if title != "Dated (edited)" || summary != "v2" || read != 1 {
		t.Fatalf("edit/read state: %q %q %d", title, summary, read)
	}
	conn.QueryRow(`SELECT published_at FROM items WHERE guid = 'b'`).Scan(&published)
	if published != formatTime(first) {
		t.Fatalf("an undated item should keep its first-seen date, got %s", published)
	}
}

func TestUpsertItemsTrimsOldest(t *testing.T) {
	st, conn := newStore(t)
	ctx := context.Background()
	id, _ := st.CreateFeed(ctx, "https://example.com/feed")
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	var items []feed.Item
	for i := 0; i < MaxItemsPerFeed+5; i++ {
		items = append(items, feed.Item{GUID: fmt.Sprint(i), Published: ptr(base.Add(time.Duration(i) * time.Minute))})
	}
	if err := st.UpsertItems(ctx, id, items, base); err != nil {
		t.Fatal(err)
	}
	var n int
	var oldest string
	conn.QueryRow(`SELECT COUNT(*), MIN(guid + 0) FROM items`).Scan(&n, &oldest)
	if n != MaxItemsPerFeed || oldest != "5" {
		t.Fatalf("kept %d items, oldest guid %s; want %d and 5", n, oldest, MaxItemsPerFeed)
	}
}
