package posts

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/mrusme/neonmodem/internal/config"
	"github.com/mrusme/neonmodem/internal/models/author"
	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/prompt"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
)

type fakeSystem struct {
	title    string
	ordering system.Ordering
	lists    map[system.Order][]post.Post

	mu    sync.Mutex
	calls map[system.Order]int
	err   error
}

func (s *fakeSystem) Kind() string                      { return "fake" }
func (s *fakeSystem) URL() string                       { return "" }
func (s *fakeSystem) Title() string                     { return s.title }
func (s *fakeSystem) Description() string               { return s.title }
func (s *fakeSystem) Capabilities() system.Capabilities { return system.CapRead }
func (s *fakeSystem) Connect(context.Context, prompt.Prompter, string) (system.Settings, error) {
	return system.Settings{}, nil
}
func (s *fakeSystem) ListForums(context.Context) ([]forum.Forum, error) { return nil, nil }
func (s *fakeSystem) Orders(string) system.Ordering                     { return s.ordering }
func (s *fakeSystem) ListPosts(_ context.Context, _ string, order system.Order) ([]post.Post, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.calls == nil {
		s.calls = map[system.Order]int{}
	}
	s.calls[order]++
	if s.err != nil {
		return nil, s.err
	}
	return s.lists[order], nil
}

func (s *fakeSystem) fail(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}
func (s *fakeSystem) LoadPost(context.Context, *post.Post) error      { return nil }
func (s *fakeSystem) CreatePost(context.Context, *post.Post) error    { return nil }
func (s *fakeSystem) CreateReply(context.Context, *reply.Reply) error { return nil }

func (s *fakeSystem) called(order system.Order) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[order]
}

func at(h int) time.Time {
	return time.Date(2026, 10, 7, h, 0, 0, 0, time.UTC)
}

func newSystems() (*fakeSystem, *fakeSystem) {
	lemmy := &fakeSystem{
		title:    "lemmy.example",
		ordering: system.Ordering{Default: system.OrderNew, Supported: system.AllOrders()},
		lists: map[system.Order][]post.Post{
			system.OrderNew:      {{ID: "lemmy-new", CreatedAt: at(5), SysIDX: 0}},
			system.OrderHot:      {{ID: "lemmy-hot", CreatedAt: at(1), SysIDX: 0}},
			system.OrderComments: {{ID: "lemmy-busy", ReplyCount: 900, CreatedAt: at(0), SysIDX: 0}},
		},
	}
	tags := &fakeSystem{
		title:    "tags.example",
		ordering: system.Only(system.OrderNew),
		lists: map[system.Order][]post.Post{
			system.OrderNew: {
				{ID: "tag-quiet", ReplyCount: 2, CreatedAt: at(4), SysIDX: 1},
				{ID: "tag-busy", ReplyCount: 9, CreatedAt: at(3), SysIDX: 1},
			},
		},
	}
	return lemmy, tags
}

func testModel(t *testing.T, systems ...system.System) (Model, *ctx.Ctx) {
	t.Helper()

	cfg := config.Defaults("/cache")
	c := ctx.New(nil, &cfg, slog.New(slog.DiscardHandler), systems)
	c.Content = [2]int{100, 30}
	m := NewModel(&c)
	v, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return v.(Model), &c
}

func run(t *testing.T, m Model, cmd tea.Cmd) (Model, []string) {
	t.Helper()

	var statuses []string
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		if next == nil {
			continue
		}
		switch msg := next().(type) {
		case tea.BatchMsg:
			queue = append(queue, msg...)
		case msgs.FeedResult:
			v, more := m.Update(msg)
			m = v.(Model)
			queue = append(queue, more)
		case msgs.FeedStatus:
			statuses = append(statuses, msg.Text)
		}
	}
	return m, statuses
}

func ids(m Model) string {
	var out []string
	for _, item := range m.list.Items() {
		out = append(out, item.(Item).ID)
	}
	return strings.Join(out, ",")
}

