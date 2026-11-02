package server

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/dbohry/huginn/internal/feed"
	"github.com/dbohry/huginn/internal/opml"
	"github.com/dbohry/huginn/internal/store"
)

const (
	maxOPMLBytes = 2 << 20
	maxOPMLFeeds = 1000
)

// exportOPML sends the user's subscriptions as an OPML file.
func (s *server) exportOPML(w http.ResponseWriter, r *http.Request, user store.User, _ store.Session) {
	feeds, err := s.store.ListFeeds(r.Context(), user.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	list := make([]opml.Feed, len(feeds))
	for i, f := range feeds {
		list[i] = opml.Feed{Title: f.Title, URL: f.URL}
	}
	w.Header().Set("Content-Type", "text/x-opml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="huginn-subscriptions.opml"`)
	w.Header().Set("Cache-Control", "no-store")
	if err := opml.Write(w, "Huginn subscriptions", list); err != nil {
		serverError(w, err)
	}
}

// importOPML adds the feeds in an uploaded OPML file. It does not fetch them
// one by one while the browser waits: they are fetched in the background.
func (s *server) importOPML(w http.ResponseWriter, r *http.Request, user store.User, _ store.Session) {
	r.Body = http.MaxBytesReader(w, r.Body, maxOPMLBytes)
	feeds, err := opml.Parse(r.Body)
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "That file is too large.")
			return
		}
		writeError(w, http.StatusBadRequest, "That file is not a valid OPML subscription list.")
		return
	}
	if len(feeds) > maxOPMLFeeds {
		writeError(w, http.StatusBadRequest, "That file has more than 1000 feeds.")
		return
	}

	var added, existing, invalid int
	for _, f := range feeds {
		address := strings.TrimSpace(f.URL)
		if feed.ValidateURL(address) != nil {
			invalid++
			continue
		}
		switch _, err := s.store.CreateFeedWithTitle(r.Context(), user.ID, address, f.Title); {
		case errors.Is(err, store.ErrExists):
			existing++
		case err != nil:
			serverError(w, err)
			return
		default:
			added++
		}
	}
	if added > 0 {
		s.background(func(ctx context.Context) { s.refresh.RefreshDue(ctx) })
	}
	writeJSON(w, http.StatusOK, map[string]int{"added": added, "existing": existing, "invalid": invalid})
}
