// Package feed fetches and parses RSS, Atom and JSON Feed documents.
package feed

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

// Parsed is a feed in the shape Huginn stores.
type Parsed struct {
	Title   string
	SiteURL string
	Items   []Item
}

// Item is one article of a feed.
type Item struct {
	// GUID identifies the item within its feed. It is the feed's own id when it
	// has one, else the link, else a hash of the title and date.
	GUID string
	// Title, URL and Author are plain text.
	Title  string
	URL    string
	Author string
	// Published is nil when the feed gave no usable date.
	Published *time.Time
	// Summary and ContentHTML are raw HTML from the feed and must be sanitized
	// before they are shown.
	Summary     string
	ContentHTML string
}

// ErrNotAFeed means a document could not be read as RSS, Atom or JSON Feed.
var ErrNotAFeed = errors.New("not a feed")

// Parse reads an RSS, Atom or JSON Feed document.
func Parse(r io.Reader) (*Parsed, error) {
	f, err := gofeed.NewParser().Parse(r)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotAFeed, err)
	}

	out := &Parsed{
		Title:   strings.TrimSpace(f.Title),
		SiteURL: strings.TrimSpace(f.Link),
	}
	seen := map[string]bool{}
	for _, it := range f.Items {
		if it == nil {
			continue
		}
		item := convert(it)
		if seen[item.GUID] {
			continue // some feeds repeat an entry; keep the first
		}
		seen[item.GUID] = true
		out.Items = append(out.Items, item)
	}
	return out, nil
}

func convert(it *gofeed.Item) Item {
	item := Item{
		Title:       strings.TrimSpace(it.Title),
		URL:         strings.TrimSpace(it.Link),
		Summary:     it.Description,
		ContentHTML: it.Content,
	}
	if it.Author != nil {
		item.Author = strings.TrimSpace(it.Author.Name)
	}
	if item.Author == "" && len(it.Authors) > 0 && it.Authors[0] != nil {
		item.Author = strings.TrimSpace(it.Authors[0].Name)
	}
	switch {
	case it.PublishedParsed != nil:
		t := it.PublishedParsed.UTC()
		item.Published = &t
	case it.UpdatedParsed != nil:
		t := it.UpdatedParsed.UTC()
		item.Published = &t
	}
	item.GUID = guidFor(it, item)
	return item
}

func guidFor(it *gofeed.Item, item Item) string {
	if g := strings.TrimSpace(it.GUID); g != "" {
		return g
	}
	if item.URL != "" {
		return item.URL
	}
	h := sha1.New()
	io.WriteString(h, item.Title)
	if item.Published != nil {
		io.WriteString(h, item.Published.Format(time.RFC3339))
	}
	return "hash:" + hex.EncodeToString(h.Sum(nil))
}
