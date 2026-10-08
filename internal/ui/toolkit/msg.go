package toolkit

import (
	"strconv"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
)

type Handler func() (bool, []tea.Cmd)

type MsgHandlingKeymapKey struct {
	ID      string
	Handler Handler
}

type MsgHandling struct {
	OnKeymapKey      []MsgHandlingKeymapKey
	OnAnyNumberKey   func(n int8) (bool, []tea.Cmd)
	OnAnyUncaughtKey func(k tea.KeyPressMsg) (bool, []tea.Cmd)
	OnViewResize     Handler
}

func (tk *ToolKit) SetMsgHandling(mh MsgHandling) {
	tk.mh = mh
}

func (tk *ToolKit) HandleMsg(msg tea.Msg) (bool, []tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		for _, k := range tk.mh.OnKeymapKey {
			if key.Matches(msg, tk.KeymapGet(k.ID)) {
				return k.Handler()
			}
		}

		if tk.mh.OnAnyNumberKey != nil && msg.Mod == 0 {
			switch msg.String() {
			case "1", "2", "3", "4", "5", "6", "7", "8", "9", "0":
				n, _ := strconv.Atoi(msg.String())
				return tk.mh.OnAnyNumberKey(int8(n))
			}
		}

		if tk.mh.OnAnyUncaughtKey != nil {
			return tk.mh.OnAnyUncaughtKey(msg)
		}

	case tea.WindowSizeMsg:
		tk.wh[0] = msg.Width
		tk.wh[1] = msg.Height
		tk.viewcache = ""
		if tk.mh.OnViewResize != nil {
			return tk.mh.OnViewResize()
		}
		return false, nil

	case msgs.FocusWindow:
		if msg.ID == tk.winID || msg.ID == "*" {
			tk.Focus()
		}
		return true, nil

	case msgs.BlurWindow:
		if msg.ID == tk.winID || msg.ID == "*" {
			tk.Blur()
		}
		return true, nil

	case msgs.ThemeChanged:
		tk.viewcache = ""
		return false, nil
	}

	return false, nil
}
