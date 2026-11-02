package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dbohry/huginn/internal/auth"
	"github.com/dbohry/huginn/internal/db"
	"github.com/dbohry/huginn/internal/store"
)

// env is one server with its own database and a clock the test controls.
type env struct {
	t     *testing.T
	s     *server
	h     http.Handler
	clock time.Time
}

func newEnv(t *testing.T) *env {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := db.Migrate(conn); err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, clock: time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)}
	e.s = newServer(conn, "test")
	e.s.now = func() time.Time { return e.clock }
	e.h = e.s.routes(fstest.MapFS{"index.html": {Data: []byte("<title>x</title>")}})
	return e
}

// client is a browser: it keeps the session cookie between requests.
type client struct {
	e      *env
	cookie *http.Cookie
}

func (e *env) browser() *client { return &client{e: e} }

type resp struct {
	*httptest.ResponseRecorder
}

func (r resp) json() map[string]any {
	var m map[string]any
	json.Unmarshal(r.Body.Bytes(), &m)
	return m
}

func (r resp) errMsg() string {
	s, _ := r.json()["error"].(string)
	return s
}

func (r resp) user() map[string]any {
	u, _ := r.json()["user"].(map[string]any)
	return u
}

func (r resp) sessionCookie() *http.Cookie {
	for _, c := range r.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	return nil
}

func (c *client) do(method, path string, body any, headers ...string) resp {
	c.e.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	if c.cookie != nil {
		req.AddCookie(c.cookie)
	}
	rec := httptest.NewRecorder()
	c.e.h.ServeHTTP(rec, req)

	r := resp{rec}
	if sc := r.sessionCookie(); sc != nil {
		if sc.MaxAge < 0 {
			c.cookie = nil
		} else {
			c.cookie = &http.Cookie{Name: sc.Name, Value: sc.Value}
		}
	}
	return r
}

type m map[string]any

func (c *client) setup(username, password string) resp {
	return c.do("POST", "/api/setup", m{"username": username, "password": password})
}

func (c *client) login(username, password string, remember bool) resp {
	return c.do("POST", "/api/login", m{"username": username, "password": password, "remember": remember})
}

func mustStatus(t *testing.T, r resp, want int) {
	t.Helper()
	if r.Code != want {
		t.Fatalf("status = %d, want %d (body %s)", r.Code, want, r.Body.String())
	}
}

func TestSetupFlow(t *testing.T) {
	e := newEnv(t)
	b := e.browser()

	r := b.do("GET", "/api/setup/status", nil)
	if mustStatus(t, r, 200); r.json()["needsSetup"] != true {
		t.Error("a fresh install must need setup")
	}

	mustStatus(t, b.setup("ab", "long-enough-password"), 400)        // username too short
	mustStatus(t, b.setup("has space", "long-enough-password"), 400) // bad characters
	r = b.setup("alex", "short")
	mustStatus(t, r, 400)
	if r.errMsg() != passwordRule {
		t.Errorf("message = %q", r.errMsg())
	}
	mustStatus(t, b.setup("alex", strings.Repeat("x", 129)), 400)

	// JSON only.
	req := httptest.NewRequest("POST", "/api/setup", strings.NewReader("username=a"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("form post: status = %d, want 415", rec.Code)
	}

	r = b.setup("Alex", "long-enough-password")
	mustStatus(t, r, 201)
	if u := r.user(); u["username"] != "Alex" || u["isAdmin"] != true || u["mustChangePassword"] != false {
		t.Errorf("user = %v", u)
	}
	c := r.sessionCookie()
	if c == nil || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.Secure {
		t.Errorf("cookie = %+v", c)
	}
	if c.MaxAge != int(persistentTTL.Seconds()) {
		t.Errorf("setup should sign in for 30 days, MaxAge = %d", c.MaxAge)
	}

	// Signed in straight away.
	r = b.do("GET", "/api/me", nil)
	mustStatus(t, r, 200)
	if r.user()["username"] != "Alex" {
		t.Errorf("me = %v", r.json())
	}

	// Setup is closed for good.
	if r = b.do("GET", "/api/setup/status", nil); r.json()["needsSetup"] != false {
		t.Error("setup should be closed")
	}
	r = e.browser().setup("mallory", "long-enough-password")
	mustStatus(t, r, 409)
}

func TestLoginAndLogout(t *testing.T) {
	e := newEnv(t)
	e.browser().setup("alex", "correct-password")

	b := e.browser()
	mustStatus(t, b.do("GET", "/api/me", nil), 401)

	wrongPass := b.login("alex", "wrong-password", false)
	unknown := b.login("nobody", "wrong-password", false)
	mustStatus(t, wrongPass, 401)
	mustStatus(t, unknown, 401)
	if wrongPass.Body.String() != unknown.Body.String() {
		t.Errorf("responses differ, which reveals which usernames exist:\n%s\n%s", wrongPass.Body, unknown.Body)
	}
	mustStatus(t, b.login("", "x", false), 400)
	mustStatus(t, b.login("alex", "", false), 400)

	// Without "Keep me signed in" the cookie ends with the browser session.
	r := b.login("ALEX", "correct-password", false)
	mustStatus(t, r, 200)
	if c := r.sessionCookie(); c == nil || c.MaxAge != 0 {
		t.Errorf("cookie = %+v, want a session cookie", c)
	}
	mustStatus(t, b.do("GET", "/api/me", nil), 200)

	mustStatus(t, b.do("POST", "/api/logout", nil), 204)
	mustStatus(t, b.do("GET", "/api/me", nil), 401)

	// With it, the cookie lasts 30 days.
	r = b.login("alex", "correct-password", true)
	if c := r.sessionCookie(); c == nil || c.MaxAge != int(persistentTTL.Seconds()) {
		t.Errorf("cookie = %+v, want 30 days", c)
	}

	// Logging out drops the session on the server, not just the cookie.
	stolen := *b.cookie
	b.do("POST", "/api/logout", nil)
	other := &client{e: e, cookie: &stolen}
	mustStatus(t, other.do("GET", "/api/me", nil), 401)
}

func TestTokenIsStoredHashed(t *testing.T) {
	e := newEnv(t)
	b := e.browser()
	b.setup("alex", "correct-password")

	var n int
	err := e.s.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE token_hash = ?`, b.cookie.Value).Scan(&n)
	if err != nil || n != 0 {
		t.Errorf("the raw token is in the database (n=%d, err=%v)", n, err)
	}
	err = e.s.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE token_hash = ?`, store.HashToken(b.cookie.Value)).Scan(&n)
	if err != nil || n != 1 {
		t.Errorf("hashed token missing (n=%d, err=%v)", n, err)
	}
}

