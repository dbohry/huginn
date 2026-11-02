package server

import (
	"context"
	"testing"
)

func TestUsersOnlySeeTheirOwnFeeds(t *testing.T) {
	e := newEnv(t)
	alex, sam, samID := adminAndUser(t, e)
	ctx := context.Background()

	alexFeed, err := e.s.store.CreateFeed(ctx, 1, "https://alex.example/feed")
	if err != nil {
		t.Fatal(err)
	}
	// The same address can be followed by two users, each with their own copy.
	if _, err := e.s.store.CreateFeed(ctx, 2, "https://alex.example/feed"); err != nil {
		t.Fatalf("a second user following the same address: %v", err)
	}
	_ = samID
	_ = alexFeed

	for name, c := range map[string]*client{"alex": alex, "sam": sam} {
		r := c.do("GET", "/api/feeds", nil)
		mustStatus(t, r, 200)
		feeds, _ := r.json()["feeds"].([]any)
		if len(feeds) != 1 {
			t.Errorf("%s sees %d feeds, want 1", name, len(feeds))
		}
	}
}

func TestFeedsNeedASession(t *testing.T) {
	e := newEnv(t)
	mustStatus(t, e.browser().do("GET", "/api/feeds", nil), 401)
}
