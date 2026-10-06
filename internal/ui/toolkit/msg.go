package toolkit

import (
	"strconv"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
)

type MsgHandlingKeymapKey struct {
	ID      string
	Handler func(m interface{}) (bool, []tea.Cmd)
}

type MsgHandling struct {
	OnKeymapKey      []MsgHandlingKeymapKey
	OnAnyNumberKey   func(m interface{}, n int8) (bool, []tea.Cmd)
	OnAnyUncaughtKey func(m interface{}, k tea.KeyPressMsg) (bool, []tea.Cmd)
	OnViewResize     func(m interface{}) (bool, []tea.Cmd)
}

func (tk *ToolKit) SetMsgHandling(mh MsgHandling) {
	tk.mh = mh
}

func (tk *ToolKit) HandleMsg(m interface{}, msg tea.Msg) (bool, []tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		for _, k := range tk.mh.OnKeymapKey {
			if key.Matches(msg, tk.KeymapGet(k.ID)) {
				return k.Handler(m)
			}
		}

		if tk.mh.OnAnyNumberKey != nil && msg.Mod == 0 {
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
		tk.viewcache = ""
		if tk.mh.OnViewResize != nil {
			return tk.mh.OnViewResize(m)
		}
		return false, nil

	case msgs.FocusWindow:
		if msg.ID == tk.winID || msg.ID == "*" {
			tk.Focus(m)
		}
		return true, nil

	case msgs.BlurWindow:
		if msg.ID == tk.winID || msg.ID == "*" {
			tk.Blur(m)
		}
		return true, nil

	case msgs.ThemeChanged:
		tk.viewcache = ""
		return false, nil
	}

	return false, nil
}
