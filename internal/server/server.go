// Package server wires up Huginn's HTTP handlers.
package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/dbohry/huginn/internal/starter"
)

// New returns the HTTP handler for the whole application. assets holds the
// built frontend (index.html at its root). version is reported by /healthz.
func New(conn *sql.DB, assets fs.FS, version string) http.Handler {
	return newServer(conn, version).routes(assets)
}

// Options are settings that change how the server behaves.
type Options struct {
	// BlockPrivate refuses to fetch feeds from private networks.
	BlockPrivate bool
}

// Start is New for the real program: it also starts the background refresher,
// which runs until ctx ends. Tests use New and drive the refresher by hand.
func Start(ctx context.Context, conn *sql.DB, assets fs.FS, version string, opts Options) http.Handler {
	s := newServer(conn, version)
	if opts.BlockPrivate {
		s.refresh.Fetcher.BlockPrivateNetworks()
	}
	if err := s.store.SeedStarterFeeds(ctx, starterRows(starter.Defaults())); err != nil {
		slog.Error("could not fill the starter feeds", "error", err)
	}
	go s.refresh.Run(ctx)
	return s.routes(assets)
}

func (s *server) routes(assets fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", health(s.db, s.version))

	mux.HandleFunc("GET /api/setup/status", s.setupStatus)
	mux.HandleFunc("POST /api/setup", s.setup)
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("POST /api/logout", s.logout)
	mux.Handle("GET /api/me", s.authed(true, s.me))
	mux.Handle("POST /api/me/password", s.authed(true, s.changePassword))

	mux.Handle("GET /api/users", s.admin(s.listUsers))
	mux.Handle("POST /api/users", s.admin(s.createUser))
	mux.Handle("POST /api/users/{id}/reset-password", s.admin(s.resetUserPassword))
	mux.Handle("DELETE /api/users/{id}", s.admin(s.deleteUser))

	mux.Handle("GET /api/feeds", s.authed(false, s.listFeeds))
	mux.Handle("POST /api/feeds", s.authed(false, s.addFeed))
	mux.Handle("PATCH /api/feeds/{id}", s.authed(false, s.updateFeed))
	mux.Handle("DELETE /api/feeds/{id}", s.authed(false, s.deleteFeed))
	mux.Handle("POST /api/feeds/{id}/refresh", s.authed(false, s.refreshFeed))
	mux.Handle("POST /api/refresh", s.authed(false, s.refreshAll))

	mux.Handle("GET /api/items", s.authed(false, s.listItems))
	mux.Handle("GET /api/items/{id}", s.authed(false, s.getItem))
	mux.Handle("POST /api/items/{id}/full", s.authed(false, s.loadFullText))
	mux.Handle("PUT /api/items/{id}/read", s.authed(false, s.setRead))
	mux.Handle("POST /api/read-all", s.authed(false, s.markAllRead))

	mux.Handle("GET /api/starter", s.authed(false, s.listStarter))
	mux.Handle("POST /api/starter/subscribe", s.authed(false, s.subscribeStarter))
	mux.Handle("POST /api/starter/dismiss", s.authed(false, s.dismissStarter))
	mux.Handle("POST /api/starter/feeds", s.admin(s.addStarterFeed))
	mux.Handle("DELETE /api/starter/feeds/{id}", s.admin(s.deleteStarterFeed))
	mux.Handle("POST /api/starter/reset", s.admin(s.resetStarter))
	mux.Handle("GET /api/starter/opml", s.admin(s.exportStarterOPML))
	mux.Handle("POST /api/starter/opml", s.admin(s.importStarterOPML))

	mux.Handle("GET /api/opml", s.authed(false, s.exportOPML))
	mux.Handle("POST /api/opml", s.authed(false, s.importOPML))

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	})
	mux.Handle("/", static(assets))
	return secureHeaders(sameOrigin(mux))
}

func health(conn *sql.DB, version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := conn.PingContext(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": version})
	}
}

// static serves the single-page app. Files that exist are served as-is
// (hashed files under /assets/ are cached for a year). Any other path without
// a file extension gets index.html so client-side routes survive a reload.
func static(assets fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		name := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if name != "." && isFile(assets, name) {
			if strings.HasPrefix(name, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			http.ServeFileFS(w, r, assets, name)
			return
		}
		if name != "." && path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}
		serveIndex(w, r, assets)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, assets fs.FS) {
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		http.Error(w, "the frontend has not been built", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	if r.Method == http.MethodHead {
		return
	}
	w.Write(index)
}

func isFile(assets fs.FS, name string) bool {
	info, err := fs.Stat(assets, name)
	return err == nil && !info.IsDir()
}

func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data: https:; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
