package hackernews

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
)

func TestPageMessageCutsBetweenCharacters(t *testing.T) {
	body := strings.Repeat("ü", messageLength+50)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<html><body>" + body + "</body></html>"))
	if err != nil {
		t.Fatal(err)
	}

	msg := pageMessage(doc)
	if !utf8.ValidString(msg) {
		t.Fatal("the message holds a split character")
	}
	if !strings.HasSuffix(msg, " ...") || utf8.RuneCountInString(msg) != messageLength+4 {
		t.Errorf("got %d characters: %q", utf8.RuneCountInString(msg), msg)
	}

	short, err := goquery.NewDocumentFromReader(strings.NewReader("<html><body>fine</body></html>"))
	if err != nil {
		t.Fatal(err)
	}
	if got := pageMessage(short); got != "fine" {
		t.Errorf("got %q", got)
	}
}
