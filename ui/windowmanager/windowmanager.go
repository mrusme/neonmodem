package windowmanager

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrusme/gobbs/ui/cmd"
	"github.com/mrusme/gobbs/ui/helpers"
	"github.com/mrusme/gobbs/ui/windows"
)

type StackItem struct {
	ID   string
	Win  windows.Window
	XYWH [4]int
}

type WM struct {
	stack []StackItem
}

func New() *WM {
	wm := new(WM)

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

	// tcmds = append(tcmds, wm.Update(id, *command))
	// tcmds = append(tcmds, wm.Update(id, tea.WindowSizeMsg{
	// 	Width:  item.XYWH[2],
	// 	Height: item.XYWH[3],
	// }))
	// tcmds = append(tcmds, wm.Update(id, *cmd.New(
	// 	cmd.WinRefreshData,
	// 	id,
	// )))

	// fcmds := wm.Focus(id)
	// tcmds = append(tcmds, fcmds...)

	return tcmds
}

func (wm *WM) CloseFocused() bool {
	return wm.Close(wm.Focused())
}

func (wm *WM) Close(id string) bool {
	for i := len(wm.stack) - 1; i >= 0; i-- {
		if wm.stack[i].ID == id {
			wm.stack = append(wm.stack[:i], wm.stack[i+1:]...)
			return true
		}
	}

	return false
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

func (wm *WM) View(view string) string {
	var v string = view

	for i := 0; i < len(wm.stack); i++ {
		v = helpers.PlaceOverlay(3, 2,
			wm.stack[i].Win.View(),
			v, true)
	}

	return v
}
