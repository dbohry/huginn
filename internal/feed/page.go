package feed

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"

	"golang.org/x/net/html/charset"
)

// MaxPageBytes is the largest web page Huginn will read when loading a full article.
const MaxPageBytes = 3 << 20

// ErrNotHTML is returned when an address gives something other than a web page, such as a PDF.
var ErrNotHTML = errors.New("not a web page")

// FetchPage downloads a web page and returns it as UTF-8 HTML, together with the
// address it ended up at after any redirects (which relative links in it belong to).
// It follows the same rules as feeds: the size limit, the timeout, and refusing
// private networks when that is switched on.
func (f *Fetcher) FetchPage(ctx context.Context, rawURL string) (io.Reader, *url.URL, error) {
	if err := ValidateURL(rawURL); err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", f.UserAgent)
	req.Header.Set("Accept", "text/html, application/xhtml+xml;q=0.9, */*;q=0.1")

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, &StatusError{Status: resp.Status}
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "" {
		media, _, err := mime.ParseMediaType(contentType)
		if err != nil || (media != "text/html" && media != "application/xhtml+xml") {
			return nil, nil, ErrNotHTML
		}
	}

	limit := int64(MaxPageBytes)
	if f.MaxBytes > 0 && f.MaxBytes < limit {
		limit = f.MaxBytes
	}
	raw, err := readLimited(resp.Body, limit)
	if err != nil {
		return nil, nil, err
	}
	// Pages are in whatever encoding their site chose. Everything after this expects UTF-8.
	utf8, err := charset.NewReader(raw, contentType)
	if err != nil {
		return nil, nil, err
	}
	body, err := io.ReadAll(utf8)
	if err != nil {
		return nil, nil, err
	}
	return bytes.NewReader(body), resp.Request.URL, nil
}
