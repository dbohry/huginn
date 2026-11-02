package server

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/dbohry/huginn/internal/auth"
)

func (r resp) users() []map[string]any {
	var out []map[string]any
	list, _ := r.json()["users"].([]any)
	for _, u := range list {
		out = append(out, u.(map[string]any))
	}
	return out
}

func idOf(v any) string { return fmt.Sprint(int64(v.(float64))) }

// adminAndUser sets up an administrator (alex) and a plain user (sam) who has
// already chosen their own password, each in their own browser.
func adminAndUser(t *testing.T, e *env) (alex, sam *client, samID string) {
	t.Helper()
	alex = e.browser()
	mustStatus(t, alex.setup("alex", "alex-password-1"), 201)

	r := alex.do("POST", "/api/users", m{"username": "sam"})
	mustStatus(t, r, 201)
	samID = idOf(r.user()["id"])
	temp := r.json()["temporaryPassword"].(string)

	sam = e.browser()
	mustStatus(t, sam.login("sam", temp, false), 200)
	mustStatus(t, sam.do("POST", "/api/me/password", m{"current": temp, "new": "sam-password-1"}), 204)
	return alex, sam, samID
}

func TestUsersEndpointsAreForAdministrators(t *testing.T) {
	e := newEnv(t)
	alex, sam, _ := adminAndUser(t, e)

	anon := e.browser()
	for _, c := range []struct{ method, path string }{
		{"GET", "/api/users"}, {"POST", "/api/users"},
		{"POST", "/api/users/1/reset-password"}, {"DELETE", "/api/users/1"},
	} {
		mustStatus(t, anon.do(c.method, c.path, m{"username": "x"}), 401)
		r := sam.do(c.method, c.path, m{"username": "newuser"})
		if r.Code != 403 || r.errMsg() != "Only administrators can do that." {
			t.Errorf("%s %s as a plain user: %d %s", c.method, c.path, r.Code, r.Body)
		}
	}

	// And nothing the plain user tried had any effect.
	r := alex.do("GET", "/api/users", nil)
	mustStatus(t, r, 200)
	if len(r.users()) != 2 {
		t.Errorf("users = %v", r.users())
	}
}

func TestAddingAUser(t *testing.T) {
	e := newEnv(t)
	alex := e.browser()
	alex.setup("alex", "alex-password-1")

	r := alex.do("POST", "/api/users", m{"username": "Sam", "isAdmin": false})
	mustStatus(t, r, 201)
	temp, _ := r.json()["temporaryPassword"].(string)
	if len(temp) != 19 {
		t.Errorf("temporary password %q", temp)
	}
	if u := r.user(); u["username"] != "Sam" || u["isAdmin"] != false || u["mustChangePassword"] != true || u["lastSeenAt"] != nil {
		t.Errorf("user = %v", u)
	}

	// The temporary password works once, and forces a change.
	sam := e.browser()
	r = sam.login("sam", temp, false)
	mustStatus(t, r, 200)
	if r.user()["mustChangePassword"] != true {
		t.Errorf("user = %v", r.user())
	}
	mustStatus(t, sam.do("GET", "/api/feeds", nil), 403)

	// Administrators can be created too.
	r = alex.do("POST", "/api/users", m{"username": "joey", "isAdmin": true})
	mustStatus(t, r, 201)
	if r.user()["isAdmin"] != true {
		t.Errorf("user = %v", r.user())
	}
}

func TestAddingAUserValidates(t *testing.T) {
	e := newEnv(t)
	alex := e.browser()
	alex.setup("alex", "alex-password-1")

	for name, body := range map[string]m{
		"too short": {"username": "ab"},
		"bad chars": {"username": "has space"},
		"empty":     {"username": ""},
		"too long":  {"username": "abcdefghijklmnopqrstuvwxyz0123456789"},
	} {
		if r := alex.do("POST", "/api/users", body); r.Code != 400 || r.errMsg() != usernameRule {
			t.Errorf("%s: %d %s", name, r.Code, r.Body)
		}
	}
	for _, taken := range []string{"alex", "ALEX", "Alex"} {
		r := alex.do("POST", "/api/users", m{"username": taken})
		if r.Code != 409 || r.errMsg() != "That username is already taken." {
			t.Errorf("%s: %d %s", taken, r.Code, r.Body)
		}
	}
	if r := alex.do("GET", "/api/users", nil); len(r.users()) != 1 {
		t.Errorf("rejected requests created users: %v", r.users())
	}
}

