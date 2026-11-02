package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dbohry/huginn/internal/db"
)

var assets = fstest.MapFS{
	"index.html":      {Data: []byte("<!doctype html><title>Huginn</title>")},
	"assets/app-1.js": {Data: []byte("console.log(1)")},
	"favicon.svg":     {Data: []byte("<svg/>")},
}

func newDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func get(h http.Handler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestHealthz(t *testing.T) {
	h := New(newDB(t), assets, "1.2.3")

	rec := get(h, http.MethodGet, "/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" || body["version"] != "1.2.3" {
		t.Errorf("body = %v", body)
	}
}

func TestHealthzFailsWhenDatabaseIsClosed(t *testing.T) {
	conn := newDB(t)
	h := New(conn, assets, "dev")
	conn.Close()

	if rec := get(h, http.MethodGet, "/healthz"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

func TestStatic(t *testing.T) {
	h := New(newDB(t), assets, "dev")

	cases := []struct {
		name, method, target string
		code                 int
		contains             string
		cache                string
	}{
		{"root", "GET", "/", 200, "<title>Huginn</title>", "no-cache"},
		{"spa route falls back to index", "GET", "/settings/appearance", 200, "<title>Huginn</title>", "no-cache"},
		{"hashed asset", "GET", "/assets/app-1.js", 200, "console.log(1)", "public, max-age=31536000, immutable"},
		{"plain file", "GET", "/favicon.svg", 200, "<svg/>", ""},
		{"missing file with extension", "GET", "/missing.js", 404, "", ""},
		{"unknown api route is json 404", "GET", "/api/nope", 404, `"error"`, "no-store"},
		{"head on index", "HEAD", "/", 200, "", "no-cache"},
		{"post to static", "POST", "/", 405, "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := get(h, c.method, c.target)
			if rec.Code != c.code {
				t.Fatalf("status = %d, want %d", rec.Code, c.code)
			}
			if !strings.Contains(rec.Body.String(), c.contains) {
				t.Errorf("body %q does not contain %q", rec.Body.String(), c.contains)
			}
			if got := rec.Header().Get("Cache-Control"); got != c.cache {
				t.Errorf("Cache-Control = %q, want %q", got, c.cache)
			}
		})
	}
}

func TestSecurityHeaders(t *testing.T) {
	rec := get(New(newDB(t), assets, "dev"), http.MethodGet, "/")
	for _, h := range []string{"X-Content-Type-Options", "X-Frame-Options", "Content-Security-Policy", "Referrer-Policy"} {
		if rec.Header().Get(h) == "" {
			t.Errorf("missing header %s", h)
		}
	}
}

func TestMissingFrontendBuild(t *testing.T) {
	rec := get(New(newDB(t), fstest.MapFS{}, "dev"), http.MethodGet, "/")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}
