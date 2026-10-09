package msgerror

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/mrusme/neonmodem/internal/config"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
)

func TestTheDialogShowsPrintableText(t *testing.T) {
	cfg := config.Defaults("/cache")
	c := ctx.New(nil, &cfg, slog.New(slog.DiscardHandler), nil)
	m := NewModel(&c)

	m.Update(msgs.ShowError{Messages: []string{"bad \x1b[2J news\nsecond \x1b]8;;https://evil.example\aline"}})
	content := m.viewport.GetContent()
	if strings.Contains(content, "\x1b[2J") || strings.Contains(content, "\x1b]8") {
		t.Errorf("an escape sequence reached the dialog: %q", content)
	}
	plain := ansi.Strip(content)
	if !strings.Contains(plain, "bad") || !strings.Contains(plain, "news") || !strings.Contains(plain, "second line") ||
		strings.Contains(plain, "evil.example") {
		t.Errorf("the text is %q", plain)
	}

	m.Update(msgs.ShowNotices{Entries: []msgs.NoticeEntry{{At: time.Now(), Text: "notice \x1b[2J"}}})
	if strings.Contains(m.viewport.GetContent(), "\x1b[2J") {
		t.Error("an escape sequence reached the notices")
	}
}
