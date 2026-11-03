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

func TestParseUsesFoldersAsCategories(t *testing.T) {
	got, err := Parse(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	want := []Feed{
		{Title: "Example Blog", URL: "https://example.com/feed.xml", Category: "Tech"},
		{Title: "Other & More", URL: "https://other.example/atom", Category: "Tech"},
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

func TestWriteGroupsCategoriesIntoFolders(t *testing.T) {
	feeds := []Feed{
		{Title: "One", URL: "https://one.example/rss", Category: "Tech"},
		{Title: "Loose", URL: "https://loose.example/rss"},
		{Title: "Two", URL: "https://two.example/rss", Category: "Tech"},
		{Title: "Three", URL: "https://three.example/rss", Category: "Food"},
	}
	var buf bytes.Buffer
	if err := Write(&buf, "t", feeds); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(buf.String(), `text="Tech"`); n != 1 {
		t.Errorf("Tech folder appears %d times, want 1:\n%s", n, buf.String())
	}
	got, err := Parse(&buf)
	if err != nil {
		t.Fatal(err)
	}
	want := []Feed{feeds[0], feeds[2], feeds[1], feeds[3]} // folders keep their feeds together
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}
