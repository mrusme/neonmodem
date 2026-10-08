package windowmanager

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
	"github.com/mrusme/neonmodem/internal/ui/windows"
)

type Geometry struct {
	Left   int
	Top    int
	Right  int
	Bottom int
}

func Centered(contentWidth int, contentHeight int, width int, height int) Geometry {
	if width > contentWidth {
		width = contentWidth
	}
	if height > contentHeight {
		height = contentHeight
	}
	left := (contentWidth - width) / 2
	top := (contentHeight - height) / 2
	return Geometry{
		Left:   left,
		Top:    top,
		Right:  contentWidth - width - left,
		Bottom: contentHeight - height - top,
	}
}

type stackItem struct {
	id   string
	win  windows.Window
	geom Geometry
}

type WM struct {
	ctx   *ctx.Ctx
	stack []stackItem
}

func New(c *ctx.Ctx) *WM {
	return &WM{ctx: c}
}

func (wm *WM) Open(id string, win windows.Window, geom Geometry, init tea.Msg) []tea.Cmd {
	if i := wm.index(id); i >= 0 {
		item := wm.stack[i]
		item.geom = geom
		wm.stack = append(slices.Delete(wm.stack, i, i+1), item)
	} else {
		wm.stack = append(wm.stack, stackItem{id: id, win: win, geom: geom})
	}

	cmds := wm.Resize(id, wm.ctx.Content[0], wm.ctx.Content[1])
	if init != nil {
		cmds = append(cmds, wm.Update(id, init))
	}
	return append(cmds, wm.Focus(id)...)
}

func (wm *WM) index(id string) int {
	return slices.IndexFunc(wm.stack, func(item stackItem) bool { return item.id == id })
}

func (wm *WM) CloseFocused() (bool, []tea.Cmd) {
	return wm.Close(wm.Focused())
}

func (wm *WM) Close(id string) (bool, []tea.Cmd) {
	for i, item := range slices.Backward(wm.stack) {
		if item.id != id {
			continue
		}

		wm.stack = slices.Delete(wm.stack, i, i+1)

		cmds := []tea.Cmd{msgs.Send(msgs.WindowClosed{ID: id})}
		if len(wm.stack) == 0 {
			cmds = append(cmds, msgs.Send(msgs.FocusView{}))
		} else {
			cmds = append(cmds, wm.Focus(wm.stack[len(wm.stack)-1].id)...)
		}
		return true, cmds
	}

	return false, nil
}

func (wm *WM) Focus(id string) []tea.Cmd {
	var cmds []tea.Cmd

	for i := range wm.stack {
		var msg tea.Msg = msgs.BlurWindow{ID: wm.stack[i].id}
		if wm.stack[i].id == id {
			msg = msgs.FocusWindow{ID: id}
		}
		var cmd tea.Cmd
		wm.stack[i].win, cmd = wm.stack[i].win.Update(msg)
		cmds = append(cmds, cmd)
	}

	cmds = append(cmds, msgs.Send(msgs.BlurView{}))
	return cmds
}

func (wm *WM) Focused() string {
	if len(wm.stack) == 0 {
		return ""
	}
	return wm.stack[len(wm.stack)-1].id
}

func (wm *WM) IsOpen(id string) bool {
	return wm.index(id) >= 0
}

func (wm *WM) IsFocused(id string) bool {
	return id == wm.Focused()
}

func (wm *WM) GetNumberOpen() int {
	return len(wm.stack)
}

func (wm *WM) Update(id string, msg tea.Msg) tea.Cmd {
	i := wm.index(id)
	if i < 0 {
		return nil
	}

	var cmd tea.Cmd
	wm.stack[i].win, cmd = wm.stack[i].win.Update(msg)
	return cmd
}

func (wm *WM) UpdateAll(msg tea.Msg) []tea.Cmd {
	var cmds []tea.Cmd
	for i := range wm.stack {
		var cmd tea.Cmd
		wm.stack[i].win, cmd = wm.stack[i].win.Update(msg)
		cmds = append(cmds, cmd)
	}
	return cmds
}

func (wm *WM) UpdateFocused(msg tea.Msg) []tea.Cmd {
	i := len(wm.stack) - 1
	if i < 0 {
		return nil
	}

	var cmd tea.Cmd
	wm.stack[i].win, cmd = wm.stack[i].win.Update(msg)
	return []tea.Cmd{cmd}
}

func (wm *WM) sizeOf(item stackItem, w int, h int) tea.WindowSizeMsg {
	return tea.WindowSizeMsg{
		Width:  w - item.geom.Left - item.geom.Right,
		Height: h - item.geom.Top - item.geom.Bottom,
	}
}

func (wm *WM) Resize(id string, w int, h int) []tea.Cmd {
	i := wm.index(id)
	if i < 0 {
		return nil
	}

	var cmd tea.Cmd
	wm.stack[i].win, cmd = wm.stack[i].win.Update(wm.sizeOf(wm.stack[i], w, h))
	return []tea.Cmd{cmd}
}

func (wm *WM) ResizeAll(w int, h int) []tea.Cmd {
	var cmds []tea.Cmd
	for i := range wm.stack {
		var cmd tea.Cmd
		wm.stack[i].win, cmd = wm.stack[i].win.Update(wm.sizeOf(wm.stack[i], w, h))
		cmds = append(cmds, cmd)
	}
	return cmds
}

func (wm *WM) View(base string) string {
	if len(wm.stack) == 0 {
		return base
	}

	layers := []*lipgloss.Layer{lipgloss.NewLayer(base).Z(0)}
	yOffset := wm.ctx.Screen[1] - wm.ctx.Content[1]

	for i, item := range wm.stack {
		content := item.win.View()
		x := item.geom.Left
		y := item.geom.Top + yOffset

		if wm.ctx.Config.RenderShadows {
			layers = append(layers, lipgloss.NewLayer(shadow(content)).X(x+1).Y(y+1).Z(2*i+1))
		}
		layers = append(layers, lipgloss.NewLayer(content).X(x).Y(y).Z(2*i+2))
	}

	return lipgloss.NewCompositor(layers...).Render()
}

var shadowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#333333"))

func shadow(content string) string {
	width := lipgloss.Width(content)
	height := lipgloss.Height(content)
	if width == 0 || height == 0 {
		return ""
	}

	line := shadowStyle.Render(strings.Repeat("░", width))
	lines := make([]string, height)
	for i := range lines {
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}
