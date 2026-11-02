package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/dbohry/huginn/internal/db"
)

func newTestHandler(t *testing.T, assets fstest.MapFS) http.Handler {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := db.Migrate(conn); err != nil {
		t.Fatal(err)
	}
	return New(conn, assets, "test")
}

func get(h http.Handler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestHealthz(t *testing.T) {
	rec := get(newTestHandler(t, fstest.MapFS{}), http.MethodGet, "/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" || body["version"] != "test" {
		t.Fatalf("body = %v", body)
	}
}

func TestUnknownAPIPathIsJSON404(t *testing.T) {
	rec := get(newTestHandler(t, fstest.MapFS{}), http.MethodGet, "/api/nope")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content type = %q", ct)
	}
}

func TestStaticServing(t *testing.T) {
	assets := fstest.MapFS{
		"index.html":    {Data: []byte("<html>app</html>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	h := newTestHandler(t, assets)

	if rec := get(h, http.MethodGet, "/assets/app.js"); rec.Code != http.StatusOK ||
		rec.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("asset: status %d, cache %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	if rec := get(h, http.MethodGet, "/some/route"); rec.Code != http.StatusOK || rec.Body.String() != "<html>app</html>" {
		t.Fatalf("spa route: status %d, body %q", rec.Code, rec.Body.String())
	}
	if rec := get(h, http.MethodGet, "/missing.js"); rec.Code != http.StatusNotFound {
		t.Fatalf("missing file: status %d", rec.Code)
	}
	if rec := get(h, http.MethodPost, "/"); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /: status %d", rec.Code)
	}
}

func TestFrontendNotBuilt(t *testing.T) {
	rec := get(newTestHandler(t, fstest.MapFS{}), http.MethodGet, "/")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	rec := get(newTestHandler(t, fstest.MapFS{}), http.MethodGet, "/healthz")
	for _, h := range []string{"X-Content-Type-Options", "X-Frame-Options", "Content-Security-Policy"} {
		if rec.Header().Get(h) == "" {
			t.Errorf("missing header %s", h)
		}
	}
}
