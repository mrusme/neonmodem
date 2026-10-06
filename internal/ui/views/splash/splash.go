package splash

import (
	"bytes"
	"image/color"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/eliukblau/pixterm/pkg/ansimage"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
	"github.com/mrusme/neonmodem/internal/ui/views"
)

const Duration = 5 * time.Second

type Model struct {
	ctx          *ctx.Ctx
	pix          *ansimage.ANSImage
	splashscreen []byte
	started      bool
	skipped      bool
}

func NewModel(c *ctx.Ctx) Model {
	m := Model{ctx: c}
	if !c.Config.RenderSplash || c.EmbedFS == nil {
		return m
	}

	data, err := c.EmbedFS.ReadFile("splashscreen.png")
	if err != nil {
		c.Logger.Error("reading the splash screen", "error", err)
		return m
	}
	m.splashscreen = data

	return m
}

func (m Model) enabled() bool {
	return m.ctx.Config.RenderSplash && len(m.splashscreen) > 0
}

func (m Model) Update(msg tea.Msg) (views.View, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if m.enabled() {
			pix, err := ansimage.NewScaledFromReader(
				bytes.NewReader(m.splashscreen),
				msg.Height*2,
				msg.Width,
				color.Transparent,
				ansimage.ScaleModeFill,
				ansimage.NoDithering,
			)
			if err != nil {
				m.ctx.Logger.Error("rendering the splash screen", "error", err)
			} else {
				m.pix = pix
			}
		}

		if m.started {
			return m, nil
		}
		m.started = true

		if !m.enabled() {
			return m, msgs.Send(msgs.ShowPosts{})
		}
		return m, tea.Tick(Duration, func(time.Time) tea.Msg {
			return msgs.ShowPosts{}
		})

	case tea.KeyPressMsg:
		if m.started && !m.skipped {
			m.skipped = true
			return m, msgs.Send(msgs.ShowPosts{})
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.pix != nil {
		return m.pix.RenderExt(false, false)
	}

	return ""
}
