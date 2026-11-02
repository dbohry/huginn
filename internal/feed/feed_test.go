package feed

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseRSS(t *testing.T) {
	p, err := Parse(strings.NewReader(string(fixture(t, "rss.xml"))))
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Example RSS" || p.SiteURL != "https://example.com/" {
		t.Fatalf("feed = %+v", p)
	}
	if len(p.Items) != 2 {
		t.Fatalf("got %d items, want 2 (the duplicate guid is dropped)", len(p.Items))
	}

	first := p.Items[0]
	if first.GUID != "post-1" || first.Title != "First post" || first.Author != "Ada" {
		t.Fatalf("first = %+v", first)
	}
	if first.Published == nil || !first.Published.Equal(time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("published = %v", first.Published)
	}
	if !strings.Contains(first.ContentHTML, "<b>content</b>") || !strings.Contains(first.Summary, "Short summary") {
		t.Fatalf("content = %q, summary = %q", first.ContentHTML, first.Summary)
	}

	second := p.Items[1]
	if second.GUID != "https://example.com/second" {
		t.Fatalf("an item without a guid should use its link, got %q", second.GUID)
	}
	if second.Published != nil {
		t.Fatalf("an item without a date should have none, got %v", second.Published)
	}
}

func TestParseAtom(t *testing.T) {
	p, err := Parse(strings.NewReader(string(fixture(t, "atom.xml"))))
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Example Atom" || len(p.Items) != 1 {
		t.Fatalf("feed = %+v", p)
	}
	it := p.Items[0]
	if it.GUID != "urn:uuid:entry-1" || it.Author != "Grace" || it.URL != "https://example.org/a1" {
		t.Fatalf("item = %+v", it)
	}
	if it.Published == nil || it.Published.Year() != 2026 {
		t.Fatalf("an Atom entry's updated date should be used, got %v", it.Published)
	}
}

func TestParseJSONFeed(t *testing.T) {
	p, err := Parse(strings.NewReader(string(fixture(t, "feed.json"))))
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Example JSON Feed" || len(p.Items) != 1 {
		t.Fatalf("feed = %+v", p)
	}
	if it := p.Items[0]; it.GUID != "j1" || it.Author != "Linus" || !strings.Contains(it.ContentHTML, "JSON content") {
		t.Fatalf("item = %+v", it)
	}
}

func TestParseRejectsNonFeeds(t *testing.T) {
	for _, name := range []string{"malformed.xml", "page.html"} {
		if _, err := Parse(strings.NewReader(string(fixture(t, name)))); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestGUIDFallsBackToHash(t *testing.T) {
	xml := `<rss version="2.0"><channel><title>t</title><item><title>Only a title</title></item></channel></rss>`
	p, err := Parse(strings.NewReader(xml))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Items) != 1 || !strings.HasPrefix(p.Items[0].GUID, "hash:") {
		t.Fatalf("items = %+v", p.Items)
	}
}

func newTestFetcher() *Fetcher {
	f := NewFetcher("test")
	f.Client.Timeout = 2 * time.Second
	return f
}

func TestFetchConditionalRequests(t *testing.T) {
	body := fixture(t, "rss.xml")
	var gotUA, gotINM string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotINM = r.Header.Get("If-None-Match")
		if gotINM == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Last-Modified", "Mon, 02 Nov 2026 10:00:00 GMT")
		w.Write(body)
	}))
	defer srv.Close()
	f := newTestFetcher()

	res, err := f.Fetch(context.Background(), srv.URL, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.NotModified || res.Feed == nil || len(res.Feed.Items) != 2 {
		t.Fatalf("first fetch = %+v", res)
	}
	if res.ETag != `"v1"` || res.LastModified == "" {
		t.Fatalf("validators = %q, %q", res.ETag, res.LastModified)
	}
	if !strings.HasPrefix(gotUA, "Huginn/test") {
		t.Fatalf("User-Agent = %q", gotUA)
	}

	res, err = f.Fetch(context.Background(), srv.URL, res.ETag, res.LastModified)
	if err != nil {
		t.Fatal(err)
	}
	if !res.NotModified || res.Feed != nil || res.ETag != `"v1"` {
		t.Fatalf("second fetch = %+v", res)
	}
}

func TestFetchErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gone":
			http.Error(w, "gone", http.StatusGone)
		case "/big":
			w.Write([]byte(strings.Repeat("x", 2048)))
		case "/slow":
			time.Sleep(500 * time.Millisecond)
		default:
			w.Write(fixture(t, "page.html"))
		}
	}))
	defer srv.Close()

	f := newTestFetcher()
	f.MaxBytes = 1024
	f.Client.Timeout = 100 * time.Millisecond

	if _, err := f.Fetch(context.Background(), srv.URL+"/gone", "", ""); err == nil || !strings.Contains(err.Error(), "410") {
		t.Errorf("410: err = %v", err)
	}
	if _, err := f.Fetch(context.Background(), srv.URL+"/big", "", ""); !errors.Is(err, ErrTooLarge) {
		t.Errorf("big: err = %v", err)
	}
	if _, err := f.Fetch(context.Background(), srv.URL+"/slow", "", ""); err == nil {
		t.Error("slow: expected a timeout")
	}
	if _, err := f.Fetch(context.Background(), srv.URL+"/html", "", ""); err == nil {
		t.Error("html: expected a parse error")
	}
}

func TestValidateURL(t *testing.T) {
	for _, ok := range []string{"http://example.com/feed", "https://example.com"} {
		if err := ValidateURL(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "example.com/feed", "ftp://example.com/feed", "file:///etc/passwd", "javascript:alert(1)", "http://"} {
		if err := ValidateURL(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
