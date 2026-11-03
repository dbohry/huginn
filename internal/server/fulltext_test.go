package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dbohry/huginn/internal/feed"
)

const unsafePage = `<!doctype html><html><head><title>Unsafe</title><script>window.pwned = 1</script></head><body>
<article><h1>An article with traps</h1>
<p>This paragraph is the real text of the article and it is long enough on its own to be recognised as the main content of the page, with a second sentence for good measure.</p>
<p>A second paragraph keeps going so that the extractor has plenty to hold on to. <a href="javascript:alert(1)">a bad link</a> and a <a href="https://example.com/fine">fine one</a>.</p>
<p><img src="/x.png" onerror="alert(1)" alt="traps"> More text follows here so the paragraph is not tiny, and the article carries on for a while longer than the minimum.</p>
</article></body></html>`

// articleSite serves pages for loading full articles, and counts requests for the article page.
func articleSite(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	read := func(name string) []byte {
		b, err := os.ReadFile("../article/testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	var hits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/article", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(read("news.html"))
	})
	mux.HandleFunc("/home", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(read("home.html"))
	})
	mux.HandleFunc("/unsafe", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(unsafePage))
	})
	mux.HandleFunc("/data.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"a":1}`))
	})
	mux.HandleFunc("/gone", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "gone", http.StatusNotFound) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &hits
}

