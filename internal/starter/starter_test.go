package starter

import (
	"net/url"
	"testing"
)

func TestDefaultsAreValid(t *testing.T) {
	feeds := Defaults()
	if len(feeds) < 20 {
		t.Fatalf("only %d feeds", len(feeds))
	}
	seen := map[string]bool{}
	topics := map[string]bool{}
	for _, f := range feeds {
		if f.Category == "" || f.Title == "" {
			t.Errorf("incomplete entry: %+v", f)
		}
		u, err := url.Parse(f.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			t.Errorf("%s: bad address %q", f.Title, f.URL)
		}
		if seen[f.URL] {
			t.Errorf("%s is listed twice", f.URL)
		}
		seen[f.URL] = true
		topics[f.Category] = true
	}
	if len(topics) < 5 {
		t.Errorf("only %d topics", len(topics))
	}
}
