package sanitize

import (
	"strings"
	"testing"
)

func TestHTMLRemovesAnythingThatCanRunCode(t *testing.T) {
	hostile := map[string]string{
		"script tag":          `<p>hi</p><script>alert(1)</script>`,
		"inline handler":      `<img src="https://example.com/a.png" onerror="alert(1)">`,
		"javascript link":     `<a href="javascript:alert(1)">click</a>`,
		"javascript mixed":    `<a href="JaVaScRiPt:alert(1)">click</a>`,
		"data link":           `<a href="data:text/html,<script>alert(1)</script>">x</a>`,
		"data image":          `<img src="data:image/svg+xml;base64,PHN2Zz4=">`,
		"iframe":              `<iframe src="https://evil.example/"></iframe>`,
		"object":              `<object data="https://evil.example/x.swf"></object>`,
		"embed":               `<embed src="https://evil.example/x.swf">`,
		"style tag":           `<style>body{display:none}</style><p>x</p>`,
		"style attribute":     `<p style="position:fixed;inset:0">x</p>`,
		"form":                `<form action="https://evil.example/"><input name="p"><button>Send</button></form>`,
		"svg":                 `<svg onload="alert(1)"><circle/></svg>`,
		"meta refresh":        `<meta http-equiv="refresh" content="0;url=https://evil.example/">`,
		"base":                `<base href="https://evil.example/">`,
		"vbscript":            `<a href="vbscript:msgbox(1)">x</a>`,
		"entity-hidden js":    `<a href="&#106;avascript:alert(1)">x</a>`,
		"nested script trick": `<scr<script>ipt>alert(1)</scr</script>ipt>`,
	}
	banned := []string{"<script", "javascript:", "vbscript:", "data:", "onerror", "onload", "<iframe", "<object",
		"<embed", "<style", "style=", "<form", "<input", "<svg", "<meta", "<base"}

	for name, in := range hostile {
		t.Run(name, func(t *testing.T) {
			out := strings.ToLower(HTML(in))
			for _, b := range banned {
				if strings.Contains(out, b) {
					t.Errorf("output still contains %q:\n in:  %s\n out: %s", b, in, out)
				}
			}
		})
	}
}

func TestHTMLDropsTheBodyOfScriptAndStyleElements(t *testing.T) {
	for _, in := range []string{`<script>alert(1)</script>`, `<style>body{display:none}</style>`} {
		if out := HTML("<p>ok</p>" + in); out != "<p>ok</p>" {
			t.Errorf("HTML(%q) = %q, want only the paragraph", in, out)
		}
	}
}

func TestHTMLShiftsHeadingsBelowTheTitle(t *testing.T) {
	cases := map[string]string{
		// The highest heading becomes h3 and the rest move with it, down to h6.
		`<h1>a</h1><h2>b</h2><h3>c</h3><h4>d</h4><h5>e</h5><h6>f</h6>`: `<h3>a</h3><h4>b</h4><h5>c</h5><h6>d</h6><h6>e</h6><h6>f</h6>`,
		`<h2>a</h2><h3>b</h3><h2>c</h2>`:                               `<h3>a</h3><h4>b</h4><h3>c</h3>`,
		`<h3>a</h3><h5>b</h5>`:                                         `<h3>a</h3><h5>b</h5>`,
		`<h5>a</h5><h6>b</h6>`:                                         `<h3>a</h3><h4>b</h4>`,
		`<p>no headings</p>`:                                           `<p>no headings</p>`,
		`<p>text &lt;h1&gt; stays as text</p>`:                         `<p>text &lt;h1&gt; stays as text</p>`,
	}
	for in, want := range cases {
		if got := HTML(in); got != want {
			t.Errorf("HTML(%s)\n got  %s\n want %s", in, got, want)
		}
	}
}

func TestHTMLKeepsOrdinaryContent(t *testing.T) {
	in := `<h3>Title</h3><p>Some <strong>bold</strong>, <em>italic</em> and <code>code</code>.</p>` +
		`<ul><li>one</li></ul><blockquote>quote</blockquote><pre>x</pre>` +
		`<img src="https://example.com/pic.jpg" alt="A picture">` +
		`<table><tr><td>cell</td></tr></table>`
	out := HTML(in)
	for _, want := range []string{"<h3>", "<strong>bold</strong>", "<em>italic</em>", "<code>code</code>",
		"<li>one</li>", "<blockquote>", "<pre>", `src="https://example.com/pic.jpg"`, `alt="A picture"`, "<td>cell</td>"} {
		if !strings.Contains(out, want) {
			t.Errorf("lost %q:\n%s", want, out)
		}
	}
}

func TestHTMLLinksAreSafeToFollow(t *testing.T) {
	out := HTML(`<a href="https://example.com/post">read</a>`)
	for _, want := range []string{`href="https://example.com/post"`, `target="_blank"`, "noopener", "noreferrer", "nofollow"} {
		if !strings.Contains(out, want) {
			t.Errorf("link is missing %q: %s", want, out)
		}
	}
	if !strings.Contains(HTML(`<a href="mailto:a@example.com">mail</a>`), "mailto:") {
		t.Error("mailto links should survive")
	}
}

func TestText(t *testing.T) {
	cases := map[string]string{
		`<p>Hello   <b>world</b></p>`:           "Hello world",
		"Fish &amp; chips":                      "Fish & chips",
		"<p>one</p>\n\n<p>two</p>":              "one two",
		`<script>alert(1)</script>Safe`:         "Safe",
		"":                                      "",
		`<img src="x" onerror="alert(1)">text`:  "text",
		"&lt;script&gt;alert(1)&lt;/script&gt;": "<script>alert(1)</script>",
	}
	for in, want := range cases {
		if got := Text(in); got != want {
			t.Errorf("Text(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := Truncate("short", 10); got != "short" {
		t.Errorf("short text changed: %q", got)
	}
	if got := Truncate("the quick brown fox jumps over", 20); got != "the quick brown fox…" {
		t.Errorf("word break: %q", got)
	}
	if got := Truncate("ααααααααααα", 5); got != "ααααα…" {
		t.Errorf("multi-byte: %q", got)
	}
}
