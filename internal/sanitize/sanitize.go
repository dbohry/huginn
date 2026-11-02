// Package sanitize makes feed content safe to show.
//
// Feeds are written by strangers. Everything from a feed that reaches the page
// goes through HTML or Text first.
package sanitize

import (
	"html"
	"strings"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"
)

var (
	article = newArticlePolicy()
	strict  = bluemonday.StrictPolicy()
)

func newArticlePolicy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	// Links leave the app in a new tab, without telling the target who sent
	// them or giving it a handle on this window.
	p.RequireNoFollowOnLinks(true)
	p.RequireNoReferrerOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	// Only web addresses: no javascript:, data: or file: links or images.
	p.AllowURLSchemes("http", "https", "mailto")
	return p
}

// HTML returns an allow-listed copy of feed HTML: ordinary text formatting,
// lists, tables, links and images, and nothing that can run code, load a
// frame, restyle the page or submit a form.
func HTML(s string) string {
	return article.Sanitize(s)
}

// Text returns the plain text of feed HTML: no tags, entities decoded and
// whitespace collapsed to single spaces.
func Text(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(strict.Sanitize(s))), " ")
}

// Truncate cuts s to at most max characters, adding an ellipsis when it cuts.
// It breaks at a space when one is close to the limit.
func Truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	cut := string(runes[:max])
	if i := strings.LastIndex(cut, " "); i > max*2/3 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,.;:-") + "…"
}
