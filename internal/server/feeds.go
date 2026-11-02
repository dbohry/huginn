package server

import (
	"net/http"

	"github.com/dbohry/huginn/internal/store"
)

// feedJSON is a feed as the interface sees it.
type feedJSON struct {
	ID      int64  `json:"id"`
	URL     string `json:"url"`
	Title   string `json:"title"`
	SiteURL string `json:"siteUrl"`
	Unread  int    `json:"unread"`
	// LastFetchedAt is Unix seconds, or null if the feed was never fetched.
	LastFetchedAt *int64 `json:"lastFetchedAt"`
	// Error is the last fetch error, or empty when the last fetch worked.
	Error string `json:"error"`
}

func toFeedJSON(f store.FeedSummary) feedJSON {
	out := feedJSON{ID: f.ID, URL: f.URL, Title: f.Title, SiteURL: f.SiteURL, Unread: f.Unread, Error: f.LastError}
	if f.LastFetchedAt != nil {
		t := f.LastFetchedAt.Unix()
		out.LastFetchedAt = &t
	}
	return out
}

func (s *server) listFeeds(w http.ResponseWriter, r *http.Request, user store.User, _ store.Session) {
	feeds, err := s.store.ListFeeds(r.Context(), user.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	out := make([]feedJSON, len(feeds))
	for i, f := range feeds {
		out[i] = toFeedJSON(f)
	}
	writeJSON(w, http.StatusOK, map[string][]feedJSON{"feeds": out})
}
