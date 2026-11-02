package feed

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	// DefaultMaxBytes is the largest feed document Huginn will read.
	DefaultMaxBytes = 5 << 20
	// DefaultTimeout bounds one whole fetch, including reading the body.
	DefaultTimeout = 20 * time.Second
)

// ErrTooLarge is returned when a feed document is bigger than the limit.
var ErrTooLarge = errors.New("feed is too large")

// Fetcher downloads feeds politely: it sends conditional requests, limits the
// size it reads and identifies itself.
type Fetcher struct {
	Client    *http.Client
	MaxBytes  int64
	UserAgent string
}

// NewFetcher returns a Fetcher with sensible limits. version is shown in the
// User-Agent.
func NewFetcher(version string) *Fetcher {
	return &Fetcher{
		Client:    &http.Client{Timeout: DefaultTimeout},
		MaxBytes:  DefaultMaxBytes,
		UserAgent: "Huginn/" + version + " (+https://github.com/dbohry/huginn)",
	}
}

// Result is the outcome of one fetch.
type Result struct {
	// NotModified is true when the server answered 304: nothing changed, and
	// Feed is nil.
	NotModified bool
	// Feed is the parsed document when it was downloaded.
	Feed *Parsed
	// ETag and LastModified are the validators to send next time. They are the
	// previous values when the server did not send new ones.
	ETag         string
	LastModified string
}

// Fetch downloads and parses the feed at rawURL. etag and lastModified are the
// validators from the previous fetch, or empty.
func (f *Fetcher) Fetch(ctx context.Context, rawURL, etag, lastModified string) (*Result, error) {
	if err := ValidateURL(rawURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", f.UserAgent)
	req.Header.Set("Accept", "application/atom+xml, application/rss+xml, application/feed+json, application/xml;q=0.9, text/xml;q=0.9, */*;q=0.5")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if lastModified != "" {
		req.Header.Set("If-Modified-Since", lastModified)
	}

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotModified:
		return &Result{NotModified: true, ETag: etag, LastModified: lastModified}, nil
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("server answered %s", resp.Status)
	}

	body, err := readLimited(resp.Body, f.MaxBytes)
	if err != nil {
		return nil, err
	}
	parsed, err := Parse(body)
	if err != nil {
		return nil, err
	}

	res := &Result{Feed: parsed, ETag: resp.Header.Get("ETag"), LastModified: resp.Header.Get("Last-Modified")}
	return res, nil
}

// ValidateURL accepts only absolute http and https addresses.
func ValidateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("the address must start with http:// or https://")
	}
	return nil
}

func readLimited(r io.Reader, max int64) (io.Reader, error) {
	data, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, ErrTooLarge
	}
	return bytes.NewReader(data), nil
}
