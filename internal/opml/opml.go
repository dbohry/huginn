// Package opml reads and writes OPML subscription lists.
package opml

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Feed is one subscription in an OPML file.
type Feed struct {
	Title string
	URL   string
}

type document struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    head     `xml:"head"`
	Body    body     `xml:"body"`
}

type head struct {
	Title string `xml:"title"`
}

type body struct {
	Outlines []outline `xml:"outline"`
}

type outline struct {
	Text     string    `xml:"text,attr"`
	Title    string    `xml:"title,attr"`
	Type     string    `xml:"type,attr"`
	XMLURL   string    `xml:"xmlUrl,attr"`
	HTMLURL  string    `xml:"htmlUrl,attr"`
	Children []outline `xml:"outline"`
}

// Parse returns every feed in an OPML document. Folders are flattened: Huginn
// has none yet. An outline without an xmlUrl is a folder or a note and is
// skipped.
func Parse(r io.Reader) ([]Feed, error) {
	var doc document
	dec := xml.NewDecoder(r)
	dec.Strict = false
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("read OPML: %w", err)
	}
	if doc.XMLName.Local != "opml" {
		return nil, errors.New("not an OPML file")
	}

	var feeds []Feed
	var walk func([]outline)
	walk = func(outlines []outline) {
		for _, o := range outlines {
			if u := strings.TrimSpace(o.XMLURL); u != "" {
				title := strings.TrimSpace(o.Title)
				if title == "" {
					title = strings.TrimSpace(o.Text)
				}
				feeds = append(feeds, Feed{Title: title, URL: u})
			}
			walk(o.Children)
		}
	}
	walk(doc.Body.Outlines)
	return feeds, nil
}

// Write writes feeds as an OPML 2.0 document.
func Write(w io.Writer, title string, feeds []Feed) error {
	doc := document{Version: "2.0", Head: head{Title: title}}
	for _, f := range feeds {
		name := f.Title
		if name == "" {
			name = f.URL
		}
		doc.Body.Outlines = append(doc.Body.Outlines, outline{Text: name, Title: name, Type: "rss", XMLURL: f.URL})
	}
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}