func TestOrderChangesReloadOnlyTheSystemsThatChange(t *testing.T) {
	lemmy, tags := newSystems()
	m, c := testModel(t, lemmy, tags)

	m, statuses := run(t, m, m.refresh())
	if ids(m) != "lemmy-new,tag-quiet,tag-busy" {
		t.Fatalf("New should sort by creation time, got %s", ids(m))
	}
	if len(statuses) != 0 {
		t.Errorf("no status expected while every system lists New: %q", statuses)
	}

	c.SetOrder(system.OrderHot)
	v, cmd := m.Update(msgs.OrderChanged{})
	m, statuses = run(t, v.(Model), cmd)
	if lemmy.called(system.OrderHot) != 1 || tags.called(system.OrderNew) != 1 {
		t.Errorf("only lemmy should reload: lemmy hot %d, tags new %d",
			lemmy.called(system.OrderHot), tags.called(system.OrderNew))
	}
	if ids(m) != "lemmy-hot,tag-quiet,tag-busy" {
		t.Errorf("Hot should interleave by rank, got %s", ids(m))
	}
	if last(statuses) != "tags.example keeps New" {
		t.Errorf("status %q", statuses)
	}

	c.SetOrder(system.OrderComments)
	v, cmd = m.Update(msgs.OrderChanged{})
	m, statuses = run(t, v.(Model), cmd)
	if tags.called(system.OrderNew) != 1 {
		t.Error("tags.example keeps the same list and must not reload")
	}
	if ids(m) != "lemmy-busy,tag-busy,tag-quiet" {
		t.Errorf("Most comments should sort the fallback list by replies, got %s", ids(m))
	}
	if last(statuses) != "tags.example sorts its newest posts by replies" {
		t.Errorf("status %q", statuses)
	}

	v, cmd = m.Update(msgs.RefreshFeed{})
	_, _ = run(t, v.(Model), cmd)
	if tags.called(system.OrderNew) != 2 || lemmy.called(system.OrderComments) != 2 {
		t.Error("a refresh should reload every selected system")
	}
}

func TestOrderChangeWithoutReloadsRemergesAtOnce(t *testing.T) {
	_, tags := newSystems()
	m, c := testModel(t, tags)
	m, _ = run(t, m, m.refresh())

	c.SetOrder(system.OrderComments)
	v, cmd := m.Update(msgs.OrderChanged{})
	m = v.(Model)
	if ids(m) != "tag-busy,tag-quiet" {
		t.Errorf("the kept posts should be sorted again right away, got %s", ids(m))
	}
	if m.pending != 0 || m.ctx.IsLoading() {
		t.Error("nothing should be loading")
	}
	_, statuses := run(t, m, cmd)
	if last(statuses) != "tags.example sorts its newest posts by replies" {
		t.Errorf("status %q", statuses)
	}
}

func TestRowsShowScoreRepliesAndSystem(t *testing.T) {
	item := Item{
		Post: post.Post{
			Author:     author.Author{ID: "alice", Name: "alice"},
			CreatedAt:  time.Now().Add(-3 * time.Hour),
			Forum:      forum.Forum{Name: "Show HN"},
			ReplyCount: 87,
			Score:      post.Score{Value: 312, Unit: post.ScorePoints},
		},
		SystemTitle: "news.ycombinator.com",
	}
	want := "by alice 3 hours ago in Show HN · 312 points · 87 replies · news.ycombinator.com"
	if got := item.Description(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func last(statuses []string) string {
	if len(statuses) == 0 {
		return ""
	}
	return statuses[len(statuses)-1]
}

func TestStatusNamesSystemsThatNeedConnect(t *testing.T) {
	lemmy, tags := newSystems()
	lemmy.fail(system.NeedsConnect("the session of vera has ended; log in again with " +
		"`neonmodem connect --type lemmy --url https://lemmy.example`"))
	m, c := testModel(t, lemmy, tags)
	m, _ = run(t, m, m.refresh())

	c.SetOrder(system.OrderHot)
	v, cmd := m.Update(msgs.OrderChanged{})
	m, statuses := run(t, v.(Model), cmd)
	if ids(m) != "tag-quiet,tag-busy" {
		t.Errorf("got %s", ids(m))
	}
	want := "lemmy.example needs `neonmodem connect`; tags.example keeps New"
	if last(statuses) != want {
		t.Errorf("got %q, want %q", last(statuses), want)
	}

	lemmy.fail(errors.New("connection refused"))
	m, statuses = run(t, m, m.refresh())
	if last(statuses) != "tags.example keeps New" {
		t.Errorf("other errors must stay out of the status line: %q", statuses)
	}

	lemmy.fail(nil)
	_, statuses = run(t, m, m.refresh())
	if last(statuses) != "tags.example keeps New" {
		t.Errorf("the phrase should go once the system loads: %q", statuses)
	}
}

func TestNeedsConnectNamesTheSystems(t *testing.T) {
	for _, tc := range []struct {
		names []string
		text  string
	}{
		{nil, ""},
		{[]string{"lemmy.ml"}, "lemmy.ml needs `neonmodem connect`"},
		{[]string{"lemmy.ml", "a.example", "b.example"},
			"lemmy.ml, a.example and b.example need `neonmodem connect`"},
	} {
		if got := needsConnect(tc.names); got != tc.text {
			t.Errorf("%v: got %q, want %q", tc.names, got, tc.text)
		}
	}
}
