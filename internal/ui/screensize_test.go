package ui

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mrusme/neonmodem/internal/config"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/header"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
)

type longTitles struct {
	countingSystem
}

func (s *longTitles) ListPosts(context.Context, string, system.Order) ([]post.Post, error) {
	var posts []post.Post
	for i := range 40 {
		posts = append(posts, post.Post{
			ID:      strings.Repeat("x", i+1),
			Subject: strings.Repeat("A long subject that must be cut, not wrapped ", 4),
			SysIDX:  0,
		})
	}
	return posts, nil
}

func TestThePostsViewAndTheScreenRenderExactlyTheirSize(t *testing.T) {
	cfg := config.Defaults("/cache")
	cfg.RenderShadows = false

	for _, size := range [][2]int{{60, 20}, {80, 24}, {100, 30}, {120, 36}} {
		for _, state := range []struct {
			name string
			sys  system.System
		}{{"with posts", &longTitles{}}, {"empty", &countingSystem{}}} {
			c := ctx.New(nil, &cfg, slog.New(slog.DiscardHandler), []system.System{state.sys})
			m := NewModel(&c)
			updated, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			m = updated.(Model)
			m.currentView = 1
			updated, cmd := m.Update(msgs.RefreshFeed{})
			m, _ = settleModel(t, updated.(Model), cmd)

			if h := lipgloss.Height(m.header.View()); h != header.Height {
				t.Errorf("%dx%d: the header renders %d rows, Height says %d", size[0], size[1], h, header.Height)
			}
			if c.Content[1] != size[1]-header.Height-noticeHeight {
				t.Errorf("%dx%d: content height %d", size[0], size[1], c.Content[1])
			}

			view := m.views[1].View()
			if w, h := lipgloss.Width(view), lipgloss.Height(view); w != c.Content[0] || h != c.Content[1] {
				t.Errorf("%dx%d %s: the posts view renders %dx%d, the content area is %dx%d",
					size[0], size[1], state.name, w, h, c.Content[0], c.Content[1])
			}
			screen := m.render()
			if w, h := lipgloss.Width(screen), lipgloss.Height(screen); w != size[0] || h != size[1] {
				t.Errorf("%dx%d %s: the screen renders %dx%d", size[0], size[1], state.name, w, h)
			}
		}
	}
}
