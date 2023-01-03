package toolkit

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrusme/gobbs/ui/cmd"
	"github.com/mrusme/gobbs/ui/theme"
	"go.uber.org/zap"
)

type ViewFunc func(m interface{}, cached bool) string

type ToolKit struct {
	winID  string
	theme  *theme.Theme
	logger *zap.SugaredLogger

	m       interface{}
	wh      [2]int
	focused bool

	viewfunc  ViewFunc
	viewcache string
}

func New(winID string, t *theme.Theme, l *zap.SugaredLogger) *ToolKit {
	tk := new(ToolKit)
	tk.winID = winID
	tk.theme = t
	tk.logger = l

	tk.wh = [2]int{0, 0}
	tk.focused = false

	return tk
}

func (tk *ToolKit) SetViewFunc(fn ViewFunc) {
	tk.viewfunc = fn
}

func (tk *ToolKit) CacheView(m interface{}) bool {
	if tk.viewfunc != nil {
		tk.viewcache = tk.viewfunc(m, false)
		return true
	}
	return false
}

func (tk *ToolKit) GetCachedView() string {
	return tk.viewcache
}

func (tk *ToolKit) IsCached() bool {
	return tk.viewcache != ""
}

func (tk *ToolKit) View(m interface{}, cached bool) string {
	return tk.viewfunc(m, cached)
}

func (tk *ToolKit) Focus(m interface{}) {
	tk.focused = true

	if tk.viewfunc != nil {
		tk.viewcache = tk.viewfunc(m, false)
	}
}

func (tk *ToolKit) Blur(m interface{}) {
	tk.focused = false

	if tk.viewfunc != nil {
		tk.viewcache = tk.viewfunc(m, false)
	}
}

func (tk *ToolKit) IsFocused() bool {
	return tk.focused
}

func (tk *ToolKit) ViewWidth() int {
	return tk.wh[0]
}

func (tk *ToolKit) ViewHeight() int {
	return tk.wh[1]
}

func (tk *ToolKit) HandleMsg(m interface{}, msg tea.Msg) (bool, []tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		tk.wh[0] = msg.Width
		tk.wh[1] = msg.Height
		return false, cmds

	case cmd.Command:
		tk.logger.Debugf("got command: %v\n", msg)
		switch msg.Call {
		case cmd.WinFocus:
			if msg.Target == tk.winID ||
				msg.Target == "*" {
				tk.logger.Debug("got WinFocus")
				tk.Focus(m)
			}
			tk.logger.Debugf("focused: %v", tk.focused)
			return true, nil
		case cmd.WinBlur:
			if msg.Target == tk.winID ||
				msg.Target == "*" {
				tk.logger.Debug("got WinBlur")
				tk.Blur(m)
			}
			tk.logger.Debugf("focused: %v", tk.focused)
			return true, nil
		}

	}
	return false, cmds
}
