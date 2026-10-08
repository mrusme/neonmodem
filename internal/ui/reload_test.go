package ui

import (
	"context"
	"fmt"
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
	p.Replies = nil
	for i := range s.loads {
		p.Replies = append(p.Replies, reply.Reply{ID: fmt.Sprintf("r%d", i), PostID: p.ID,
			Body: fmt.Sprintf("reply number %d", i), Author: author(i)})
	}
	return nil
}
func (s *countingSystem) CreatePost(context.Context, *post.Post) error    { return nil }
func (s *countingSystem) CreateReply(context.Context, *reply.Reply) error { return nil }

func author(i int) (a struct{ ID, Name string }) {
	a.ID = fmt.Sprintf("u%d", i)
	a.Name = fmt.Sprintf("user%d", i)
	return a
}

func drain(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	msg := cmd()
	if msg == nil {
		return m
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			m = drain(t, m, c)
		}
		return m
	}
	updated, next := m.Update(msg)
	m = updated.(Model)
	return drain(t, m, next)
}

func TestReloadAfterReplyReachesThePostWindow(t *testing.T) {
	cfg := config.Defaults("/cache")
	cfg.RenderImages = false
	cfg.RenderShadows = false
	sys := &countingSystem{}
	c := ctx.New(nil, &cfg, slog.New(slog.DiscardHandler), []system.System{sys})

	m := NewModel(&c)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 48})
	m = updated.(Model)
	m.currentView = 1

	updated, cmd := m.Update(msgs.OpenPost{Post: post.Post{ID: "p", Subject: "subject", SysIDX: 0}})
	m = drain(t, updated.(Model), cmd)
	if sys.loads != 1 {
		t.Fatalf("expected one load after opening, got %d", sys.loads)
	}
	before := m.View().Content
	if !strings.Contains(before, "reply number 0") {
		t.Fatalf("first render does not show the reply:\n%s", before)
	}

	updated, cmd = m.Update(msgs.ReloadPost{})
	m = drain(t, updated.(Model), cmd)
	if sys.loads != 2 {
		t.Fatalf("expected a second load after the reload, got %d", sys.loads)
	}
	after := m.View().Content
	if !strings.Contains(after, "reply number 1") {
		t.Fatalf("the reloaded post is not shown:\n%s", after)
	}
}
