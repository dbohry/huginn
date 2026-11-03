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
	// Category is the folder the feed sits in, or "" if it has none. Folders
	// nested deeper than one level count as their outermost folder.
	Category string
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
	Type     string    `xml:"type,attr,omitempty"`
	XMLURL   string    `xml:"xmlUrl,attr,omitempty"`
	HTMLURL  string    `xml:"htmlUrl,attr,omitempty"`
	Children []outline `xml:"outline"`
}

// Parse returns every feed in an OPML document. A folder becomes the Category
// of the feeds inside it. An outline without an xmlUrl is a folder or a note.
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
	var walk func([]outline, string)
	walk = func(outlines []outline, category string) {
		for _, o := range outlines {
			if u := strings.TrimSpace(o.XMLURL); u != "" {
				title := strings.TrimSpace(o.Title)
				if title == "" {
					title = strings.TrimSpace(o.Text)
				}
				feeds = append(feeds, Feed{Title: title, URL: u, Category: category})
			}
			inner := category
			if inner == "" && strings.TrimSpace(o.XMLURL) == "" {
				inner = strings.TrimSpace(o.Text)
				if inner == "" {
					inner = strings.TrimSpace(o.Title)
				}
			}
			walk(o.Children, inner)
		}
	}
	walk(doc.Body.Outlines, "")
	return feeds, nil
}

// Write writes feeds as an OPML 2.0 document.
func Write(w io.Writer, title string, feeds []Feed) error {
	doc := document{Version: "2.0", Head: head{Title: title}}
	folders := map[string]int{} // category -> index in doc.Body.Outlines
	for _, f := range feeds {
		name := f.Title
		if name == "" {
			name = f.URL
		}
		o := outline{Text: name, Title: name, Type: "rss", XMLURL: f.URL}
		if f.Category == "" {
			doc.Body.Outlines = append(doc.Body.Outlines, o)
			continue
		}
		i, ok := folders[f.Category]
		if !ok {
			i = len(doc.Body.Outlines)
			folders[f.Category] = i
			doc.Body.Outlines = append(doc.Body.Outlines, outline{Text: f.Category, Title: f.Category})
		}
		doc.Body.Outlines[i].Children = append(doc.Body.Outlines[i].Children, o)
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
