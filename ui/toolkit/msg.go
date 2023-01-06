package toolkit

import (
	"strconv"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrusme/gobbs/ui/cmd"
)

type MsgHandlingKeymapKey struct {
	ID      string
	Handler func(m interface{}) (bool, []tea.Cmd)
}

type MsgHandling struct {
	OnKeymapKey         []MsgHandlingKeymapKey
	OnAnyNumberKey      func(m interface{}, n int8) (bool, []tea.Cmd)
	OnAnyUncaughtKey    func(m interface{}, k tea.KeyMsg) (bool, []tea.Cmd)
	OnViewResize        func(m interface{}) (bool, []tea.Cmd)
	OnWinOpenCmd        func(m interface{}, c cmd.Command) (bool, []tea.Cmd)
	OnWinCloseCmd       func(m interface{}, c cmd.Command) (bool, []tea.Cmd)
	OnWinRefreshDataCmd func(m interface{}, c cmd.Command) (bool, []tea.Cmd)
	OnWinFreshDataCmd   func(m interface{}, c cmd.Command) (bool, []tea.Cmd)
	OnMsgErrorCmd       func(m interface{}, c cmd.Command) (bool, []tea.Cmd)
}

func (tk *ToolKit) SetMsgHandling(mh MsgHandling) {
	tk.mh = mh
}

func (tk *ToolKit) HandleMsg(m interface{}, msg tea.Msg) (bool, []tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		for i := 0; i < len(tk.mh.OnKeymapKey); i++ {
			if key.Matches(msg, tk.KeymapGet(tk.mh.OnKeymapKey[i].ID)) {
				return tk.mh.OnKeymapKey[i].Handler(m)
			}
		}

		if tk.mh.OnAnyNumberKey != nil {
			switch msg.String() {
			case "1", "2", "3", "4", "5", "6", "7", "8", "9", "0":
				n, _ := strconv.Atoi(msg.String())
				return tk.mh.OnAnyNumberKey(m, int8(n))
			}
		}

		if tk.mh.OnAnyUncaughtKey != nil {
			return tk.mh.OnAnyUncaughtKey(m, msg)
		}

	case tea.WindowSizeMsg:
		tk.wh[0] = msg.Width
		tk.wh[1] = msg.Height
		if tk.mh.OnViewResize != nil {
			return tk.mh.OnViewResize(m)
		}
		return false, cmds

	case cmd.Command:
		switch msg.Call {
		case cmd.WinFocus:
			if msg.Target == tk.winID ||
				msg.Target == "*" {
				tk.Focus(m)
			}
			return true, nil
		case cmd.WinBlur:
			if msg.Target == tk.winID ||
				msg.Target == "*" {
				tk.Blur(m)
			}
			return true, nil
		case cmd.WinOpen:
			if tk.mh.OnWinOpenCmd != nil {
				return tk.mh.OnWinOpenCmd(m, msg)
			}
		case cmd.WinClose:
			if tk.mh.OnWinCloseCmd != nil {
				return tk.mh.OnWinCloseCmd(m, msg)
			}
		case cmd.WinRefreshData:
			if tk.mh.OnWinRefreshDataCmd != nil {
				return tk.mh.OnWinRefreshDataCmd(m, msg)
			}
		case cmd.WinFreshData:
			if tk.mh.OnWinFreshDataCmd != nil {
				return tk.mh.OnWinFreshDataCmd(m, msg)
			}
		case cmd.MsgError:
			if tk.mh.OnMsgErrorCmd != nil {
				return tk.mh.OnMsgErrorCmd(m, msg)
			}
		}

	}
	return false, cmds
}
