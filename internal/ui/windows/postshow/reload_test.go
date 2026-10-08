package postshow

import (
	"context"
	"fmt"
	"log/slog"
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
)

type countingSystem struct {
	loads int
}

func (s *countingSystem) Kind() string                      { return "counting" }
func (s *countingSystem) URL() string                       { return "" }
func (s *countingSystem) Title() string                     { return "counting" }
func (s *countingSystem) Description() string               { return "counting" }
func (s *countingSystem) Capabilities() system.Capabilities { return system.CapRead | system.CapWrite }
func (s *countingSystem) Connect(context.Context, prompt.Prompter, string) (system.Settings, error) {
	return system.Settings{}, nil
}
func (s *countingSystem) ListForums(context.Context) ([]forum.Forum, error) { return nil, nil }
func (s *countingSystem) Orders(string) system.Ordering                     { return system.Only(system.OrderNew) }
func (s *countingSystem) ListPosts(context.Context, string, system.Order) ([]post.Post, error) {
	return nil, nil
}
func (s *countingSystem) LoadPost(ctx context.Context, p *post.Post) error {
	s.loads++
	p.Body = fmt.Sprintf("body after load %d", s.loads)
	p.Replies = nil
	for i := range s.loads {
		p.Replies = append(p.Replies, reply.Reply{ID: fmt.Sprintf("r%d", i), PostID: p.ID, Body: fmt.Sprintf("reply %d", i)})
	}
	return nil
}
func (s *countingSystem) CreatePost(context.Context, *post.Post) error    { return nil }
func (s *countingSystem) CreateReply(context.Context, *reply.Reply) error { return nil }

func run(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command")
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("the load command produced no message")
	}
	m.Update(msg)
}

func TestReloadAppliesFreshContent(t *testing.T) {
	cfg := config.Defaults("/cache")
	cfg.RenderImages = false
	sys := &countingSystem{}
	c := ctx.New(nil, &cfg, slog.New(slog.DiscardHandler), []system.System{sys})

	m := NewModel(&c)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m.Update(msgs.FocusWindow{ID: WIN_ID})

	_, cmd := m.Update(msgs.OpenPost{Post: post.Post{ID: "p", Subject: "s", SysIDX: 0}})
	run(t, m, cmd)
	if len(m.allReplies) != 1 {
		t.Fatalf("after the first load expected 1 reply, got %d", len(m.allReplies))
	}
	first := m.View()

	_, cmd = m.Update(msgs.ReloadPost{})
	run(t, m, cmd)
	if len(m.allReplies) != 2 {
		t.Fatalf("after the reload expected 2 replies, got %d", len(m.allReplies))
	}
	if m.loading || c.IsLoading() {
		t.Error("loading flags should be cleared after the reload")
	}
	second := m.View()
	if first == second {
		t.Fatal("the view did not change after the reload")
	}
}
