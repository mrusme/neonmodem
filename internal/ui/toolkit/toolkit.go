package toolkit

import (
	"charm.land/bubbles/v2/key"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/theme"
)

type ViewFunc func(cached bool) string

type ToolKit struct {
	winID string
	ctx   *ctx.Ctx

	mh MsgHandling

	wh      [2]int
	focused bool

	keybindings map[string]key.Binding
	closeHelp   string

	viewfunc  ViewFunc
	viewcache string

	errorDialog bool
}

func New(winID string, c *ctx.Ctx) *ToolKit {
	return &ToolKit{
		winID:       winID,
		ctx:         c,
		keybindings: make(map[string]key.Binding),
	}
}

func (tk *ToolKit) ID() string {
	return tk.winID
}

func (tk *ToolKit) Theme() *theme.Theme {
	return tk.ctx.Theme
}

func (tk *ToolKit) SetViewFunc(fn ViewFunc) {
	tk.viewfunc = fn
}

func (tk *ToolKit) InvalidateCache() {
	tk.viewcache = ""
}

func (tk *ToolKit) DefaultCaching(cached bool) string {
	if cached && !tk.focused && tk.viewcache != "" {
		return tk.viewcache
	}

	return ""
}

func (tk *ToolKit) View(cached bool) string {
	return tk.viewfunc(cached)
}

func (tk *ToolKit) Focus() {
	tk.focused = true
	tk.cache()
}

func (tk *ToolKit) Blur() {
	tk.focused = false
	tk.cache()
}

func (tk *ToolKit) cache() {
	if tk.viewfunc != nil {
		tk.viewcache = tk.viewfunc(false)
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
