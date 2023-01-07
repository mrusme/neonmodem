package windowmanager

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrusme/neonmodem/ui/cmd"
	"github.com/mrusme/neonmodem/ui/ctx"
	"github.com/mrusme/neonmodem/ui/helpers"
	"github.com/mrusme/neonmodem/ui/windows"
)

type StackItem struct {
	ID   string
	Win  windows.Window
	XYWH [4]int
}

type WM struct {
	ctx   *ctx.Ctx
	stack []StackItem
}

func New(c *ctx.Ctx) *WM {
	wm := new(WM)
	wm.ctx = c

	return wm
}

func (wm *WM) Open(id string, win windows.Window, xywh [4]int, command *cmd.Command) []tea.Cmd {
	var tcmds []tea.Cmd

	if wm.IsOpen(id) {
		if wm.IsFocused(id) {
			return tcmds
		}
		return wm.Focus(id)
	}

	item := new(StackItem)
	item.ID = id
	item.Win = win
	item.XYWH = xywh

	wm.stack = append(wm.stack, *item)

	command.AddArg("xywh", item.XYWH)
	tcmds = append(tcmds, wm.Update(id, *command))
	tcmds = append(tcmds, wm.Resize(id, wm.ctx.Content[0], wm.ctx.Content[1])...)
	// tcmds = append(tcmds, wm.Update(id, *cmd.New(
	// 	cmd.WinRefreshData,
	// 	id,
	// )))

	fcmds := wm.Focus(id)
	tcmds = append(tcmds, fcmds...)

	return tcmds
}

func (wm *WM) CloseFocused() (bool, []tea.Cmd) {
	return wm.Close(wm.Focused())
}

func (wm *WM) Close(id string, args ...cmd.Arg) (bool, []tea.Cmd) {
	var tcmds []tea.Cmd
	for i := len(wm.stack) - 1; i >= 0; i-- {
		if wm.stack[i].ID == id {
			wm.stack = append(wm.stack[:i], wm.stack[i+1:]...)
			tcmds = append(tcmds, cmd.New(cmd.WinClose, id, args...).Tea())
			wm.ctx.Loading = false

			nrOpen := wm.GetNumberOpen()
			if nrOpen == 0 {
				tcmds = append(tcmds, cmd.New(cmd.ViewFocus, "*").Tea())
			} else {
				tcmds = append(tcmds, wm.Focus(wm.stack[(nrOpen-1)].ID)...)
			}
			return true, tcmds
		}
	}

	return false, tcmds
}

func (wm *WM) Focus(id string) []tea.Cmd {
	var tcmds []tea.Cmd

	for i := 0; i < len(wm.stack); i++ {
		var tcmd tea.Cmd
		if wm.stack[i].ID == id {
			wm.stack[i].Win, tcmd = wm.stack[i].Win.Update(*cmd.New(cmd.WinFocus, wm.stack[i].ID))
		} else {
			wm.stack[i].Win, tcmd = wm.stack[i].Win.Update(*cmd.New(cmd.WinBlur, wm.stack[i].ID))
		}
		tcmds = append(tcmds, tcmd)
	}

	tcmds = append(tcmds, cmd.New(cmd.ViewBlur, "*").Tea())
	return tcmds
}

func (wm *WM) Focused() string {
	l := len(wm.stack) - 1
	if l < 0 {
		return ""
	}

	return wm.stack[l].ID
}

func (wm *WM) IsOpen(id string) bool {
	for _, win := range wm.stack {
		if win.ID == id {
			return true
		}
	}
	return false
}

func (wm *WM) IsFocused(id string) bool {
	return id == wm.Focused()
}

func (wm *WM) GetNumberOpen() int {
	return len(wm.stack)
}

func (wm *WM) Update(id string, msg tea.Msg) tea.Cmd {
	var teaCmd tea.Cmd

	for i := 0; i < len(wm.stack); i++ {
		if wm.stack[i].ID == id {
			wm.stack[i].Win, teaCmd = wm.stack[i].Win.Update(msg)
		}
	}

	return teaCmd
}

func (wm *WM) UpdateAll(msg tea.Msg) []tea.Cmd {
	var tcmd tea.Cmd
	var tcmds []tea.Cmd

	for i := 0; i < len(wm.stack); i++ {
		wm.stack[i].Win, tcmd = wm.stack[i].Win.Update(msg)
		tcmds = append(tcmds, tcmd)
	}

	return tcmds
}

func (wm *WM) UpdateFocused(msg tea.Msg) []tea.Cmd {
	var tcmd tea.Cmd
	var tcmds []tea.Cmd

	i := len(wm.stack) - 1
	if i < 0 {
		return tcmds
	}

	wm.stack[i].Win, tcmd = wm.stack[i].Win.Update(msg)
	tcmds = append(tcmds, tcmd)

	return tcmds
}

func (wm *WM) Resize(id string, w int, h int) []tea.Cmd {
	var tcmd tea.Cmd
	var tcmds []tea.Cmd

	for i := 0; i < len(wm.stack); i++ {
		if wm.stack[i].ID == id {
			wm.stack[i].Win, tcmd = wm.stack[i].Win.Update(tea.WindowSizeMsg{
				Width:  w - wm.stack[i].XYWH[0] - wm.stack[i].XYWH[2],
				Height: h - wm.stack[i].XYWH[1] - wm.stack[i].XYWH[3],
			})
			tcmds = append(tcmds, tcmd)
		}
	}

	return tcmds
}

func (wm *WM) ResizeAll(w int, h int) []tea.Cmd {
	var tcmd tea.Cmd
	var tcmds []tea.Cmd

	for i := 0; i < len(wm.stack); i++ {
		wm.stack[i].Win, tcmd = wm.stack[i].Win.Update(tea.WindowSizeMsg{
			Width:  w - wm.stack[i].XYWH[0] - wm.stack[i].XYWH[2],
			Height: h - wm.stack[i].XYWH[1] - wm.stack[i].XYWH[3],
		})
		tcmds = append(tcmds, tcmd)
	}

	return tcmds
}

func (wm *WM) View(view string, onlyFocused bool) string {
	var v string = view
	var j int = 0

	if onlyFocused {
		j = len(wm.stack) - 1
		if j < 0 {
			j = 0
		}
	}

	for i := j; i < len(wm.stack); i++ {
		v = helpers.PlaceOverlay(
			wm.stack[i].XYWH[0],
			wm.stack[i].XYWH[1]+(wm.ctx.Screen[1]-wm.ctx.Content[1]),
			wm.stack[i].Win.View(),
			v,
			true,
		)
	}

	return v
}
