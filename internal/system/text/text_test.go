package text

import (
	"strings"
	"testing"
	"time"
)

func TestMarkdown(t *testing.T) {
	got := Markdown("<p>Hello <strong>there</strong> and <a href=\"https://x.y\">link</a></p>")
	if !strings.Contains(got, "**there**") || !strings.Contains(got, "[link](https://x.y)") {
		t.Errorf("unexpected markdown: %q", got)
	}
	if Markdown("   ") != "" {
		t.Error("blank html should give an empty string")
	}
}

func TestMarkdownKeepsEmojiAsText(t *testing.T) {
	got := Markdown(`<p>Hi <img src="/images/emoji/twitter/wave.png?v=15" title=":wave:" class="emoji" alt=":wave:" loading="lazy" width="20" height="20"> there</p>`)
	if !strings.Contains(got, ":wave:") || strings.Contains(got, "![") {
		t.Errorf("emoji image should become its alt text: %q", got)
	}
	got = Markdown(`<p><img src="https://example.com/photo.png" alt="a photo"></p>`)
	if !strings.Contains(got, "![a photo](https://example.com/photo.png)") {
		t.Errorf("ordinary images must stay images: %q", got)
	}
}

func TestTime(t *testing.T) {
	want := time.Date(2026, 10, 4, 12, 23, 41, 0, time.FixedZone("", -5*3600))
	got := Time("2026-10-04T12:23:41.000-05:00")
	if !got.Equal(want) {
		t.Errorf("parsed %v, want %v", got, want)
	}
	if !Time("").IsZero() || !Time("not a date").IsZero() {
		t.Error("unparsable input must give the zero time")
	}
}

func TestTruncate(t *testing.T) {
	if got := Truncate("héllo wörld", 5); got != "héllo" {
		t.Errorf("got %q", got)
	}
	if got := Truncate("short", 10); got != "short" {
		t.Errorf("got %q", got)
	}
	if got := Truncate("anything", 0); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := FirstNonEmpty("", "", "x", "y"); got != "x" {
		t.Errorf("got %q", got)
	}
	if got := FirstNonEmpty(); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestFirstParagraph(t *testing.T) {
	cases := []struct {
		name     string
		markdown string
		want     string
	}{
		{"plain", "Welcome to Programmer Humor!\n\nThis is a place for jokes.", "Welcome to Programmer Humor!"},
		{"heading first", "## Rules\n\nBe nice.", "Be nice."},
		{"links and emphasis", "Welcome to the [Python community](https://x.y/c/python) on **programming.dev**, _really_!", "Welcome to the Python community on programming.dev, really!"},
		{"code span", "Run `go test \\*` here", "Run go test \\* here"},
		{"escapes", `Stars \*not\* emphasis`, "Stars *not* emphasis"},
		{"entities", "Tom &amp; Jerry &#169; &copy;", "Tom & Jerry © ©"},
		{"autolink", "See <https://example.com> now", "See https://example.com now"},
		{"image only first", "![logo](x.png)\n\nThe real text", "The real text"},
		{"image inside", "A ![logo](x.png) logo", "A logo"},
		{"raw html", "Real <b>bold</b> text", "Real bold text"},
		{"spoiler", ":::spoiler Credits\n- icon by someone\n:::\n\nA community for art.", "A community for art."},
		{"soft breaks", "One line\nand the next\\\nand a hard break", "One line and the next and a hard break"},
		{"list and quote", "- one\n- two\n\n> quoted\n\n```\ncode\n```\n\n---\n\nAfter all that", "After all that"},
		{"empty", "", ""},
		{"heading only", "# Rules", ""},
	}
	for _, tc := range cases {
		if got := FirstParagraph(tc.markdown); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
