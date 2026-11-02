package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dbohry/huginn/internal/feed"
)

// feedSite serves the test feeds from ../feed/testdata plus a few broken paths.
func feedSite(t *testing.T) *httptest.Server {
	t.Helper()
	read := func(name string) []byte {
		b, err := os.ReadFile("../feed/testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/rss", func(w http.ResponseWriter, r *http.Request) { w.Write(read("rss.xml")) })
	mux.HandleFunc("/atom", func(w http.ResponseWriter, r *http.Request) { w.Write(read("atom.xml")) })
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) { w.Write(read("feed.json")) })
	mux.HandleFunc("/page", func(w http.ResponseWriter, r *http.Request) { w.Write(read("page.html")) })
	mux.HandleFunc("/home", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><link rel="alternate" type="application/atom+xml" href="/atom"></head><body>Home</body></html>`))
	})
	mux.HandleFunc("/gone", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "gone", http.StatusGone) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func (c *client) raw(method, path, contentType, body string) resp {
	c.e.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	if c.cookie != nil {
		req.AddCookie(c.cookie)
	}
	rec := httptest.NewRecorder()
	c.e.h.ServeHTTP(rec, req)
	return resp{rec}
}

func (r resp) list(key string) []map[string]any {
	var out []map[string]any
	items, _ := r.json()[key].([]any)
	for _, v := range items {
		out = append(out, v.(map[string]any))
	}
	return out
}

func (r resp) obj(key string) map[string]any {
	o, _ := r.json()[key].(map[string]any)
	return o
}

func TestAddFeedFetchesItAndShowsItsArticles(t *testing.T) {
	e := newEnv(t)
	site := feedSite(t)
	b := e.browser()
	mustStatus(t, b.setup("alex", "alex-password-1"), 201)

	r := b.do("POST", "/api/feeds", m{"url": site.URL + "/rss"})
	mustStatus(t, r, 201)
	f := r.obj("feed")
	if f["title"] != "Example RSS" || f["unread"] != float64(2) || f["error"] != "" {
		t.Fatalf("feed = %v", f)
	}

	items := b.do("GET", "/api/items", nil)
	mustStatus(t, items, 200)
	list := items.list("items")
	if len(list) != 2 {
		t.Fatalf("%d articles, want 2", len(list))
	}
	// The undated article is stamped with the time it was first seen, so it is
	// the newest; look the dated one up by title.
	var first map[string]any
	for _, it := range list {
		if it["title"] == "First post" {
			first = it
		}
	}
	if first == nil {
		t.Fatalf("no article titled First post in %v", list)
	}
	if first["title"] != "First post" || first["feedTitle"] != "Example RSS" || first["unread"] != true {
		t.Fatalf("first = %v", first)
	}
	if first["snippet"] != "Short summary" {
		t.Errorf("snippet should be plain text, got %q", first["snippet"])
	}

	detail := b.do("GET", "/api/items/"+idOf(first["id"]), nil)
	mustStatus(t, detail, 200)
	if got := detail.obj("item")["content"]; got != "<p>Full <b>content</b></p>" {
		t.Errorf("content = %q", got)
	}
}

func TestAddFeedRefusesBadAddresses(t *testing.T) {
	e := newEnv(t)
	site := feedSite(t)
	b := e.browser()
	mustStatus(t, b.setup("alex", "alex-password-1"), 201)

	cases := []struct {
		name, url string
		status    int
		message   string
	}{
		{"empty", "", 400, "Enter the address"},
		{"not a web address", "ftp://example.com/feed", 400, "not a web address"},
		{"a web page", site.URL + "/page", 422, "not an RSS, Atom or JSON feed"},
		{"an error page", site.URL + "/gone", 422, "410"},
		{"nothing there", "http://127.0.0.1:1/feed", 422, "could not be reached"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := b.do("POST", "/api/feeds", m{"url": c.url})
			mustStatus(t, r, c.status)
			if !strings.Contains(r.errMsg(), c.message) {
				t.Errorf("message %q should contain %q", r.errMsg(), c.message)
			}
		})
	}
	if feeds := b.do("GET", "/api/feeds", nil).list("feeds"); len(feeds) != 0 {
		t.Fatalf("failed adds left %d feeds behind", len(feeds))
	}

	mustStatus(t, b.do("POST", "/api/feeds", m{"url": site.URL + "/atom"}), 201)
	r := b.do("POST", "/api/feeds", m{"url": site.URL + "/atom"})
	mustStatus(t, r, 409)
}

func TestAddingASiteSubscribesToTheFeedItAdvertises(t *testing.T) {
	e := newEnv(t)
	site := feedSite(t)
	b := e.browser()
	mustStatus(t, b.setup("alex", "alex-password-1"), 201)

	r := b.do("POST", "/api/feeds", m{"url": site.URL + "/home"})
	mustStatus(t, r, 201)
	f := r.obj("feed")
	if f["title"] != "Example Atom" || f["url"] != site.URL+"/atom" {
		t.Fatalf("feed = %v", f)
	}
	// The page itself is not left behind as a broken feed.
	if feeds := b.do("GET", "/api/feeds", nil).list("feeds"); len(feeds) != 1 {
		t.Fatalf("%d feeds, want 1", len(feeds))
	}
	// Adding the page again finds the same feed, which is already followed.
	mustStatus(t, b.do("POST", "/api/feeds", m{"url": site.URL + "/home"}), 409)
}

func TestFeedsAndArticlesBelongToTheirUser(t *testing.T) {
	e := newEnv(t)
	site := feedSite(t)
	alex, sam, _ := adminAndUser(t, e)

	r := sam.do("POST", "/api/feeds", m{"url": site.URL + "/rss"})
	mustStatus(t, r, 201)
	feedID := idOf(r.obj("feed")["id"])
	items := sam.do("GET", "/api/items", nil).list("items")
	itemID := idOf(items[0]["id"])

	// Alex follows the same address without touching Sam's copy.
	mustStatus(t, alex.do("POST", "/api/feeds", m{"url": site.URL + "/rss"}), 201)

	for _, c := range []struct{ method, path string }{
		{"DELETE", "/api/feeds/" + feedID},
		{"POST", "/api/feeds/" + feedID + "/refresh"},
	} {
		mustStatus(t, alex.do(c.method, c.path, nil), 404)
	}
	// Alex's own feed has another id, so Sam's ids do not exist for Alex.
	mustStatus(t, alex.do("GET", "/api/items/"+itemID, nil), 404)
	mustStatus(t, alex.do("PUT", "/api/items/"+itemID+"/read", m{"read": true}), 404)
	mustStatus(t, alex.do("POST", "/api/read-all", m{"feedId": r.obj("feed")["id"]}), 404)
	mustStatus(t, alex.do("GET", "/api/items?feed="+feedID, nil), 200)
	if got := alex.do("GET", "/api/items?feed="+feedID, nil).list("items"); len(got) != 0 {
		t.Errorf("Alex can list %d of Sam's articles", len(got))
	}
	if n := len(sam.do("GET", "/api/feeds", nil).list("feeds")); n != 1 {
		t.Errorf("Sam has %d feeds, want 1", n)
	}
}

func TestReadState(t *testing.T) {
	e := newEnv(t)
	site := feedSite(t)
	b := e.browser()
	mustStatus(t, b.setup("alex", "alex-password-1"), 201)
	rss := b.do("POST", "/api/feeds", m{"url": site.URL + "/rss"}).obj("feed")
	atom := b.do("POST", "/api/feeds", m{"url": site.URL + "/atom"}).obj("feed")

	items := b.do("GET", "/api/items", nil).list("items")
	if len(items) != 3 {
		t.Fatalf("%d articles, want 3", len(items))
	}
	id := idOf(items[0]["id"])

	mustStatus(t, b.do("PUT", "/api/items/"+id+"/read", m{"read": true}), 204)
	unread := b.do("GET", "/api/items?unread=1", nil).list("items")
	if len(unread) != 2 {
		t.Fatalf("%d unread, want 2", len(unread))
	}
	mustStatus(t, b.do("PUT", "/api/items/"+id+"/read", m{"read": false}), 204)
	if n := len(b.do("GET", "/api/items?unread=1", nil).list("items")); n != 3 {
		t.Fatalf("%d unread after undoing, want 3", n)
	}

	r := b.do("POST", "/api/read-all", m{"feedId": atom["id"]})
	mustStatus(t, r, 200)
	if r.json()["marked"] != float64(1) {
		t.Errorf("marked = %v, want 1", r.json()["marked"])
	}
	feeds := b.do("GET", "/api/feeds", nil).list("feeds")
	unreadBy := map[string]float64{}
	for _, f := range feeds {
		unreadBy[f["title"].(string)] = f["unread"].(float64)
	}
	if unreadBy["Example Atom"] != 0 || unreadBy["Example RSS"] != 2 {
		t.Fatalf("unread counts = %v", unreadBy)
	}

	mustStatus(t, b.do("POST", "/api/read-all", m{}), 200)
	if n := len(b.do("GET", "/api/items?unread=1", nil).list("items")); n != 0 {
		t.Fatalf("%d unread after marking everything read", n)
	}
	_ = rss
}

func TestArticleContentIsSanitized(t *testing.T) {
	e := newEnv(t)
	b := e.browser()
	mustStatus(t, b.setup("alex", "alex-password-1"), 201)
	ctx := context.Background()

	id, err := e.s.store.CreateFeed(ctx, 1, "https://evil.example/feed")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	err = e.s.store.UpsertItems(ctx, id, []feed.Item{{
		GUID: "x", Title: "<b>Hostile</b> title", URL: "javascript:alert(1)", Published: &now,
		Summary: `<p>Summary <script>alert(1)</script>text</p>`,
		ContentHTML: `<p onclick="alert(1)">Body</p><script>alert(2)</script>` +
			`<img src=x onerror="alert(3)"><a href="javascript:alert(4)">link</a><iframe src="https://evil.example/"></iframe>`,
	}}, now)
	if err != nil {
		t.Fatal(err)
	}

	list := b.do("GET", "/api/items", nil).list("items")
	if len(list) != 1 {
		t.Fatalf("%d articles", len(list))
	}
	if list[0]["url"] != "" {
		t.Errorf("a javascript: link must not be offered, got %q", list[0]["url"])
	}
	if s, _ := list[0]["snippet"].(string); strings.Contains(s, "<") || strings.Contains(s, "alert") {
		t.Errorf("snippet = %q", s)
	}

	body := b.do("GET", "/api/items/"+idOf(list[0]["id"]), nil).obj("item")["content"].(string)
	lower := strings.ToLower(body)
	for _, bad := range []string{"<script", "onclick", "onerror", "javascript:", "<iframe"} {
		if strings.Contains(lower, bad) {
			t.Errorf("content still contains %q: %s", bad, body)
		}
	}
	if !strings.Contains(body, "Body") {
		t.Errorf("the harmless text was lost: %s", body)
	}
}

func TestListItemsPaging(t *testing.T) {
	e := newEnv(t)
	b := e.browser()
	mustStatus(t, b.setup("alex", "alex-password-1"), 201)
	ctx := context.Background()
	id, _ := e.s.store.CreateFeed(ctx, 1, "https://example.com/feed")
	base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	var items []feed.Item
	for i := 0; i < 5; i++ {
		p := base.Add(time.Duration(i) * time.Hour)
		items = append(items, feed.Item{GUID: fmt.Sprint(i), Title: fmt.Sprintf("Item %d", i), Published: &p})
	}
	if err := e.s.store.UpsertItems(ctx, id, items, base); err != nil {
		t.Fatal(err)
	}

	var titles []string
	next := ""
	for page := 0; page < 10; page++ {
		path := "/api/items?limit=2"
		if next != "" {
			path += "&after=" + url.QueryEscape(next)
		}
		r := b.do("GET", path, nil)
		mustStatus(t, r, 200)
		for _, it := range r.list("items") {
			titles = append(titles, it["title"].(string))
		}
		next, _ = r.json()["next"].(string)
		if next == "" {
			break
		}
	}
	if got := strings.Join(titles, ","); got != "Item 4,Item 3,Item 2,Item 1,Item 0" {
		t.Fatalf("paged through %q", got)
	}

	for _, bad := range []string{"limit=0", "limit=1000", "limit=x", "after=nonsense", "feed=abc"} {
		mustStatus(t, b.do("GET", "/api/items?"+bad, nil), 400)
	}
}

func TestOPMLImportAndExport(t *testing.T) {
	e := newEnv(t)
	site := feedSite(t)
	b := e.browser()
	mustStatus(t, b.setup("alex", "alex-password-1"), 201)
	mustStatus(t, b.do("POST", "/api/feeds", m{"url": site.URL + "/rss"}), 201)

	file := `<?xml version="1.0"?><opml version="2.0"><head/><body>
	  <outline text="Folder"><outline type="rss" text="Atom one" xmlUrl="` + site.URL + `/atom"/></outline>
	  <outline type="rss" text="Already followed" xmlUrl="` + site.URL + `/rss"/>
	  <outline type="rss" text="JSON one" xmlUrl="` + site.URL + `/json"/>
	  <outline type="rss" text="Bad scheme" xmlUrl="ftp://example.com/feed"/>
	</body></opml>`
	r := b.raw("POST", "/api/opml", "text/xml", file)
	mustStatus(t, r, 200)
	if got := r.json(); got["added"] != float64(2) || got["existing"] != float64(1) || got["invalid"] != float64(1) {
		t.Fatalf("import result = %v", got)
	}
	e.s.bg.Wait() // the new feeds are fetched in the background

	titles := map[string]bool{}
	for _, f := range b.do("GET", "/api/feeds", nil).list("feeds") {
		titles[f["title"].(string)] = true
	}
	for _, want := range []string{"Example RSS", "Example Atom", "Example JSON Feed"} {
		if !titles[want] {
			t.Errorf("feeds after import = %v, missing %q", titles, want)
		}
	}

	export := b.do("GET", "/api/opml", nil)
	mustStatus(t, export, 200)
	if ct := export.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/x-opml") {
		t.Errorf("Content-Type = %q", ct)
	}
	if cd := export.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Errorf("Content-Disposition = %q", cd)
	}
	for _, want := range []string{site.URL + "/rss", site.URL + "/atom", site.URL + "/json"} {
		if !strings.Contains(export.Body.String(), want) {
			t.Errorf("export is missing %s", want)
		}
	}

	mustStatus(t, b.raw("POST", "/api/opml", "text/xml", "not opml"), 400)
}

func TestRefreshAllRunsInTheBackground(t *testing.T) {
	e := newEnv(t)
	site := feedSite(t)
	b := e.browser()
	mustStatus(t, b.setup("alex", "alex-password-1"), 201)
	mustStatus(t, b.do("POST", "/api/feeds", m{"url": site.URL + "/rss"}), 201)

	r := b.do("POST", "/api/refresh", nil)
	mustStatus(t, r, 202)
	if r.json()["feeds"] != float64(1) {
		t.Errorf("response = %v", r.json())
	}
	e.s.bg.Wait()

	feeds := b.do("GET", "/api/feeds", nil).list("feeds")
	if feeds[0]["lastFetchedAt"] == nil {
		t.Error("the feed has no last-fetched time")
	}
}

func TestRefreshOneFeedReportsProblems(t *testing.T) {
	e := newEnv(t)
	site := feedSite(t)
	b := e.browser()
	mustStatus(t, b.setup("alex", "alex-password-1"), 201)
	id, err := e.s.store.CreateFeed(context.Background(), 1, site.URL+"/gone")
	if err != nil {
		t.Fatal(err)
	}

	r := b.do("POST", fmt.Sprintf("/api/feeds/%d/refresh", id), nil)
	mustStatus(t, r, 422)
	if !strings.Contains(r.errMsg(), "410") {
		t.Errorf("message = %q", r.errMsg())
	}
	feeds := b.do("GET", "/api/feeds", nil).list("feeds")
	if msg, _ := feeds[0]["error"].(string); !strings.Contains(msg, "410") {
		t.Errorf("the failure should be shown on the feed, got %q", msg)
	}
}

func TestAPIRequiresASession(t *testing.T) {
	e := newEnv(t)
	b := e.browser()
	for _, c := range []struct{ method, path string }{
		{"GET", "/api/feeds"}, {"POST", "/api/feeds"}, {"DELETE", "/api/feeds/1"},
		{"POST", "/api/feeds/1/refresh"}, {"POST", "/api/refresh"}, {"GET", "/api/items"},
		{"GET", "/api/items/1"}, {"PUT", "/api/items/1/read"}, {"POST", "/api/read-all"},
		{"GET", "/api/opml"}, {"POST", "/api/opml"},
	} {
		if r := b.do(c.method, c.path, nil); r.Code != 401 {
			t.Errorf("%s %s without a session: status %d, want 401", c.method, c.path, r.Code)
		}
	}
}
