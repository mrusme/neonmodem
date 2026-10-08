package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
	"github.com/mrusme/neonmodem/internal/ui/windows/msgerror"
)

func notice(m Model, text string) Model {
	updated, _ := m.Update(msgs.Notice{Text: text, IsError: true})
	return updated.(Model)
}

func TestTheLineCountsTheNoticesBehindTheLatest(t *testing.T) {
	m, _ := rootModel(t)
	m = notice(m, "one")
	m = notice(m, "two")
	m = notice(m, "three")

	line := ansi.Strip(m.noticeLine())
	if !strings.Contains(line, "three (+2)") {
		t.Fatalf("the line shows %q", line)
	}
	if len(m.notices) != 3 || m.notices[0].Text != "one" {
		t.Errorf("the history is %+v", m.notices)
	}
}

func TestBangOpensTheNoticesNewestFirstAndResetsTheCount(t *testing.T) {
	m, _ := rootModel(t)
	m = notice(m, "one")
	m = notice(m, "two")
	m = notice(m, "three")

	updated, cmd := m.Update(tea.KeyPressMsg{Code: '!', Text: "!"})
	m, _ = settleModel(t, updated.(Model), cmd)
	if !m.wm.IsOpen(msgerror.WIN_ID) {
		t.Fatal("! should open the notices dialog")
	}
	if m.unseen != 0 {
		t.Errorf("opening the dialog should reset the count, got %d", m.unseen)
	}

	view := ansi.Strip(m.View().Content)
	three, two, one := strings.Index(view, "three"), strings.Index(view, "two"), strings.Index(view, "one")
	if three < 0 || two < 0 || one < 0 || three > two || two > one {
		t.Errorf("the dialog should list the notices newest first:\n%s", view)
	}
	if !strings.Contains(view, "Notices") {
		t.Errorf("the dialog should be titled Notices:\n%s", view)
	}

	updated, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m, _ = settleModel(t, updated.(Model), cmd)
	m = notice(m, "four")
	if line := ansi.Strip(m.noticeLine()); strings.Contains(line, "(+") {
		t.Errorf("a single notice after the dialog shows no count: %q", line)
	}
}

func TestTheHistoryKeepsTheLastFifty(t *testing.T) {
	m, _ := rootModel(t)
	for i := range 60 {
		m = notice(m, strings.Repeat("x", i+1))
	}
	if len(m.notices) != maxNotices {
		t.Fatalf("the history holds %d notices, expected %d", len(m.notices), maxNotices)
	}
	if len(m.notices[0].Text) != 11 {
		t.Errorf("the oldest kept notice should be the eleventh, got %d characters", len(m.notices[0].Text))
	}
}
