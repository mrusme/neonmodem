package postshow

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderedPostFitsTheViewportWidth(t *testing.T) {
	c := testCtx(t)
	p := testPost(3)
	p.Subject = strings.Repeat("A remarkably long subject ", 8) + "ends here"
	p.Author.Name = strings.Repeat("name", 12)
	p.Replies[2].Author.Name = strings.Repeat("replier", 9)
	p.Body = "https://example.com/" + strings.Repeat("x", 120)

	const width = 60
	rendered, ok := renderPost(context.Background(), c, p, width)
	if !ok {
		t.Fatal("renderPost was superseded")
	}

	for i, line := range strings.Split(rendered.content, "\n") {
		if w := ansi.StringWidth(line); w > width {
			t.Errorf("line %d is %d wide: %q", i, w, ansi.Strip(line))
		}
	}

	plain := strings.Join(strings.Fields(ansi.Strip(rendered.content)), "")
	for _, want := range []string{"endshere", strings.Repeat("x", 120), "#1", "#2", "#3"} {
		if !strings.Contains(plain, want) {
			t.Errorf("the rendered post lost %q", want)
		}
	}
}

func TestLoadingPlaceholderFitsTheViewportWidth(t *testing.T) {
	c := testCtx(t)
	p := testPost(0)
	p.Subject = strings.Repeat("long subject ", 12)

	for i, line := range strings.Split(renderLoadingPlaceholder(c, p, 40), "\n") {
		if w := ansi.StringWidth(line); w > 40 {
			t.Errorf("line %d is %d wide", i, w)
		}
	}
}
