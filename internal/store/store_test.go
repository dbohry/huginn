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
	// Feeds belong to a user: every test here uses user 1.
	if _, err := conn.Exec(`INSERT INTO users (id, username, password_hash, created_at) VALUES (1, 'tester', 'x', 0)`); err != nil {
		t.Fatal(err)
	}
	return New(conn), conn
}

func ptr(t time.Time) *time.Time { return &t }

func TestCreateFeedRejectsDuplicates(t *testing.T) {
	st, _ := newStore(t)
	ctx := context.Background()

	id, err := st.CreateFeed(ctx, 1, "https://example.com/feed")
	if err != nil || id == 0 {
		t.Fatalf("create: %d, %v", id, err)
	}
	if _, err := st.CreateFeed(ctx, 1, "https://example.com/feed"); err != ErrExists {
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

	fresh, _ := st.CreateFeed(ctx, 1, "https://a.example/feed")   // never fetched: due
	later, _ := st.CreateFeed(ctx, 1, "https://b.example/feed")   // next fetch in the future
	overdue, _ := st.CreateFeed(ctx, 1, "https://c.example/feed") // next fetch passed

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
	id, _ := st.CreateFeed(ctx, 1, "https://example.com/feed")
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
	id, _ := st.CreateFeed(ctx, 1, "https://example.com/feed")
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

func seedItems(t *testing.T, st *Store, feedID int64, base time.Time, n int, prefix string) {
	t.Helper()
	var items []feed.Item
	for i := 0; i < n; i++ {
		items = append(items, feed.Item{
			GUID: fmt.Sprintf("%s%d", prefix, i), Title: fmt.Sprintf("%s item %d", prefix, i),
			Published: ptr(base.Add(time.Duration(i) * time.Hour)), Summary: "<p>sum</p>", ContentHTML: "<p>body</p>",
		})
	}
	if err := st.UpsertItems(context.Background(), feedID, items, base); err != nil {
		t.Fatal(err)
	}
}

func TestListItemsPagesNewestFirst(t *testing.T) {
	st, _ := newStore(t)
	ctx := context.Background()
	id, _ := st.CreateFeed(ctx, 1, "https://example.com/feed")
	seedItems(t, st, id, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), 5, "a")

	page1, next, err := st.ListItems(ctx, ItemFilter{UserID: 1, Limit: 2})
	if err != nil || len(page1) != 2 || next == nil {
		t.Fatalf("page 1: %d items, next %v, err %v", len(page1), next, err)
	}
	if page1[0].Title != "a item 4" || page1[1].Title != "a item 3" {
		t.Fatalf("not newest first: %q, %q", page1[0].Title, page1[1].Title)
	}
	page2, next, _ := st.ListItems(ctx, ItemFilter{UserID: 1, Limit: 2, After: next})
	if len(page2) != 2 || page2[0].Title != "a item 2" || next == nil {
		t.Fatalf("page 2: %+v next %v", page2, next)
	}
	page3, next, _ := st.ListItems(ctx, ItemFilter{UserID: 1, Limit: 2, After: next})
	if len(page3) != 1 || page3[0].Title != "a item 0" || next != nil {
		t.Fatalf("page 3: %+v next %v", page3, next)
	}
}

func TestItemsAreScopedToTheirOwner(t *testing.T) {
	st, conn := newStore(t)
	ctx := context.Background()
	if _, err := conn.Exec(`INSERT INTO users (id, username, password_hash, created_at) VALUES (2, 'other', 'x', 0)`); err != nil {
		t.Fatal(err)
	}
	mine, _ := st.CreateFeed(ctx, 1, "https://example.com/feed")
	theirs, _ := st.CreateFeed(ctx, 2, "https://example.com/feed")
	base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	seedItems(t, st, mine, base, 2, "mine")
	seedItems(t, st, theirs, base, 3, "theirs")

	items, _, _ := st.ListItems(ctx, ItemFilter{UserID: 1})
	if len(items) != 2 {
		t.Fatalf("user 1 sees %d items, want 2", len(items))
	}
	var theirItem int64
	conn.QueryRow(`SELECT id FROM items WHERE feed_id = ? LIMIT 1`, theirs).Scan(&theirItem)

	if _, err := st.GetItem(ctx, 1, theirItem); err != ErrNotFound {
		t.Errorf("GetItem of another user's item: %v", err)
	}
	if err := st.SetRead(ctx, 1, theirItem, true); err != ErrNotFound {
		t.Errorf("SetRead of another user's item: %v", err)
	}
	if err := st.DeleteFeed(ctx, 1, theirs); err != ErrNotFound {
		t.Errorf("DeleteFeed of another user's feed: %v", err)
	}
	if _, err := st.GetUserFeed(ctx, 1, theirs); err != ErrNotFound {
		t.Errorf("GetUserFeed of another user's feed: %v", err)
	}
	if n, _ := st.MarkAllRead(ctx, 1, 0); n != 2 {
		t.Errorf("MarkAllRead changed %d rows, want only user 1's 2", n)
	}
	var unread int
	conn.QueryRow(`SELECT COUNT(*) FROM items WHERE is_read = 0`).Scan(&unread)
	if unread != 3 {
		t.Errorf("%d unread left, want user 2's 3 untouched", unread)
	}
}

func TestReadStateAndUnreadCounts(t *testing.T) {
	st, _ := newStore(t)
	ctx := context.Background()
	a, _ := st.CreateFeedWithTitle(ctx, 1, "https://a.example/feed", "Beta")
	b, _ := st.CreateFeedWithTitle(ctx, 1, "https://b.example/feed", "alpha")
	base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	seedItems(t, st, a, base, 3, "a")
	seedItems(t, st, b, base, 2, "b")

	feeds, _ := st.ListFeeds(ctx, 1)
	if len(feeds) != 2 || feeds[0].Title != "alpha" || feeds[0].Unread != 2 || feeds[1].Unread != 3 {
		t.Fatalf("feeds = %+v", feeds)
	}

	items, _, _ := st.ListItems(ctx, ItemFilter{UserID: 1, FeedID: a})
	if err := st.SetRead(ctx, 1, items[0].ID, true); err != nil {
		t.Fatal(err)
	}
	unread, _, _ := st.ListItems(ctx, ItemFilter{UserID: 1, FeedID: a, UnreadOnly: true})
	if len(unread) != 2 {
		t.Fatalf("%d unread in feed a, want 2", len(unread))
	}
	if err := st.SetRead(ctx, 1, items[0].ID, false); err != nil {
		t.Fatal(err)
	}

	if n, _ := st.MarkAllRead(ctx, 1, a); n != 3 {
		t.Fatalf("MarkAllRead(feed a) changed %d, want 3", n)
	}
	feeds, _ = st.ListFeeds(ctx, 1)
	if feeds[0].Unread != 2 || feeds[1].Unread != 0 {
		t.Fatalf("after marking feed a read: %+v", feeds)
	}

	full, err := st.GetItem(ctx, 1, items[0].ID)
	if err != nil || full.ContentHTML != "<p>body</p>" || full.FeedTitle != "Beta" {
		t.Fatalf("GetItem: %+v, %v", full, err)
	}
	if err := st.DeleteFeed(ctx, 1, a); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetItem(ctx, 1, items[0].ID); err != ErrNotFound {
		t.Fatalf("items should go with their feed: %v", err)
	}
}
