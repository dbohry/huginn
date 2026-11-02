// Package server wires up Huginn's HTTP handlers.
package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// New returns the HTTP handler for the whole application. assets holds the
// built frontend (index.html at its root). version is reported by /healthz.
func New(conn *sql.DB, assets fs.FS, version string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", health(conn, version))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	})
	mux.Handle("/", static(assets))
	return secureHeaders(mux)
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
			"default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