func TestLoginRateLimit(t *testing.T) {
	e := newEnv(t)
	e.browser().setup("alex", "correct-password")
	b := e.browser()

	for i := 0; i < 10; i++ {
		mustStatus(t, b.login("alex", "wrong-"+string(rune('a'+i)), false), 401)
	}
	r := b.login("alex", "correct-password", false)
	mustStatus(t, r, 429)
	if r.Header().Get("Retry-After") == "" || !strings.Contains(r.errMsg(), "Try again in") {
		t.Errorf("headers=%v body=%s", r.Header(), r.Body)
	}
	if b.cookie != nil {
		t.Error("a blocked attempt must not sign anyone in")
	}
}

func TestSuccessfulLoginClearsFailures(t *testing.T) {
	e := newEnv(t)
	e.browser().setup("alex", "correct-password")
	b := e.browser()

	for round := 0; round < 3; round++ {
		for i := 0; i < 8; i++ {
			mustStatus(t, b.login("alex", "wrong", false), 401)
		}
		mustStatus(t, b.login("alex", "correct-password", false), 200)
	}
}

func TestOriginCheck(t *testing.T) {
	e := newEnv(t)
	b := e.browser()

	mustStatus(t, b.do("POST", "/api/login", m{"username": "a", "password": "b"}, "Origin", "http://evil.example"), 403)
	mustStatus(t, b.do("POST", "/api/login", m{"username": "a", "password": "b"}, "Sec-Fetch-Site", "cross-site"), 403)
	// Same origin is fine (the test request's host is example.com).
	mustStatus(t, b.do("POST", "/api/login", m{"username": "a", "password": "b"}, "Origin", "http://example.com"), 401)
	mustStatus(t, b.do("POST", "/api/login", m{"username": "a", "password": "b"}, "Sec-Fetch-Site", "same-origin"), 401)
	// Reading is never blocked.
	mustStatus(t, b.do("GET", "/api/setup/status", nil, "Origin", "http://evil.example"), 200)
}

func TestSecureCookieBehindHTTPSProxy(t *testing.T) {
	e := newEnv(t)
	r := e.browser().do("POST", "/api/setup", m{"username": "alex", "password": "correct-password"}, "X-Forwarded-Proto", "https")
	mustStatus(t, r, 201)
	if c := r.sessionCookie(); c == nil || !c.Secure {
		t.Errorf("cookie = %+v, want Secure", c)
	}
}

