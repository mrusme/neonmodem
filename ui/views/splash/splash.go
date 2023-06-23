package splash

import (
	"bytes"
	"image/color"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/eliukblau/pixterm/pkg/ansimage"
	"github.com/mrusme/neonmodem/ui/cmd"
	"github.com/mrusme/neonmodem/ui/ctx"
	"github.com/mrusme/neonmodem/ui/views/posts"
)

var (
	VIEW_ID = "splash"
)

type Model struct {
	ctx          *ctx.Ctx
	pix          *ansimage.ANSImage
	splashscreen []byte
}

func (m Model) Init() tea.Cmd {
	return nil
}

func NewModel(c *ctx.Ctx) Model {
	var err error

	m := Model{
		ctx: c,
		pix: nil,
	}
	if !m.ctx.Config.RenderSplash {
		return m
	}

	m.splashscreen, err = m.ctx.EmbedFS.ReadFile("splashscreen.png")
	if err != nil {
		m.ctx.Logger.Error(err)
	}

	m.ctx.Logger.Debugf("Screen W/H: %d %d\n", m.ctx.Screen[0], m.ctx.Screen[1])

	return m
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var err error

	// var lcmd tea.Cmd
	// m.list, lcmd = m.list.Update(msg)
	// cmds = append(cmds, lcmd)
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.ctx.Screen[0] = msg.Width
		m.ctx.Screen[1] = msg.Height
		m.pix, err = ansimage.NewScaledFromReader(
			bytes.NewReader(m.splashscreen),
			m.ctx.Screen[1]*2,
			m.ctx.Screen[0],
			color.Transparent,
			ansimage.ScaleModeFill,
			ansimage.NoDithering,
		)
		if err != nil {
			m.ctx.Logger.Error(err)
		}
		return m, m.sleep()

	}

	return m, nil
}

func (m *Model) sleep() tea.Cmd {
	return func() tea.Msg {
		if m.ctx.Config.RenderSplash {
			time.Sleep(time.Second * 5)
		}

		c := cmd.New(
			cmd.ViewOpen,
			posts.VIEW_ID,
		)
		return *c
	}
}

func (m Model) View() string {
	return m.buildView(true)
}

func (m Model) buildView(cached bool) string {
	if m.pix != nil {
		return m.pix.RenderExt(false, false)
	}

	return ""
}
