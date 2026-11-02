package opml

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

const sample = `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <head><title>Subscriptions</title></head>
  <body>
    <outline text="Tech" title="Tech">
      <outline type="rss" text="Example Blog" xmlUrl="https://example.com/feed.xml" htmlUrl="https://example.com/"/>
      <outline type="rss" title="Other &amp; More" xmlUrl="https://other.example/atom"/>
    </outline>
    <outline type="rss" text="Top level" xmlUrl="https://top.example/rss"/>
    <outline text="A note without a feed"/>
  </body>
</opml>`

func TestParseFlattensFolders(t *testing.T) {
	got, err := Parse(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	want := []Feed{
		{Title: "Example Blog", URL: "https://example.com/feed.xml"},
		{Title: "Other & More", URL: "https://other.example/atom"},
		{Title: "Top level", URL: "https://top.example/rss"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestParseRejectsOtherDocuments(t *testing.T) {
	for name, in := range map[string]string{
		"not xml":   "hello",
		"other xml": `<rss version="2.0"><channel/></rss>`,
		"empty":     "",
	} {
		if _, err := Parse(strings.NewReader(in)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestWriteThenParseRoundTrips(t *testing.T) {
	feeds := []Feed{
		{Title: "A & B <feed>", URL: "https://a.example/feed?x=1&y=2"},
		{Title: "", URL: "https://untitled.example/rss"},
	}
	var buf bytes.Buffer
	if err := Write(&buf, "Huginn subscriptions", feeds); err != nil {
		t.Fatal(err)
	}
	got, err := Parse(&buf)
	if err != nil {
		t.Fatal(err)
	}
	want := []Feed{feeds[0], {Title: "https://untitled.example/rss", URL: "https://untitled.example/rss"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}
