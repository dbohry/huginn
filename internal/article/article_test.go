package article

import (
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
)

func extract(t *testing.T, file string) (Article, error) {
	t.Helper()
	f, err := os.Open("testdata/" + file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	u, _ := url.Parse("https://news.example.com/business/ai-bonds")
	return Extract(f, u)
}

func TestExtractFindsTheArticleAndLeavesThePageAround(t *testing.T) {
	a, err := extract(t, "news.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(a.Title, "Bond markets brace for AI borrowing") {
		t.Errorf("title = %q", a.Title)
	}
	for _, want := range []string{
		"competing with governments for the same investors",
		"cost of borrowing across the euro area",
		"who gets the money first",
		"Investors weigh a crowded market.",
	} {
		if !strings.Contains(a.ContentHTML, want) {
			t.Errorf("the article is missing %q:\n%s", want, a.ContentHTML)
		}
	}
	for _, unwanted := range []string{"Subscribe to our newsletter", "No comments yet", "All rights reserved", "<script", "injected"} {
		if strings.Contains(a.ContentHTML, unwanted) {
			t.Errorf("the article still contains %q", unwanted)
		}
	}
	if a.TextLength < 400 {
		t.Errorf("TextLength = %d, expected a few hundred characters", a.TextLength)
	}
}

func TestExtractMakesLinksAndPicturesAbsolute(t *testing.T) {
	a, err := extract(t, "news.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`src="https://news.example.com/img/bonds.jpg"`, `href="https://news.example.com/business/ecb-explained"`} {
		if !strings.Contains(a.ContentHTML, want) {
			t.Errorf("expected %s in:\n%s", want, a.ContentHTML)
		}
	}
}

func TestExtractRefusesPagesWithoutAnArticle(t *testing.T) {
	if _, err := extract(t, "home.html"); !errors.Is(err, ErrNoArticle) {
		t.Fatalf("a home page: err = %v, want ErrNoArticle", err)
	}
	u, _ := url.Parse("https://example.com/")
	for name, page := range map[string]string{
		"empty":       "",
		"not html":    "\x00\x01\x02 binary",
		"script only": "<html><body><div id=app></div><script>render()</script></body></html>",
	} {
		if _, err := Extract(strings.NewReader(page), u); !errors.Is(err, ErrNoArticle) {
			t.Errorf("%s: err = %v, want ErrNoArticle", name, err)
		}
	}
}

func TestExtractDropsAHeadingThatRepeatsTheTitle(t *testing.T) {
	u, _ := url.Parse("https://news.example.com/story/9")
	load := func(titles ...string) Article {
		t.Helper()
		f, err := os.Open("testdata/repeated-title.html")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		a, err := Extract(f, u, titles...)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	const headline = "Summary story 9"
	const laterHeading = "A later heading that belongs to the article"

	// Without a title the article keeps its own headline: this is what the fix is for.
	if got := load(); !strings.Contains(got.ContentHTML, headline) {
		t.Fatalf("the test page should keep its headline when no title is given:\n%s", got.ContentHTML)
	}
	// An unrelated title removes nothing.
	if got := load("Something else entirely"); !strings.Contains(got.ContentHTML, headline) {
		t.Errorf("an unrelated title removed the headline:\n%s", got.ContentHTML)
	}
	// The title the article is already shown under removes the repeat, however it is spaced or cased.
	got := load("  summary STORY   9 ")
	if strings.Contains(got.ContentHTML, headline) {
		t.Errorf("the repeated headline is still there:\n%s", got.ContentHTML)
	}
	// ...but only that one: the text and a later heading stay.
	for _, want := range []string{"The full text of story nine begins here", laterHeading, "finishes the full text of story nine"} {
		if !strings.Contains(got.ContentHTML, want) {
			t.Errorf("dropping the title also dropped %q:\n%s", want, got.ContentHTML)
		}
	}
}
