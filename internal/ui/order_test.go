package ui

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/mrusme/neonmodem/internal/config"
	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/prompt"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
	"github.com/mrusme/neonmodem/internal/ui/windows/popuplist"
)

type orderedSystem struct {
	title    string
	ordering system.Ordering
}

func (s *orderedSystem) Kind() string                      { return "fake" }
func (s *orderedSystem) URL() string                       { return "" }
func (s *orderedSystem) Title() string                     { return s.title }
func (s *orderedSystem) Description() string               { return s.title }
func (s *orderedSystem) Capabilities() system.Capabilities { return system.CapRead }
func (s *orderedSystem) Connect(context.Context, prompt.Prompter, string) (system.Settings, error) {
	return system.Settings{}, nil
}
func (s *orderedSystem) ListForums(context.Context) ([]forum.Forum, error) { return nil, nil }
func (s *orderedSystem) Orders(string) system.Ordering                     { return s.ordering }
func (s *orderedSystem) ListPosts(context.Context, string, system.Order) ([]post.Post, error) {
	return nil, nil
}
func (s *orderedSystem) LoadPost(context.Context, *post.Post) error      { return nil }
func (s *orderedSystem) CreatePost(context.Context, *post.Post) error    { return nil }
func (s *orderedSystem) CreateReply(context.Context, *reply.Reply) error { return nil }

func orderModel(t *testing.T) (Model, *ctx.Ctx) {
	t.Helper()

	systems := []system.System{
		&orderedSystem{"lemmy.example", system.Ordering{Default: system.OrderNew, Supported: system.AllOrders()}},
		&orderedSystem{"lobste.rs", system.Ordering{Default: system.OrderNew,
			Supported: []system.Order{system.OrderNew, system.OrderActive, system.OrderHot}}},
		&orderedSystem{"hup.example", system.Only(system.OrderActive)},
	}
	cfg := &config.Config{}
	c := ctx.New(nil, cfg, slog.New(slog.DiscardHandler), systems)
	m := NewModel(&c)
	m.setSizes(120, 48)
	m.currentView = 1
	return m, &c
}

func TestOrderPickerListsOrdersWithNotes(t *testing.T) {
	m, c := orderModel(t)
	c.SetOrder(system.OrderTopWeek)

	items, selected := m.orderItems(200)
	if len(items) != len(system.AllOrders()) {
		t.Fatalf("lemmy.example supports every order, got %d items", len(items))
	}
	if items[selected].(OrderItem).Order != system.OrderTopWeek {
		t.Errorf("the cursor should start on the current order, got %d", selected)
	}

	notes := map[system.Order]string{}
	for _, item := range items {
		o := item.(OrderItem)
		notes[o.Order] = o.Note
	}
	want := map[system.Order]string{
		system.OrderNew:      "hup.example sorts its active posts by date",
		system.OrderActive:   "",
		system.OrderHot:      "hup.example keeps Active",
		system.OrderTopWeek:  "lobste.rs keeps New, hup.example keeps Active",
		system.OrderComments: "lobste.rs and hup.example sort their active posts by replies",
	}
	for order, note := range want {
		if notes[order] != note {
			t.Errorf("%s: note %q, want %q", order, notes[order], note)
		}
	}

	hot := OrderItem{Order: system.OrderHot, Note: notes[system.OrderHot]}
	if hot.Description() != "Each site's own trending order\nhup.example keeps Active" {
		t.Errorf("description %q", hot.Description())
	}
}

func TestOrderPickerForOneSystemOnlyShowsItsOrders(t *testing.T) {
	m, c := orderModel(t)
	c.SetCurrentSystem(1)
	c.SetOrder(system.OrderTopWeek)

	items, selected := m.orderItems(200)
	var labels []string
	for _, item := range items {
		o := item.(OrderItem)
		labels = append(labels, o.Order.Label())
		if o.Note != "" {
			t.Errorf("one system needs no notes: %q", o.Note)
		}
	}
	if strings.Join(labels, ",") != "New,Active,Hot" || selected != 0 {
		t.Errorf("got %v with the cursor on %d", labels, selected)
	}
}

func TestOrderPickerWrapsLongNotes(t *testing.T) {
	m, _ := orderModel(t)

	items, _ := m.orderItems(30)
	for _, item := range items {
		o := item.(OrderItem)
		if o.Order != system.OrderComments {
			continue
		}
		lines := strings.Split(o.Note, "\n")
		if len(lines) < 2 {
			t.Errorf("a long note should wrap: %q", o.Note)
		}
		for _, line := range lines {
			if len(line) > 30 {
				t.Errorf("line %q is longer than 30 columns", line)
			}
		}
		if strings.Join(strings.Fields(o.Note), " ") != "lobste.rs and hup.example sort their active posts by replies" {
			t.Errorf("wrapping changed the text: %q", o.Note)
		}
	}
}

func TestCtrlOOpensTheOrderPickerAndAPickApplies(t *testing.T) {
	m, c := orderModel(t)

	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})
	m = updated.(Model)
	if cmd == nil || !m.wm.IsOpen(popuplist.WIN_ID) {
		t.Fatal("ctrl+o should open the order picker")
	}

	updated, cmd = m.Update(msgs.Picked{Kind: msgs.PickOrder, Item: OrderItem{Order: system.OrderHot}})
	m = updated.(Model)
	if c.GetOrder() != system.OrderHot {
		t.Errorf("the picked order wasn't applied: %q", c.GetOrder())
	}
	if m.wm.IsOpen(popuplist.WIN_ID) {
		t.Error("the picker should close")
	}

	found := false
	for _, msg := range collectMsgs(cmd) {
		if _, ok := msg.(msgs.OrderChanged); ok {
			found = true
		}
		if _, ok := msg.(msgs.RefreshFeed); ok {
			t.Error("an order change must not trigger a full refresh")
		}
	}
	if !found {
		t.Error("picking an order should send OrderChanged")
	}
}

func TestStatusLineYieldsToNotices(t *testing.T) {
	m, _ := orderModel(t)

	updated, _ := m.Update(msgs.FeedStatus{Text: "lobste.rs keeps New"})
	m = updated.(Model)
	if !strings.Contains(m.noticeLine(), "lobste.rs keeps New") {
		t.Fatalf("status missing from %q", m.noticeLine())
	}

	updated, _ = m.Update(msgs.Notice{Text: "Reply posted"})
	m = updated.(Model)
	if line := m.noticeLine(); !strings.Contains(line, "Reply posted") || strings.Contains(line, "keeps New") {
		t.Errorf("a notice should cover the status: %q", line)
	}

	updated, _ = m.Update(noticeExpiredMsg{id: m.noticeID})
	m = updated.(Model)
	if !strings.Contains(m.noticeLine(), "lobste.rs keeps New") {
		t.Errorf("the status should return after the notice: %q", m.noticeLine())
	}
}

func collectMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, collectMsgs(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}
