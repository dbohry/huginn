package server

import "testing"

func TestBlockedWordsHideArticlesWholeWordOnly(t *testing.T) {
	e := newEnv(t)
	site := feedSite(t)
	alex, sam, _ := adminAndUser(t, e)
	for _, c := range []*client{alex, sam} {
		mustStatus(t, c.do("POST", "/api/feeds", m{"url": site.URL + "/rss"}), 201)
	}
	titles := func(c *client) []string {
		var out []string
		for _, it := range c.do("GET", "/api/items", nil).list("items") {
			out = append(out, it["title"].(string))
		}
		return out
	}
	unread := func(c *client) float64 {
		return c.do("GET", "/api/feeds", nil).list("feeds")[0]["unread"].(float64)
	}
	if len(titles(alex)) != 2 || unread(alex) != 2 {
		t.Fatalf("before blocking: %v, %v unread", titles(alex), unread(alex))
	}

	// Case and spacing do not matter; a part of a word does not match.
	r := alex.do("POST", "/api/blocked-words", m{"word": "  FIRST  "})
	mustStatus(t, r, 201)
	if r.obj("word")["word"] != "first" {
		t.Errorf("word = %v, want first", r.obj("word"))
	}
	mustStatus(t, alex.do("POST", "/api/blocked-words", m{"word": "first"}), 409)
	mustStatus(t, alex.do("POST", "/api/blocked-words", m{"word": "   "}), 400)
	mustStatus(t, alex.do("POST", "/api/blocked-words", m{"word": "gui"}), 201)

	// "First post" is hidden; "gui" is only part of the word "guid".
	got := titles(alex)
	if len(got) != 1 || got[0] != "No guid, no date" {
		t.Fatalf("after blocking: %v", got)
	}
	if n := unread(alex); n != 1 {
		t.Errorf("unread = %v, want 1: counts must match the list", n)
	}
	if n := len(alex.do("GET", "/api/items?unread=1", nil).list("items")); n != 1 {
		t.Errorf("unread list has %d, want 1", n)
	}

	// Blocking is per user.
	if len(titles(sam)) != 2 || unread(sam) != 2 {
		t.Errorf("Sam is affected by Alex's words: %v", titles(sam))
	}

	// Marking everything read leaves hidden articles alone.
	mustStatus(t, alex.do("POST", "/api/read-all", m{}), 200)
	words := alex.do("GET", "/api/blocked-words", nil).list("words")
	if len(words) != 2 {
		t.Fatalf("words = %v", words)
	}
	for _, w := range words {
		mustStatus(t, alex.do("DELETE", "/api/blocked-words/"+idOf(w["id"]), nil), 204)
	}
	if n := unread(alex); n != 1 {
		t.Errorf("after unblocking, unread = %v, want 1: only the hidden one was left unread", n)
	}
	mustStatus(t, sam.do("DELETE", "/api/blocked-words/"+idOf(words[0]["id"]), nil), 404)
}
