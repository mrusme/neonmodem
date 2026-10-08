package ui

import (
	"context"
	"log/slog"
	"testing"

	"charm.land/bubbles/v2/list"
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

type fakeSystem struct {
	listForumsCalls int
}

func (s *fakeSystem) Kind() string                      { return "fake" }
func (s *fakeSystem) URL() string                       { return "" }
func (s *fakeSystem) Title() string                     { return "fake" }
func (s *fakeSystem) Description() string               { return "fake" }
func (s *fakeSystem) Capabilities() system.Capabilities { return system.CapListForums }
func (s *fakeSystem) Connect(ctx context.Context, p prompt.Prompter, u string) (system.Settings, error) {
	return system.Settings{}, nil
}
func (s *fakeSystem) ListForums(ctx context.Context) ([]forum.Forum, error) {
	s.listForumsCalls++
	return []forum.Forum{
		{ID: "f1", Name: "Cloud Providers/AWS"},
		{ID: "f2", Name: "Cloud Providers/GCP"},
	}, nil
}
func (s *fakeSystem) Orders(string) system.Ordering { return system.Only(system.OrderNew) }
func (s *fakeSystem) ListPosts(ctx context.Context, forumID string, order system.Order) ([]post.Post, error) {
	return nil, nil
}
func (s *fakeSystem) LoadPost(ctx context.Context, p *post.Post) error      { return nil }
func (s *fakeSystem) CreatePost(ctx context.Context, p *post.Post) error    { return nil }
func (s *fakeSystem) CreateReply(ctx context.Context, r *reply.Reply) error { return nil }

func testModel(t *testing.T, fake *fakeSystem) (Model, *ctx.Ctx) {
	t.Helper()

	cfg := &config.Config{}
	c := ctx.New(nil, cfg, slog.New(slog.DiscardHandler), []system.System{fake})
	m := NewModel(&c)
	m.setSizes(120, 48)
	m.currentView = 1

	return m, &c
}

// TestForumSelectorDoesNotListOnTheEventLoop is the regression test for opening
// the forum selector: it queries every connected system, so that must not
// happen inside Update.
func TestForumSelectorDoesNotListOnTheEventLoop(t *testing.T) {
	fake := &fakeSystem{}
	m, c := testModel(t, fake)

	_, teaCmd := m.Update(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})

	if fake.listForumsCalls != 0 {
		t.Fatal("ListForums ran on the event loop instead of in a command")
	}
	if teaCmd == nil {
		t.Fatal("opening the selector scheduled no work at all")
	}
	if !c.IsLoading() {
		t.Error("expected the spinner to be running while forums are fetched")
	}
	if !m.wm.IsOpen(popuplist.WIN_ID) {
		t.Error("the selector window should be open right away")
	}
}

// TestListForumsCommandDeliversItems covers the background half: the command
// fetches and addresses the result at the already-open selector.
func TestListForumsCommandDeliversItems(t *testing.T) {
	fake := &fakeSystem{}
	m, _ := testModel(t, fake)

	all := forum.Forum{ID: "", Name: "All"}
	msg := m.listForums(all)()

	if fake.listForumsCalls != 1 {
		t.Fatalf("expected ListForums to run once, got %d", fake.listForumsCalls)
	}

	result, ok := msg.(msgs.PickerItems)
	if !ok {
		t.Fatalf("expected a PickerItems message, got %T", msg)
	}
	if result.Kind != msgs.PickForum {
		t.Fatalf("result is not a forum picker result: %v", result.Kind)
	}
	// "All" stays first, then whatever the systems returned.
	if len(result.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result.Items))
	}
	if got := result.Items[0].(forum.Forum).Name; got != "All" {
		t.Errorf("expected \"All\" to stay first, got %q", got)
	}
	if got := result.Items[1].(forum.Forum).Name; got != "Cloud Providers/AWS" {
		t.Errorf("unexpected second item %q", got)
	}
}

func TestSystemItemsStartWithAll(t *testing.T) {
	m, _ := testModel(t, &fakeSystem{})

	items := m.systemItems()
	if len(items) != 2 {
		t.Fatalf("expected the All row plus one system, got %d", len(items))
	}
	first, ok := items[0].(SystemItem)
	if !ok || first.Index != -1 || first.Title() != "All" {
		t.Fatalf("unexpected first item: %#v", items[0])
	}
	second := items[1].(SystemItem)
	if second.Index != 0 || second.Title() != "fake" {
		t.Fatalf("unexpected second item: %#v", second)
	}
}

func TestPickingASystemClosesTheSelectorAndRefreshes(t *testing.T) {
	m, c := testModel(t, &fakeSystem{})

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl})
	m = updated.(Model)
	if !m.wm.IsOpen(popuplist.WIN_ID) {
		t.Fatal("system selector did not open")
	}

	updated, cmd := m.Update(msgs.Picked{Kind: msgs.PickSystem, Item: SystemItem{Index: 0}})
	m = updated.(Model)
	if m.wm.IsOpen(popuplist.WIN_ID) {
		t.Error("selector should close after picking")
	}
	if c.GetCurrentSystem() != 0 {
		t.Errorf("current system %d, want 0", c.GetCurrentSystem())
	}
	if cmd == nil {
		t.Fatal("picking should schedule a refresh")
	}
}

func TestNoticeShowsAndExpires(t *testing.T) {
	m, _ := testModel(t, &fakeSystem{})

	updated, cmd := m.Update(msgs.Notice{Text: "Reply posted"})
	m = updated.(Model)
	if m.notice != "Reply posted" {
		t.Fatalf("notice not stored: %q", m.notice)
	}
	if cmd == nil {
		t.Fatal("a notice must schedule its expiry")
	}

	updated, _ = m.Update(noticeExpiredMsg{id: m.noticeID})
	m = updated.(Model)
	if m.notice != "" {
		t.Error("notice should be cleared when its timer fires")
	}
}

func TestEscapeWithoutWindowsReachesTheView(t *testing.T) {
	m, _ := testModel(t, &fakeSystem{})

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected the posts view to quit on esc")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", cmd())
	}
}

var _ list.Item = SystemItem{}