func TestSessionLifetimeAndExtension(t *testing.T) {
	e := newEnv(t)
	e.browser().setup("alex", "correct-password")
	start := e.clock

	b := e.browser()
	b.login("alex", "correct-password", false)

	// Used within the day: stays valid, and the expiry moves forward.
	e.clock = start.Add(6 * time.Hour)
	r := b.do("GET", "/api/me", nil)
	mustStatus(t, r, 200)
	if r.sessionCookie() == nil {
		t.Error("an extended session should refresh its cookie")
	}
	_, _, err := e.s.store.SessionUser(context.Background(), store.HashToken(b.cookie.Value), start.Add(17*time.Hour))
	if err != nil {
		t.Errorf("session was not extended: %v", err)
	}

	// Checking again straight away does not write again.
	e.clock = start.Add(6*time.Hour + time.Minute)
	if r = b.do("GET", "/api/me", nil); r.sessionCookie() != nil {
		t.Error("the session should not be extended on every request")
	}

	// Left alone for longer than 12 hours: gone, and the cookie is cleared.
	e.clock = start.Add(6*time.Hour + 13*time.Hour)
	r = b.do("GET", "/api/me", nil)
	mustStatus(t, r, 401)
	if c := r.sessionCookie(); c == nil || c.MaxAge >= 0 {
		t.Errorf("expired session should clear the cookie, got %+v", c)
	}
}

func TestTemporaryPasswordRestrictions(t *testing.T) {
	e := newEnv(t)
	b := e.browser()
	b.setup("alex", "correct-password")
	ctx := context.Background()

	temp, err := auth.ResetPassword(ctx, e.s.store, "alex")
	if err != nil {
		t.Fatal(err)
	}
	mustStatus(t, b.do("GET", "/api/me", nil), 401) // reset signed everyone out

	r := b.login("alex", temp, false)
	mustStatus(t, r, 200)
	if r.user()["mustChangePassword"] != true {
		t.Errorf("user = %v", r.user())
	}
	mustStatus(t, b.do("GET", "/api/me", nil), 200) // allowed, so the app can show the change screen

	// Anything else that needs a session is refused until the password changes.
	other := e.s.authed(false, func(w http.ResponseWriter, r *http.Request, _ store.User, _ store.Session) {
		w.WriteHeader(204)
	})
	req := httptest.NewRequest("GET", "/x", nil)
	req.AddCookie(b.cookie)
	rec := httptest.NewRecorder()
	other.ServeHTTP(rec, req)
	if rec.Code != 403 || !strings.Contains(rec.Body.String(), "password_change_required") {
		t.Errorf("status = %d, body = %s", rec.Code, rec.Body)
	}
}

func TestChangePassword(t *testing.T) {
	e := newEnv(t)
	laptop := e.browser()
	laptop.setup("alex", "old-password-1")
	phone := e.browser()
	phone.login("alex", "old-password-1", false)

	mustStatus(t, e.browser().do("POST", "/api/me/password", m{"current": "x", "new": "y"}), 401)

	// A wrong current password is 403, not 401: the user is still signed in.
	r := laptop.do("POST", "/api/me/password", m{"current": "not-it", "new": "new-password-2"})
	mustStatus(t, r, 403)
	mustStatus(t, laptop.do("GET", "/api/me", nil), 200)
	mustStatus(t, laptop.do("POST", "/api/me/password", m{"current": "old-password-1", "new": "short"}), 400)
	r = laptop.do("POST", "/api/me/password", m{"current": "old-password-1", "new": "old-password-1"})
	mustStatus(t, r, 400)

	mustStatus(t, laptop.do("POST", "/api/me/password", m{"current": "old-password-1", "new": "new-password-2"}), 204)

	// This browser stays signed in. The other one is signed out.
	mustStatus(t, laptop.do("GET", "/api/me", nil), 200)
	mustStatus(t, phone.do("GET", "/api/me", nil), 401)

	mustStatus(t, e.browser().login("alex", "old-password-1", false), 401)
	mustStatus(t, e.browser().login("alex", "new-password-2", false), 200)
}

func TestChangingTheTemporaryPasswordClearsTheFlag(t *testing.T) {
	e := newEnv(t)
	e.browser().setup("alex", "correct-password")
	temp, _ := auth.ResetPassword(context.Background(), e.s.store, "alex")

	b := e.browser()
	b.login("alex", temp, false)
	mustStatus(t, b.do("POST", "/api/me/password", m{"current": temp, "new": "my-own-password"}), 204)

	r := b.do("GET", "/api/me", nil)
	if r.user()["mustChangePassword"] != false {
		t.Errorf("user = %v", r.user())
	}
}

func TestChangePasswordRateLimit(t *testing.T) {
	e := newEnv(t)
	b := e.browser()
	b.setup("alex", "correct-password")

	for i := 0; i < 10; i++ {
		mustStatus(t, b.do("POST", "/api/me/password", m{"current": "guess", "new": "new-password-2"}), 403)
	}
	mustStatus(t, b.do("POST", "/api/me/password", m{"current": "correct-password", "new": "new-password-2"}), 429)
}

func TestUnknownAPIRoutesAndMethods(t *testing.T) {
	e := newEnv(t)
	b := e.browser()
	mustStatus(t, b.do("GET", "/api/nope", nil), 404)
	mustStatus(t, b.do("GET", "/api/login", nil), 404)
}
