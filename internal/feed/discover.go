package feed

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// maxPageBytes is the most of a web page read while looking for its feed.
const maxPageBytes = 1 << 20

// feedTypes are the media types a page uses to advertise its feeds.
var feedTypes = map[string]bool{
	"application/rss+xml":   true,
	"application/atom+xml":  true,
	"application/feed+json": true,
	"text/xml":              true,
	"application/xml":       true,
}

// Discover looks for the feed a web page advertises with
// <link rel="alternate" type="application/rss+xml" href="...">. It returns the
// address of the first one, made absolute, or "" if the page has none.
func (f *Fetcher) Discover(ctx context.Context, pageURL string) (string, error) {
	if err := ValidateURL(pageURL); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", f.UserAgent)
	req.Header.Set("Accept", "text/html, application/xhtml+xml;q=0.9, */*;q=0.1")

	resp, err := f.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", &StatusError{Status: resp.Status}
	}

	base := resp.Request.URL // after redirects: relative links are relative to where the page ended up
	return findFeedLink(io.LimitReader(resp.Body, maxPageBytes), base)
}

// findFeedLink returns the first advertised feed in an HTML document.
func findFeedLink(r io.Reader, base *url.URL) (string, error) {
	z := html.NewTokenizer(r)
	for {
		switch z.Next() {
		case html.ErrorToken:
			if err := z.Err(); err != io.EOF {
				return "", fmt.Errorf("read page: %w", err)
			}
			return "", nil
		case html.StartTagToken, html.SelfClosingTagToken:
			tok := z.Token()
			if tok.Data == "body" {
				return "", nil // feed links live in <head>
			}
			if tok.Data != "link" {
				continue
			}
			var rel, typ, href string
			for _, a := range tok.Attr {
				switch strings.ToLower(a.Key) {
				case "rel":
					rel = strings.ToLower(a.Val)
				case "type":
					typ = strings.ToLower(strings.TrimSpace(a.Val))
				case "href":
					href = strings.TrimSpace(a.Val)
				}
			}
			if href == "" || !hasToken(rel, "alternate") || !feedTypes[typ] {
				continue
			}
			ref, err := url.Parse(href)
			if err != nil {
				continue
			}
			abs := base.ResolveReference(ref)
			if abs.Scheme != "http" && abs.Scheme != "https" {
				continue
			}
			return abs.String(), nil
		}
	}
}

func hasToken(list, want string) bool {
	for _, t := range strings.Fields(list) {
		if t == want {
			return true
		}
	}
	return false
}
