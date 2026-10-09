package posts

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestFailedFeedsShowPrintableErrors(t *testing.T) {
	lemmy, tags := newSystems()
	m, _ := testModel(t, lemmy, tags)
	m.pending = 0
	m.feeds = map[int]systemFeed{0: {err: errors.New("bad \x1b[2J news")}}

	out := m.placeholder()
	if strings.Contains(out, "\x1b[2J") || !strings.Contains(ansi.Strip(out), "bad  news") {
		t.Errorf("the placeholder is %q", out)
	}
}
