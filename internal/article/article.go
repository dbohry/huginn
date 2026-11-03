// Package article finds the readable article inside a web page, the way a
// browser's "reader view" does.
package article

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/go-shiori/go-readability"
	"golang.org/x/net/html"
)

// ErrNoArticle is returned when a page has no article text Huginn can find: a
// page that needs a script to show its text, a paywall, a home page, a video.
var ErrNoArticle = errors.New("no article text found")

const (
	// minTextLength is how much text a page needs before it counts as an article.
	minTextLength = 200
	// maxElements bounds the work done on one page.
	maxElements = 40000
)

// Article is the readable part of a page.
type Article struct {
	Title string
	// ContentHTML is the article as HTML, with relative links made absolute. It is
	// still a stranger's HTML and must be sanitized before it is shown.
	ContentHTML string
	TextLength  int // characters of text
}

// Extract finds the main article in an HTML page. pageURL is the address the page
// came from, so that relative links and pictures keep working. titles are the
// names the article is already shown under (the feed's title for it, for example):
// if the text starts with a heading that repeats one, that heading is dropped.
func Extract(page io.Reader, pageURL *url.URL, titles ...string) (Article, error) {
	parser := readability.NewParser()
	parser.MaxElemsToParse = maxElements

	found, err := parser.Parse(page, pageURL)
	if err != nil {
		return Article{}, fmt.Errorf("%w: %v", ErrNoArticle, err)
	}
	text := strings.TrimSpace(found.TextContent)
	length := utf8.RuneCountInString(text)
	if length < minTextLength || strings.TrimSpace(found.Content) == "" {
		return Article{}, ErrNoArticle
	}

	content := found.Content
	if found.Node != nil && dropRepeatedTitle(found.Node, append(titles, found.Title)) {
		var b strings.Builder
		if err := html.Render(&b, found.Node); err == nil {
			content = b.String()
		}
	}
	return Article{Title: strings.TrimSpace(found.Title), ContentHTML: content, TextLength: length}, nil
}

// dropRepeatedTitle removes the first heading in the article when it only repeats
// one of titles, and reports whether it did. Only the first heading is looked at:
// a later one is part of the article.
func dropRepeatedTitle(root *html.Node, titles []string) bool {
	want := map[string]bool{}
	for _, t := range titles {
		if t = normalize(t); t != "" {
			want[t] = true
		}
	}
	var heading *html.Node
	var find func(n *html.Node) bool
	find = func(n *html.Node) bool {
		if n.Type == html.ElementNode && len(n.Data) == 2 && n.Data[0] == 'h' && n.Data[1] >= '1' && n.Data[1] <= '6' {
			heading = n
			return true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if find(c) {
				return true
			}
		}
		return false
	}
	if !find(root) || !want[normalize(textOf(heading))] || heading.Parent == nil {
		return false
	}
	heading.Parent.RemoveChild(heading)
	return true
}

func textOf(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(textOf(c))
	}
	return b.String()
}

// normalize makes two spellings of a title comparable: no case, one space between words.
func normalize(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}
