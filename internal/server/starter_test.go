package server

import (
	"context"
	"strings"
	"testing"

	"github.com/dbohry/huginn/internal/store"
)

func seedStarter(t *testing.T, e *env) {
	t.Helper()
	err := e.s.store.SeedStarterFeeds(context.Background(), []store.StarterFeed{
		{Category: "Technology", Title: "Tech One", URL: "https://tech.example/one.xml"},
		{Category: "Technology", Title: "Tech Two", URL: "https://tech.example/two.xml"},
		{Category: "Food", Title: "Food One", URL: "https://food.example/rss"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func topics(r resp) []map[string]any {
	return r.list("topics")
}

func TestStarterIsOfferedOnceToNewAccounts(t *testing.T) {
	e := newEnv(t)
	alex := e.browser()
	r := alex.setup("alex", "alex-password-1")
	mustStatus(t, r, 201)
	if r.user()["starterDone"] != false {
		t.Fatalf("a new account is offered the starter feeds: %v", r.user())
	}

	mustStatus(t, alex.do("POST", "/api/starter/dismiss", nil), 204)
	me := alex.do("GET", "/api/me", nil)
	if me.user()["starterDone"] != true {
		t.Errorf("after dismissing: %v", me.user())
	}
}

func TestStarterListIsGroupedByTopicAndMarksFollowedFeeds(t *testing.T) {
	e := newEnv(t)
	seedStarter(t, e)
	alex := e.browser()
	mustStatus(t, alex.setup("alex", "alex-password-1"), 201)
	if _, err := e.s.store.CreateFeed(context.Background(), 1, "https://food.example/rss"); err != nil {
		t.Fatal(err)
	}

	r := alex.do("GET", "/api/starter", nil)
	mustStatus(t, r, 200)
	got := topics(r)
	if len(got) != 2 || got[0]["name"] != "Technology" || got[1]["name"] != "Food" {
		t.Fatalf("topics = %v", got)
	}
	if n := len(got[0]["feeds"].([]any)); n != 2 {
		t.Errorf("Technology has %d feeds, want 2", n)
	}
	food := got[1]["feeds"].([]any)[0].(map[string]any)
	if food["followed"] != true {
		t.Errorf("an already followed feed is not marked: %v", food)
	}
}

func TestSubscribingToStarterFeedsFilesThemUnderTheirTopic(t *testing.T) {
	e := newEnv(t)
	seedStarter(t, e)
	alex := e.browser()
	mustStatus(t, alex.setup("alex", "alex-password-1"), 201)

	list := topics(alex.do("GET", "/api/starter", nil))
	tech := list[0]["feeds"].([]any)[0].(map[string]any)
	food := list[1]["feeds"].([]any)[0].(map[string]any)

	r := alex.do("POST", "/api/starter/subscribe", m{"ids": []any{tech["id"], food["id"], 9999}})
	mustStatus(t, r, 200)
	e.s.bg.Wait()
	if r.json()["added"] != float64(2) || r.json()["existing"] != float64(0) {
		t.Fatalf("result = %v", r.json())
	}

	feeds := alex.do("GET", "/api/feeds", nil).list("feeds")
	byTitle := map[string]string{}
	for _, f := range feeds {
		byTitle[f["title"].(string)] = f["category"].(string)
	}
	if byTitle["Tech One"] != "Technology" || byTitle["Food One"] != "Food" || len(byTitle) != 2 {
		t.Errorf("feeds = %v", byTitle)
	}
	if alex.do("GET", "/api/me", nil).user()["starterDone"] != true {
		t.Error("subscribing does not end the first-login offer")
	}

	// Choosing the same feeds again follows nothing twice.
	r = alex.do("POST", "/api/starter/subscribe", m{"ids": []any{tech["id"]}})
	if r.json()["added"] != float64(0) || r.json()["existing"] != float64(1) {
		t.Errorf("second time = %v", r.json())
	}
}

func TestOnlyAdministratorsEditTheStarterList(t *testing.T) {
	e := newEnv(t)
	seedStarter(t, e)
	alex, sam, _ := adminAndUser(t, e)

	for _, c := range []struct{ method, path string }{
		{"POST", "/api/starter/feeds"}, {"DELETE", "/api/starter/feeds/1"},
		{"POST", "/api/starter/reset"}, {"GET", "/api/starter/opml"}, {"POST", "/api/starter/opml"},
	} {
		if r := sam.do(c.method, c.path, m{"category": "x"}); r.Code != 403 {
			t.Errorf("%s %s as a plain user: %d", c.method, c.path, r.Code)
		}
	}

	r := alex.do("POST", "/api/starter/feeds", m{"category": "Music", "title": "Pitchfork", "url": "pitchfork.com/rss"})
	mustStatus(t, r, 201)
	id := idOf(r.obj("feed")["id"])
	if got := r.obj("feed")["url"]; got != "https://pitchfork.com/rss" {
		t.Errorf("url = %v", got)
	}
	mustStatus(t, alex.do("POST", "/api/starter/feeds", m{"category": "Music", "url": "https://pitchfork.com/rss"}), 409)
	mustStatus(t, alex.do("POST", "/api/starter/feeds", m{"category": "Music", "url": "not a url"}), 400)
	mustStatus(t, alex.do("POST", "/api/starter/feeds", m{"category": "", "url": "https://x.example/rss"}), 400)

	// Everyone sees the addition.
	if n := len(topics(sam.do("GET", "/api/starter", nil))); n != 3 {
		t.Errorf("%d topics after adding Music, want 3", n)
	}

	mustStatus(t, alex.do("DELETE", "/api/starter/feeds/"+id, nil), 204)
	mustStatus(t, alex.do("DELETE", "/api/starter/feeds/"+id, nil), 404)
	if n := len(topics(sam.do("GET", "/api/starter", nil))); n != 2 {
		t.Errorf("%d topics after removing Music, want 2", n)
	}
}

func TestStarterListCanBeReplacedFromOPMLAndReset(t *testing.T) {
	e := newEnv(t)
	seedStarter(t, e)
	alex := e.browser()
	mustStatus(t, alex.setup("alex", "alex-password-1"), 201)

	file := `<?xml version="1.0"?><opml version="2.0"><body>
	  <outline text="Music"><outline type="rss" text="Pitchfork" xmlUrl="https://pitchfork.com/rss"/></outline>
	  <outline type="rss" text="Loose" xmlUrl="https://loose.example/rss"/>
	  <outline type="rss" text="Bad" xmlUrl="ftp://bad.example/rss"/>
	</body></opml>`
	r := alex.raw("POST", "/api/starter/opml", "text/xml", file)
	mustStatus(t, r, 200)
	if r.json()["feeds"] != float64(2) || r.json()["skipped"] != float64(1) {
		t.Fatalf("result = %v", r.json())
	}
	got := topics(alex.do("GET", "/api/starter", nil))
	if len(got) != 2 || got[0]["name"] != "Music" || got[1]["name"] != "Other" {
		t.Fatalf("topics = %v", got)
	}

	mustStatus(t, alex.raw("POST", "/api/starter/opml", "text/xml", "<opml version=\"2.0\"><body/></opml>"), 400)

	out := alex.do("GET", "/api/starter/opml", nil)
	mustStatus(t, out, 200)
	if !strings.Contains(out.Body.String(), `text="Music"`) {
		t.Errorf("export has no Music folder:\n%s", out.Body)
	}

	mustStatus(t, alex.do("POST", "/api/starter/reset", nil), 204)
	if n := len(topics(alex.do("GET", "/api/starter", nil))); n < 5 {
		t.Errorf("after reset the built-in list has %d topics", n)
	}
}

func TestStarterSeedingHappensOnce(t *testing.T) {
	e := newEnv(t)
	seedStarter(t, e)
	ctx := context.Background()
	for _, f := range mustList(t, e) {
		if err := e.s.store.DeleteStarterFeed(ctx, f.ID); err != nil {
			t.Fatal(err)
		}
	}
	seedStarter(t, e) // an emptied list stays empty
	if n := len(mustList(t, e)); n != 0 {
		t.Errorf("%d feeds came back after the administrator removed them all", n)
	}
}

func mustList(t *testing.T, e *env) []store.StarterFeed {
	t.Helper()
	l, err := e.s.store.ListStarterFeeds(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestFeedCategoryCanBeChanged(t *testing.T) {
	e := newEnv(t)
	alex := e.browser()
	mustStatus(t, alex.setup("alex", "alex-password-1"), 201)
	id, err := e.s.store.CreateFeed(context.Background(), 1, "https://a.example/feed")
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/feeds/" + idOf(float64(id))

	r := alex.do("PATCH", path, m{"category": "  Reading   list "})
	mustStatus(t, r, 200)
	if got := r.obj("feed")["category"]; got != "Reading list" {
		t.Errorf("category = %q", got)
	}
	// Setting only the topic leaves the full-text setting alone.
	if r.obj("feed")["fullText"] != false {
		t.Errorf("fullText changed: %v", r.obj("feed"))
	}
	mustStatus(t, alex.do("PATCH", path, m{"category": strings.Repeat("x", 41)}), 400)
	mustStatus(t, alex.do("PATCH", path, m{}), 400)
	r = alex.do("PATCH", path, m{"category": ""})
	mustStatus(t, r, 200)
	if r.obj("feed")["category"] != "" {
		t.Errorf("category not cleared: %v", r.obj("feed"))
	}
}