func TestListShowsLastSeen(t *testing.T) {
	e := newEnv(t)
	alex, _, _ := adminAndUser(t, e)

	users := alex.do("GET", "/api/users", nil).users()
	if len(users) != 2 || users[0]["username"] != "alex" || users[1]["username"] != "sam" {
		t.Fatalf("users = %v", users)
	}
	for _, u := range users {
		if u["lastSeenAt"] == nil {
			t.Errorf("%v has signed in, so lastSeenAt should be set", u["username"])
		}
		if _, leaked := u["passwordHash"]; leaked {
			t.Error("the password hash must never be sent")
		}
	}
	if users[0]["lastSeenAt"] != float64(e.clock.Unix()) {
		t.Errorf("alex lastSeenAt = %v, want %d", users[0]["lastSeenAt"], e.clock.Unix())
	}

	// Using the app later moves it forward (sessions are extended every few minutes).
	e.clock = e.clock.Add(2 * time.Hour)
	alex.do("GET", "/api/feeds", nil)
	users = alex.do("GET", "/api/users", nil).users()
	if users[0]["lastSeenAt"] != float64(e.clock.Unix()) {
		t.Errorf("alex lastSeenAt = %v, want %d", users[0]["lastSeenAt"], e.clock.Unix())
	}
}

func TestResettingAnotherUsersPassword(t *testing.T) {
	e := newEnv(t)
	alex, sam, samID := adminAndUser(t, e)
	mustStatus(t, sam.do("GET", "/api/feeds", nil), 200)

	r := alex.do("POST", "/api/users/"+samID+"/reset-password", nil)
	mustStatus(t, r, 200)
	temp := r.json()["temporaryPassword"].(string)

	// Sam is signed out everywhere, the old password is dead, the new one forces a change.
	mustStatus(t, sam.do("GET", "/api/feeds", nil), 401)
	mustStatus(t, e.browser().login("sam", "sam-password-1", false), 401)
	b := e.browser()
	r = b.login("sam", temp, false)
	mustStatus(t, r, 200)
	if r.user()["mustChangePassword"] != true {
		t.Errorf("user = %v", r.user())
	}

	mustStatus(t, alex.do("POST", "/api/users/999/reset-password", nil), 404)
	mustStatus(t, alex.do("POST", "/api/users/abc/reset-password", nil), 404)
}

func TestAnAdministratorCannotResetOrRemoveThemselves(t *testing.T) {
	e := newEnv(t)
	alex := e.browser()
	r := alex.setup("alex", "alex-password-1")
	alexID := idOf(r.user()["id"])

	mustStatus(t, alex.do("POST", "/api/users/"+alexID+"/reset-password", nil), 400)
	mustStatus(t, alex.do("DELETE", "/api/users/"+alexID, nil), 400)

	// Still signed in, with the same password.
	mustStatus(t, alex.do("GET", "/api/me", nil), 200)
	mustStatus(t, e.browser().login("alex", "alex-password-1", false), 200)
}

func TestRemovingAUser(t *testing.T) {
	e := newEnv(t)
	alex, sam, samID := adminAndUser(t, e)
	ctx := context.Background()
	sid, _ := strconv.ParseInt(samID, 10, 64)
	if _, err := e.s.store.CreateFeed(ctx, sid, "https://sam.example/feed"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.store.CreateFeed(ctx, 1, "https://alex.example/feed"); err != nil { // alex is the first account
		t.Fatal(err)
	}

	mustStatus(t, alex.do("DELETE", "/api/users/"+samID, nil), 204)

	mustStatus(t, sam.do("GET", "/api/me", nil), 401)
	mustStatus(t, e.browser().login("sam", "sam-password-1", false), 401)
	if r := alex.do("GET", "/api/users", nil); len(r.users()) != 1 {
		t.Errorf("users = %v", r.users())
	}
	var n int
	e.s.db.QueryRow(`SELECT COUNT(*) FROM feeds`).Scan(&n)
	if n != 1 {
		t.Errorf("%d feeds left, want only alex's", n)
	}
	mustStatus(t, alex.do("DELETE", "/api/users/"+samID, nil), 404)

	// The name is free to use again.
	mustStatus(t, alex.do("POST", "/api/users", m{"username": "sam"}), 201)
}

func TestAdminEndpointsAreBlockedWhileThePasswordIsTemporary(t *testing.T) {
	e := newEnv(t)
	alex := e.browser()
	alex.setup("alex", "alex-password-1")
	temp, _ := auth.ResetPassword(context.Background(), e.s.store, "alex")

	b := e.browser()
	b.login("alex", temp, false)
	mustStatus(t, b.do("GET", "/api/users", nil), 403)
	mustStatus(t, b.do("POST", "/api/users", m{"username": "x1234"}), 403)
}