// seedArticle adds one article to a feed owned by user 1 and returns its id.
func seedArticle(t *testing.T, e *env, feedID int64, guid, link, summary, content string) string {
	t.Helper()
	now := time.Now()
	err := e.s.store.UpsertItems(context.Background(), feedID, []feed.Item{
		{GUID: guid, Title: "Title " + guid, URL: link, Summary: summary, ContentHTML: content, Published: &now},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := e.s.db.QueryRow(`SELECT id FROM items WHERE feed_id = ? AND guid = ?`, feedID, guid).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprint(id)
}

func newFeed(t *testing.T, e *env, owner int64, url string) int64 {
	t.Helper()
	id, err := e.s.store.CreateFeed(context.Background(), owner, url)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestSummaryOnlyHintFollowsWhatTheFeedGave(t *testing.T) {
	e := newEnv(t)
	b := e.browser()
	mustStatus(t, b.setup("alex", "alex-password-1"), 201)
	fid := newFeed(t, e, 1, "https://example.com/feed")

	teaser := seedArticle(t, e, fid, "teaser", "https://example.com/a", "Two short sentences. That is all the feed shares.", "")
	long := seedArticle(t, e, fid, "long", "https://example.com/b", strings.Repeat("A whole article in the description. ", 30), "")
	content := seedArticle(t, e, fid, "content", "https://example.com/c", "Short summary.", "<p>The feed also gave the content.</p>")

	for id, want := range map[string]bool{teaser: true, long: false, content: false} {
		item := b.do("GET", "/api/items/"+id, nil).obj("item")
		if item["summaryOnly"] != want {
			t.Errorf("article %s: summaryOnly = %v, want %v", id, item["summaryOnly"], want)
		}
		if item["fullText"] != false {
			t.Errorf("article %s: nothing is loaded yet, fullText = %v", id, item["fullText"])
		}
	}
}

func TestLoadingTheFullArticleKeepsItAndFetchesOnce(t *testing.T) {
	e := newEnv(t)
	site, hits := articleSite(t)
	b := e.browser()
	mustStatus(t, b.setup("alex", "alex-password-1"), 201)
	fid := newFeed(t, e, 1, "https://example.com/feed")
	id := seedArticle(t, e, fid, "one", site.URL+"/article", "Just a teaser from the feed.", "")

	r := b.do("POST", "/api/items/"+id+"/full", nil)
	mustStatus(t, r, 200)
	item := r.obj("item")
	content, _ := item["content"].(string)
	for _, want := range []string{"competing with governments for the same investors", "who gets the money first"} {
		if !strings.Contains(content, want) {
			t.Errorf("the full article is missing %q:\n%s", want, content)
		}
	}
	for _, unwanted := range []string{"Subscribe to our newsletter", "No comments yet", "<script", "Just a teaser"} {
		if strings.Contains(content, unwanted) {
			t.Errorf("the full article should not contain %q", unwanted)
		}
	}
	if item["fullText"] != true || item["summaryOnly"] != false {
		t.Errorf("fullText = %v, summaryOnly = %v", item["fullText"], item["summaryOnly"])
	}
	if !strings.Contains(content, "https://"+strings.TrimPrefix(site.URL, "http://")) && !strings.Contains(content, site.URL+"/img/bonds.jpg") {
		t.Errorf("the picture's address should be absolute:\n%s", content)
	}

	// Opening the article later shows the kept text, and asking again does not fetch the page again.
	again := b.do("GET", "/api/items/"+id, nil).obj("item")
	if again["fullText"] != true || !strings.Contains(again["content"].(string), "who gets the money first") {
		t.Errorf("the kept article is not shown when opened: %v", again["fullText"])
	}
	mustStatus(t, b.do("POST", "/api/items/"+id+"/full", nil), 200)
	if hits.Load() != 1 {
		t.Errorf("the page was fetched %d times, want once", hits.Load())
	}
}

func TestUnloadingTheFullArticleBringsBackTheFeedsVersion(t *testing.T) {
	e := newEnv(t)
	site, hits := articleSite(t)
	b := e.browser()
	mustStatus(t, b.setup("alex", "alex-password-1"), 201)
	fid := newFeed(t, e, 1, "https://example.com/feed")
	id := seedArticle(t, e, fid, "one", site.URL+"/article", "Just a teaser from the feed.", "")

	mustStatus(t, b.do("POST", "/api/items/"+id+"/full", nil), 200)
	r := b.do("DELETE", "/api/items/"+id+"/full", nil)
	mustStatus(t, r, 200)
	item := r.obj("item")
	if item["fullText"] != false || !strings.Contains(item["content"].(string), "Just a teaser") {
		t.Errorf("after unloading: fullText = %v, content = %v", item["fullText"], item["content"])
	}
	again := b.do("GET", "/api/items/"+id, nil).obj("item")
	if again["fullText"] != false {
		t.Errorf("the full article came back when opened: %v", again["fullText"])
	}
	// Loading it again fetches the page again.
	mustStatus(t, b.do("POST", "/api/items/"+id+"/full", nil), 200)
	if hits.Load() != 2 {
		t.Errorf("the page was fetched %d times, want twice", hits.Load())
	}
	mustStatus(t, b.do("DELETE", "/api/items/999999/full", nil), 404)
	mustStatus(t, e.browser().do("DELETE", "/api/items/"+id+"/full", nil), 401)
}

func TestLoadingTheFullArticleExplainsFailures(t *testing.T) {
	e := newEnv(t)
	site, _ := articleSite(t)
	b := e.browser()
	mustStatus(t, b.setup("alex", "alex-password-1"), 201)
	fid := newFeed(t, e, 1, "https://example.com/feed")

	cases := []struct {
		name, link, message string
	}{
		{"a page with no article", site.URL + "/home", "could not find the article text"},
		{"a page that is gone", site.URL + "/gone", "404"},
		{"something that is not a web page", site.URL + "/data.json", "not a web page"},
		{"an address that is not a web address", "javascript:alert(1)", "no web address"},
		{"no address at all", "", "no web address"},
		{"a site that cannot be reached", "http://127.0.0.1:1/article", "could not be reached"},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id := seedArticle(t, e, fid, fmt.Sprint("fail", i), c.link, "A short summary.", "")
			r := b.do("POST", "/api/items/"+id+"/full", nil)
			mustStatus(t, r, 422)
			if !strings.Contains(r.errMsg(), c.message) {
				t.Errorf("message %q should contain %q", r.errMsg(), c.message)
			}
			// A failure keeps nothing, and the article still opens as before.
			item := b.do("GET", "/api/items/"+id, nil).obj("item")
			if item["fullText"] != false || item["summaryOnly"] != true {
				t.Errorf("after a failure: fullText = %v, summaryOnly = %v", item["fullText"], item["summaryOnly"])
			}
		})
	}
}

func TestTheFullArticleIsSanitized(t *testing.T) {
	e := newEnv(t)
	site, _ := articleSite(t)
	b := e.browser()
	mustStatus(t, b.setup("alex", "alex-password-1"), 201)
	fid := newFeed(t, e, 1, "https://example.com/feed")
	id := seedArticle(t, e, fid, "traps", site.URL+"/unsafe", "A short summary.", "")

	r := b.do("POST", "/api/items/"+id+"/full", nil)
	mustStatus(t, r, 200)
	content := strings.ToLower(r.obj("item")["content"].(string))
	if !strings.Contains(content, "real text of the article") {
		t.Fatalf("the article text is missing: %s", content)
	}
	for _, bad := range []string{"<script", "onerror", "javascript:", "alert(1)", "window.pwned"} {
		if strings.Contains(content, bad) {
			t.Errorf("the full article still contains %q:\n%s", bad, content)
		}
	}
	// The same cleaning applies when it is opened again later.
	later := strings.ToLower(b.do("GET", "/api/items/"+id, nil).obj("item")["content"].(string))
	if strings.Contains(later, "onerror") || strings.Contains(later, "javascript:") {
		t.Errorf("a kept article came back unclean:\n%s", later)
	}
}

func TestFullArticlesBelongToTheirUser(t *testing.T) {
	e := newEnv(t)
	site, hits := articleSite(t)
	alex, sam, _ := adminAndUser(t, e)
	samFeed := newFeed(t, e, 2, "https://example.com/feed") // Sam is the second account
	id := seedArticle(t, e, samFeed, "sams", site.URL+"/article", "A short summary.", "")

	mustStatus(t, alex.do("POST", "/api/items/"+id+"/full", nil), 404)
	if hits.Load() != 0 {
		t.Error("another user's article was fetched")
	}
	mustStatus(t, sam.do("POST", "/api/items/"+id+"/full", nil), 200)
	mustStatus(t, e.browser().do("POST", "/api/items/"+id+"/full", nil), 401)
}

func TestLoadingFullArticlesRespectsThePrivateNetworkSetting(t *testing.T) {
	e := newEnv(t)
	e.s.refresh.Fetcher.BlockPrivateNetworks()
	site, hits := articleSite(t) // on 127.0.0.1
	b := e.browser()
	mustStatus(t, b.setup("alex", "alex-password-1"), 201)
	fid := newFeed(t, e, 1, "https://example.com/feed")
	id := seedArticle(t, e, fid, "local", site.URL+"/article", "A short summary.", "")

	r := b.do("POST", "/api/items/"+id+"/full", nil)
	mustStatus(t, r, 422)
	if !strings.Contains(r.errMsg(), "private network") {
		t.Errorf("message = %q", r.errMsg())
	}
	if hits.Load() != 0 {
		t.Error("the page on a private network was fetched")
	}
}

func TestFeedFullTextSwitchThroughTheAPI(t *testing.T) {
	e := newEnv(t)
	alex, sam, _ := adminAndUser(t, e)
	mine := newFeed(t, e, 1, "https://example.com/feed")
	theirs := newFeed(t, e, 2, "https://example.com/feed")

	listed := func(c *client) bool {
		return c.do("GET", "/api/feeds", nil).list("feeds")[0]["fullText"] == true
	}
	if listed(alex) {
		t.Fatal("full articles are off by default")
	}
	r := alex.do("PATCH", fmt.Sprintf("/api/feeds/%d", mine), m{"fullText": true})
	mustStatus(t, r, 200)
	if r.obj("feed")["fullText"] != true || !listed(alex) {
		t.Errorf("the switch did not turn on: %v", r.obj("feed"))
	}
	if listed(sam) {
		t.Error("another user's feed was switched")
	}
	mustStatus(t, alex.do("PATCH", fmt.Sprintf("/api/feeds/%d", theirs), m{"fullText": true}), 404)
	mustStatus(t, alex.do("PATCH", fmt.Sprintf("/api/feeds/%d", mine), m{}), 400)
	mustStatus(t, alex.do("PATCH", fmt.Sprintf("/api/feeds/%d", mine), m{"fullText": false}), 200)
	if listed(alex) {
		t.Error("the switch did not turn off")
	}
	mustStatus(t, e.browser().do("PATCH", fmt.Sprintf("/api/feeds/%d", mine), m{"fullText": true}), 401)
}
