package refresh

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dbohry/huginn/internal/db"
	"github.com/dbohry/huginn/internal/feed"
	"github.com/dbohry/huginn/internal/store"
)

func setup(t *testing.T) (*Service, *store.Store, *sql.DB, *time.Time) {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := db.Migrate(conn); err != nil {
		t.Fatal(err)
	}
	st := store.New(conn)
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	f := feed.NewFetcher("test")
	f.Client.Timeout = 2 * time.Second
	svc := New(st, f, slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.Now = func() time.Time { return now }
	return svc, st, conn, &now
}

func rssFixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("../feed/testdata/rss.xml")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestBackoff(t *testing.T) {
	iv := 30 * time.Minute
	cases := map[int]time.Duration{0: iv, 1: time.Hour, 2: 2 * time.Hour, 3: 4 * time.Hour, 10: MaxBackoff}
	for errs, want := range cases {
		if got := Backoff(iv, errs); got != want {
			t.Errorf("Backoff(%d) = %v, want %v", errs, got, want)
		}
	}
}

func TestRefreshFeedStoresItemsThenUsesConditionalGet(t *testing.T) {
	svc, st, conn, now := setup(t)
	body := rssFixture(t)
	var requests, notModified int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		if r.Header.Get("If-None-Match") == `"v1"` {
			atomic.AddInt32(&notModified, 1)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		w.Write(body)
	}))
	defer srv.Close()

	ctx := context.Background()
	id, _ := st.CreateFeed(ctx, srv.URL)
	if err := svc.RefreshFeed(ctx, id); err != nil {
		t.Fatal(err)
	}

	f, _ := st.GetFeed(ctx, id)
	if f.Title != "Example RSS" || f.ETag != `"v1"` || f.NextFetchAt == nil || !f.NextFetchAt.Equal(now.Add(DefaultInterval)) {
		t.Fatalf("feed after refresh: %+v", f)
	}
	var n int
	conn.QueryRow(`SELECT COUNT(*) FROM items`).Scan(&n)
	if n != 2 {
		t.Fatalf("%d items stored, want 2", n)
	}

	if err := svc.RefreshFeed(ctx, id); err != nil {
		t.Fatal(err)
	}
	if notModified != 1 {
		t.Fatalf("second refresh should have been a 304, got %d", notModified)
	}
	if f, _ = st.GetFeed(ctx, id); f.Title != "Example RSS" {
		t.Fatalf("a 304 must not erase the title: %+v", f)
	}
}

func TestRefreshFeedFailureBacksOff(t *testing.T) {
	svc, st, _, now := setup(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx := context.Background()
	id, _ := st.CreateFeed(ctx, srv.URL)

	if err := svc.RefreshFeed(ctx, id); err == nil {
		t.Fatal("expected the fetch error to be returned")
	}
	f, _ := st.GetFeed(ctx, id)
	if f.ErrorCount != 1 || f.LastError == "" || !f.NextFetchAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("after one failure: %+v", f)
	}
	svc.RefreshFeed(ctx, id)
	if f, _ = st.GetFeed(ctx, id); f.ErrorCount != 2 || !f.NextFetchAt.Equal(now.Add(2*time.Hour)) {
		t.Fatalf("after two failures: %+v", f)
	}
}

func TestRefreshDueOnlyFetchesDueFeeds(t *testing.T) {
	svc, st, _, now := setup(t)
	body := rssFixture(t)
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Write(body)
	}))
	defer srv.Close()

	ctx := context.Background()
	due1, _ := st.CreateFeed(ctx, srv.URL+"/one")
	due2, _ := st.CreateFeed(ctx, srv.URL+"/two")
	notDue, _ := st.CreateFeed(ctx, srv.URL+"/three")
	st.RecordSuccess(ctx, notDue, *now, now.Add(time.Hour), "", "", "", "")

	if n := svc.RefreshDue(ctx); n != 2 {
		t.Fatalf("tried %d feeds, want 2", n)
	}
	if hits != 2 {
		t.Fatalf("%d requests, want 2", hits)
	}
	for _, id := range []int64{due1, due2} {
		if f, _ := st.GetFeed(ctx, id); f.LastFetchedAt == nil {
			t.Errorf("feed %d was not fetched", id)
		}
	}
	if n := svc.RefreshDue(ctx); n != 0 {
		t.Fatalf("a second pass found %d due feeds, want 0", n)
	}
}
