package toolkit

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/mrusme/gobbs/ui/theme"
	"go.uber.org/zap"
)

type ViewFunc func(m interface{}, cached bool) string

type ToolKit struct {
	winID  string
	theme  *theme.Theme
	logger *zap.SugaredLogger

	mh MsgHandling

	m       interface{}
	wh      [2]int
	focused bool

	keybindings map[string]key.Binding

	viewfunc  ViewFunc
	viewcache string
}

func New(winID string, t *theme.Theme, l *zap.SugaredLogger) *ToolKit {
	tk := new(ToolKit)
	tk.winID = winID
	tk.theme = t
	tk.logger = l

	tk.mh = MsgHandling{}

	tk.wh = [2]int{0, 0}
	tk.focused = false

	tk.keybindings = make(map[string]key.Binding)

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
