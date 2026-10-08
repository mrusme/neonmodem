package header

import (
	"log/slog"
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	"github.com/mrusme/neonmodem/internal/config"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
)

type otherMsg struct{}

func testCtx() *ctx.Ctx {
	cfg := config.Defaults("/cache")
	c := ctx.New(nil, &cfg, slog.New(slog.DiscardHandler), nil)
	return &c
}

// TestSpinnerAnimatesWhileLoading pins down the tick behaviour the spinner
// depends on: exactly one chain is started when loading begins, each tick
// schedules the next one, ordinary messages do not inject extra ticks, and the
// chain retires once loading is done.
func TestSpinnerAnimatesWhileLoading(t *testing.T) {
	c := testCtx()
	m := NewModel(c)

	// Idle: nothing to schedule.
	m, cmd := m.Update(otherMsg{})
	if cmd != nil {
		t.Fatal("a tick was scheduled while not loading")
	}

	// Loading begins: exactly one tick chain starts.
	c.StartLoading(ctx.LoadFeed)
	m, cmd = m.Update(otherMsg{})
	if cmd == nil {
		t.Fatal("no tick was scheduled when loading began")
	}

	tick, ok := cmd().(spinner.TickMsg)
	if !ok {
		t.Fatalf("expected a spinner.TickMsg, got %T", cmd())
	}

	// The tick advances the frame and schedules the next one, which is what
	// makes the spinner actually animate.
	before := m.spinner.View()
	m, cmd = m.Update(tick)
	if cmd == nil {
		t.Fatal("the tick chain died: no follow-up tick was scheduled")
	}
	if after := m.spinner.View(); after == before {
		t.Errorf("spinner frame did not advance (%q -> %q)", before, after)
	}

	// Other messages must not pile on additional ticks while loading.
	if _, extra := m.Update(otherMsg{}); extra != nil {
		t.Error("an ordinary message scheduled an extra tick while loading")
	}

	// Loading ends: the chain is not fed any further.
	c.StopLoading(ctx.LoadFeed)
	next, _ := cmd().(spinner.TickMsg)
	if _, after := m.Update(next); after != nil {
		t.Error("the tick chain kept running after loading finished")
	}
}

// TestSpinnerRestartsOnNextLoad makes sure a second load animates too, rather
// than the chain having retired for good.
func TestSpinnerRestartsOnNextLoad(t *testing.T) {
	c := testCtx()
	m := NewModel(c)

	c.StartLoading(ctx.LoadFeed)
	m, first := m.Update(otherMsg{})
	if first == nil {
		t.Fatal("first load did not start the spinner")
	}

	c.StopLoading(ctx.LoadFeed)
	m, _ = m.Update(otherMsg{})

	c.StartLoading(ctx.LoadFeed)
	_, second := m.Update(otherMsg{})
	if second == nil {
		t.Fatal("second load did not restart the spinner")
	}
}

func TestViewShowsProgress(t *testing.T) {
	c := testCtx()
	c.Config.RenderBanner = false
	m := NewModel(c)

	c.StartLoading(ctx.LoadFeed)
	c.Progress = "2/3"
	m, _ = m.Update(otherMsg{})

	view := m.View()
	if view == "" {
		t.Fatal("empty header view")
	}
	if !strings.Contains(view, "2/3") {
		t.Errorf("progress not shown in header: %q", view)
	}
}
