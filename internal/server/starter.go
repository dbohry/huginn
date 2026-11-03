package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/dbohry/huginn/internal/feed"
	"github.com/dbohry/huginn/internal/opml"
	"github.com/dbohry/huginn/internal/starter"
	"github.com/dbohry/huginn/internal/store"
)

const (
	maxCategoryRunes = 40
	maxTitleRunes    = 120
	maxStarterFeeds  = 500
)

// cleanCategory trims a topic name and says what is wrong with it, if anything.
func cleanCategory(c string) (string, string) {
	c = strings.Join(strings.Fields(c), " ")
	if utf8.RuneCountInString(c) > maxCategoryRunes {
		return "", "A topic name can have at most 40 characters."
	}
	return c, ""
}

func starterRows(in []starter.Feed) []store.StarterFeed {
	out := make([]store.StarterFeed, len(in))
	for i, f := range in {
		out[i] = store.StarterFeed{Category: f.Category, Title: f.Title, URL: f.URL}
	}
	return out
}

type starterFeedJSON struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Followed bool   `json:"followed"` // by the signed-in user
}

type starterTopicJSON struct {
	Name  string            `json:"name"`
	Feeds []starterFeedJSON `json:"feeds"`
}

// listStarter returns the suggested feeds grouped by topic.
func (s *server) listStarter(w http.ResponseWriter, r *http.Request, user store.User, _ store.Session) {
	feeds, err := s.store.ListStarterFeeds(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	followed, err := s.store.FollowedURLs(r.Context(), user.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	topics := []starterTopicJSON{}
	index := map[string]int{}
	for _, f := range feeds {
		name := f.Category
		if name == "" {
			name = "Other"
		}
		i, ok := index[name]
		if !ok {
			i = len(topics)
			index[name] = i
			topics = append(topics, starterTopicJSON{Name: name, Feeds: []starterFeedJSON{}})
		}
		topics[i].Feeds = append(topics[i].Feeds, starterFeedJSON{ID: f.ID, Title: f.Title, URL: f.URL, Followed: followed[f.URL]})
	}
	writeJSON(w, http.StatusOK, map[string]any{"topics": topics})
}

// subscribeStarter follows the chosen suggestions, filed under their topics.
// Like an OPML import it does not fetch them while the browser waits.
func (s *server) subscribeStarter(w http.ResponseWriter, r *http.Request, user store.User, _ store.Session) {
	var in struct {
		IDs []int64 `json:"ids"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.IDs) > maxStarterFeeds {
		writeError(w, http.StatusBadRequest, "That is too many feeds at once.")
		return
	}
	all, err := s.store.ListStarterFeeds(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	byID := make(map[int64]store.StarterFeed, len(all))
	for _, f := range all {
		byID[f.ID] = f
	}

	var added, existing int
	for _, id := range in.IDs {
		f, ok := byID[id]
		if !ok {
			continue // removed by an administrator since the list was shown
		}
		switch _, err := s.store.CreateFeedInCategory(r.Context(), user.ID, f.URL, f.Title, f.Category); {
		case errors.Is(err, store.ErrExists):
			existing++
		case err != nil:
			serverError(w, err)
			return
		default:
			added++
		}
	}
	if err := s.store.SetStarterDone(r.Context(), user.ID); err != nil {
		serverError(w, err)
		return
	}
	if added > 0 {
		s.background(func(ctx context.Context) { s.refresh.RefreshDue(ctx) })
	}
	writeJSON(w, http.StatusOK, map[string]int{"added": added, "existing": existing})
}

// dismissStarter records that the user has seen the offer and wants none of it.
func (s *server) dismissStarter(w http.ResponseWriter, r *http.Request, user store.User, _ store.Session) {
	if err := s.store.SetStarterDone(r.Context(), user.ID); err != nil {
		serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- administrators edit the list ---

func (s *server) addStarterFeed(w http.ResponseWriter, r *http.Request, _ store.User, _ store.Session) {
	var in struct {
		Category string `json:"category"`
		Title    string `json:"title"`
		URL      string `json:"url"`
	}
	if !decode(w, r, &in) {
		return
	}
	category, msg := cleanCategory(in.Category)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	title := strings.TrimSpace(in.Title)
	if utf8.RuneCountInString(title) > maxTitleRunes {
		writeError(w, http.StatusBadRequest, "A title can have at most 120 characters.")
		return
	}
	address := strings.TrimSpace(in.URL)
	if address != "" && !strings.Contains(address, "://") {
		address = "https://" + address
	}
	if address == "" || feed.ValidateURL(address) != nil {
		writeError(w, http.StatusBadRequest, "Enter the web address of a feed.")
		return
	}
	if title == "" {
		title = feedName("", address)
	}
	if category == "" {
		writeError(w, http.StatusBadRequest, "Choose a topic for the feed.")
		return
	}
	f, err := s.store.AddStarterFeed(r.Context(), category, title, address)
	switch {
	case errors.Is(err, store.ErrExists):
		writeError(w, http.StatusConflict, "That feed is already in the list.")
	case err != nil:
		serverError(w, err)
	default:
		writeJSON(w, http.StatusCreated, map[string]starterFeedJSON{"feed": {ID: f.ID, Title: f.Title, URL: f.URL}})
	}
}

func (s *server) deleteStarterFeed(w http.ResponseWriter, r *http.Request, _ store.User, _ store.Session) {
	id, ok := pathID(w, r, "Feed not found.")
	if !ok {
		return
	}
	switch err := s.store.DeleteStarterFeed(r.Context(), id); {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "Feed not found.")
	case err != nil:
		serverError(w, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// resetStarter puts the built-in list back.
func (s *server) resetStarter(w http.ResponseWriter, r *http.Request, _ store.User, _ store.Session) {
	if err := s.store.ReplaceStarterFeeds(r.Context(), starterRows(starter.Defaults())); err != nil {
		serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) exportStarterOPML(w http.ResponseWriter, r *http.Request, _ store.User, _ store.Session) {
	feeds, err := s.store.ListStarterFeeds(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	list := make([]opml.Feed, len(feeds))
	for i, f := range feeds {
		list[i] = opml.Feed{Title: f.Title, URL: f.URL, Category: f.Category}
	}
	w.Header().Set("Content-Type", "text/x-opml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="huginn-starter-feeds.opml"`)
	w.Header().Set("Cache-Control", "no-store")
	if err := opml.Write(w, "Huginn starter feeds", list); err != nil {
		serverError(w, err)
	}
}

// importStarterOPML replaces the list with the feeds in an OPML file. Folders
// become topics; a feed outside any folder goes under "Other".
func (s *server) importStarterOPML(w http.ResponseWriter, r *http.Request, _ store.User, _ store.Session) {
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
	var rows []store.StarterFeed
	skipped := 0
	for _, f := range feeds {
		address := strings.TrimSpace(f.URL)
		category, msg := cleanCategory(f.Category)
		if feed.ValidateURL(address) != nil || msg != "" {
			skipped++
			continue
		}
		if category == "" {
			category = "Other"
		}
		title := strings.TrimSpace(f.Title)
		if title == "" {
			title = feedName("", address)
		}
		rows = append(rows, store.StarterFeed{Category: category, Title: title, URL: address})
	}
	if len(rows) == 0 {
		writeError(w, http.StatusBadRequest, "That file has no feeds in it.")
		return
	}
	if len(rows) > maxStarterFeeds {
		writeError(w, http.StatusBadRequest, "That file has more than 500 feeds.")
		return
	}
	if err := s.store.ReplaceStarterFeeds(r.Context(), rows); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"feeds": len(rows), "skipped": skipped})
}
