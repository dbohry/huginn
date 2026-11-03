package server

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dbohry/huginn/internal/article"
	"github.com/dbohry/huginn/internal/sanitize"
	"github.com/dbohry/huginn/internal/store"
)

const (
	defaultPageSize = 50
	maxPageSize     = 100
	snippetLength   = 220

	// summaryOnlyLimit is the longest summary, in characters, that still looks like a
	// teaser rather than the article, for a feed that gives nothing else.
	summaryOnlyLimit = 600
	// loadArticleTimeout bounds fetching and reading one page. It is shorter than the
	// server's write timeout.
	loadArticleTimeout = 25 * time.Second
)

// itemJSON is an article in a list.
type itemJSON struct {
	ID        int64  `json:"id"`
	FeedID    int64  `json:"feedId"`
	FeedTitle string `json:"feedTitle"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Author    string `json:"author"`
	// PublishedAt is Unix seconds.
	PublishedAt int64 `json:"publishedAt"`
	// Snippet is plain text, cut short.
	Snippet string `json:"snippet"`
	Unread  bool   `json:"unread"`
}

// itemDetailJSON is one article opened for reading.
type itemDetailJSON struct {
	itemJSON
	// Content is sanitized HTML, safe to insert into the page.
	Content string `json:"content"`
	// FullText is true when Content is the full article loaded from its own page, not
	// what the feed gave.
	FullText bool `json:"fullText"`
	// SummaryOnly is true when the feed gave nothing but a short summary, so the full
	// article is only on the site.
	SummaryOnly bool `json:"summaryOnly"`
}

// detailJSON prepares an article for reading: the full article if one has been
// loaded, else the feed's content, else its summary.
func detailJSON(it store.Item) itemDetailJSON {
	body := it.FullHTML
	if strings.TrimSpace(body) == "" {
		body = it.ContentHTML
	}
	if strings.TrimSpace(body) == "" {
		body = it.Summary
	}
	haveFull := strings.TrimSpace(it.FullHTML) != ""
	feedGaveContent := strings.TrimSpace(it.ContentHTML) != ""
	return itemDetailJSON{
		itemJSON:    toItemJSON(it),
		Content:     sanitize.HTML(body),
		FullText:    haveFull,
		SummaryOnly: !haveFull && !feedGaveContent && len([]rune(sanitize.Text(it.Summary))) < summaryOnlyLimit,
	}
}

func toItemJSON(it store.Item) itemJSON {
	title := strings.TrimSpace(it.Title)
	if title == "" {
		title = "(untitled)"
	}
	return itemJSON{
		ID: it.ID, FeedID: it.FeedID, FeedTitle: feedName(it.FeedTitle, it.FeedURL),
		Title: title, URL: safeLink(it.URL), Author: it.Author,
		PublishedAt: it.PublishedAt.Unix(),
		Snippet:     sanitize.Truncate(sanitize.Text(it.Summary), snippetLength),
		Unread:      !it.IsRead,
	}
}

// safeLink returns link when it is a web address, else "". The interface puts
// it in an href, where javascript: would run code.
func safeLink(link string) string {
	l := strings.ToLower(strings.TrimSpace(link))
	if strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") {
		return strings.TrimSpace(link)
	}
	return ""
}

func (s *server) listItems(w http.ResponseWriter, r *http.Request, user store.User, _ store.Session) {
	q := r.URL.Query()
	f := store.ItemFilter{UserID: user.ID, UnreadOnly: q.Get("unread") == "1", Limit: defaultPageSize}

	if v := q.Get("feed"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "Unknown feed.")
			return
		}
		f.FeedID = id
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxPageSize {
			writeError(w, http.StatusBadRequest, "The page size must be between 1 and 100.")
			return
		}
		f.Limit = n
	}
	if v := q.Get("after"); v != "" {
		c, ok := decodeCursor(v)
		if !ok {
			writeError(w, http.StatusBadRequest, "That page marker is not valid.")
			return
		}
		f.After = c
	}

	items, next, err := s.store.ListItems(r.Context(), f)
	if err != nil {
		serverError(w, err)
		return
	}
	out := make([]itemJSON, len(items))
	for i, it := range items {
		out[i] = toItemJSON(it)
	}
	resp := map[string]any{"items": out, "next": ""}
	if next != nil {
		resp["next"] = encodeCursor(next)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *server) getItem(w http.ResponseWriter, r *http.Request, user store.User, _ store.Session) {
	id, ok := pathID(w, r, "Article not found.")
	if !ok {
		return
	}
	it, err := s.store.GetItem(r.Context(), user.ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Article not found.")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]itemDetailJSON{"item": detailJSON(it)})
}

// loadFullText fetches the article's own page, finds the article in it and keeps
// it, so the page is fetched only once. Asking again returns what was kept.
func (s *server) loadFullText(w http.ResponseWriter, r *http.Request, user store.User, _ store.Session) {
	id, ok := pathID(w, r, "Article not found.")
	if !ok {
		return
	}
	it, err := s.store.GetItem(r.Context(), user.ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Article not found.")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}

	if strings.TrimSpace(it.FullHTML) == "" {
		link := safeLink(it.URL)
		if link == "" {
			writeError(w, http.StatusUnprocessableEntity, "This article has no web address to load it from.")
			return
		}
		if !s.allowExtract(w, user.ID) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), loadArticleTimeout)
		defer cancel()
		html, err := s.fetchArticle(ctx, link, it.Title)
		if err != nil {
			s.articleFailed(w, err)
			return
		}
		if err := s.store.SaveFullText(r.Context(), user.ID, id, html); err != nil {
			serverError(w, err)
			return
		}
		it.FullHTML = html
	}
	writeJSON(w, http.StatusOK, map[string]itemDetailJSON{"item": detailJSON(it)})
}

// fetchArticle downloads a page and returns the article HTML found in it.
func (s *server) fetchArticle(ctx context.Context, link, title string) (string, error) {
	page, finalURL, err := s.refresh.Fetcher.FetchPage(ctx, link)
	if err != nil {
		return "", err
	}
	found, err := article.Extract(page, finalURL, title)
	if err != nil {
		return "", err
	}
	return found.ContentHTML, nil
}

func (s *server) articleFailed(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, article.ErrNoArticle):
		writeError(w, http.StatusUnprocessableEntity, "Could not load the full article. Huginn could not find the article text on that page.")
	case isFetchProblem(err):
		writeError(w, http.StatusUnprocessableEntity, "Could not load the full article. "+fetchMessage(err))
	default:
		serverError(w, err)
	}
}

func (s *server) allowExtract(w http.ResponseWriter, userID int64) bool {
	key := strconv.FormatInt(userID, 10)
	if ok, retry := s.extractByUser.Allow(key); !ok {
		tooManyAttempts(w, retry)
		return false
	}
	s.extractByUser.Fail(key)
	return true
}

func (s *server) setRead(w http.ResponseWriter, r *http.Request, user store.User, _ store.Session) {
	id, ok := pathID(w, r, "Article not found.")
	if !ok {
		return
	}
	var in struct {
		Read bool `json:"read"`
	}
	if !decode(w, r, &in) {
		return
	}
	switch err := s.store.SetRead(r.Context(), user.ID, id, in.Read); {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "Article not found.")
	case err != nil:
		serverError(w, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// markAllRead marks every unread article read, or only a feed's when a feedId
// is given.
func (s *server) markAllRead(w http.ResponseWriter, r *http.Request, user store.User, _ store.Session) {
	var in struct {
		FeedID int64 `json:"feedId"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.FeedID != 0 {
		if _, err := s.store.GetUserFeed(r.Context(), user.ID, in.FeedID); errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Feed not found.")
			return
		} else if err != nil {
			serverError(w, err)
			return
		}
	}
	n, err := s.store.MarkAllRead(r.Context(), user.ID, in.FeedID)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"marked": n})
}

// A cursor is the publication time and id of the last article on a page,
// encoded so the interface can pass it back without understanding it.
func encodeCursor(c *store.Cursor) string {
	return base64.RawURLEncoding.EncodeToString([]byte(c.Published + "|" + strconv.FormatInt(c.ID, 10)))
}

func decodeCursor(s string) (*store.Cursor, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, false
	}
	published, idText, ok := strings.Cut(string(raw), "|")
	if !ok {
		return nil, false
	}
	if _, err := time.Parse(time.RFC3339, published); err != nil {
		return nil, false
	}
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil || id <= 0 {
		return nil, false
	}
	return &store.Cursor{Published: published, ID: id}, true
}
