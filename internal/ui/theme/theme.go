package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/mrusme/neonmodem/internal/config"
)

type FocusedBlurred struct {
	Focused lipgloss.Style
	Blurred lipgloss.Style
}

type FocusedBlurredSelected struct {
	Focused  lipgloss.Style
	Blurred  lipgloss.Style
	Selected lipgloss.Style
}

type Dialog struct {
	Window    FocusedBlurred
	Titlebar  FocusedBlurred
	Bottombar lipgloss.Style
}

type List struct {
	List       FocusedBlurred
	Item       FocusedBlurredSelected
	ItemDetail FocusedBlurredSelected
}

type Theme struct {
	Header struct {
		Selector lipgloss.Style
		Spinner  lipgloss.Style
	}

	DialogBox      Dialog
	ErrorDialogBox Dialog

	PostsList List
	PopupList List

	Post struct {
		Author  lipgloss.Style
		Subject lipgloss.Style
	}

	Reply struct {
		Author lipgloss.Style
	}

	Muted  lipgloss.Style
	Notice lipgloss.Style
	Alert  lipgloss.Style
}

func New(cfg *config.Theme, dark bool) *Theme {
	b := builder{dark: dark}
	t := &Theme{}

	t.Header.Selector = b.style(cfg.Header.Selector)
	t.Header.Spinner = b.style(cfg.Header.Spinner)

	t.DialogBox = b.dialog(cfg.DialogBox)
	t.ErrorDialogBox = b.dialog(cfg.ErrorDialogBox)

	t.PostsList = b.list(cfg.PostsList)
	t.PopupList = b.list(cfg.PopupList)

	t.Post.Author = b.style(cfg.Post.Author)
	t.Post.Subject = b.style(cfg.Post.Subject)
	t.Reply.Author = b.style(cfg.Reply.Author)

	t.Muted = lipgloss.NewStyle().Foreground(lipgloss.Color("#777777"))
	t.Notice = lipgloss.NewStyle().Foreground(t.DialogBox.Titlebar.Focused.GetBackground())
	t.Alert = lipgloss.NewStyle().Foreground(t.ErrorDialogBox.Titlebar.Focused.GetBackground())

	return t
}

type builder struct {
	dark bool
}

func (b builder) color(a config.Adaptive) color.Color {
	if a.IsZero() {
		return lipgloss.NoColor{}
	}
	if b.dark {
		if a.Dark != "" {
			return lipgloss.Color(a.Dark)
		}
		return lipgloss.Color(a.Light)
	}
	if a.Light != "" {
		return lipgloss.Color(a.Light)
	}
	return lipgloss.Color(a.Dark)
}

func (b builder) border(cfg config.Border) lipgloss.Border {
	return lipgloss.Border{
		Top:          cfg.Top,
		Bottom:       cfg.Bottom,
		Left:         cfg.Left,
		Right:        cfg.Right,
		TopLeft:      cfg.TopLeft,
		TopRight:     cfg.TopRight,
		BottomLeft:   cfg.BottomLeft,
		BottomRight:  cfg.BottomRight,
		MiddleLeft:   cfg.MiddleLeft,
		MiddleRight:  cfg.MiddleRight,
		Middle:       cfg.Middle,
		MiddleTop:    cfg.MiddleTop,
		MiddleBottom: cfg.MiddleBottom,
	}
}

func (b builder) style(item config.ThemeItem) lipgloss.Style {
	s := lipgloss.NewStyle().
		Margin(item.Margin...).
		Padding(item.Padding...).
		Foreground(b.color(item.Foreground)).
		Background(b.color(item.Background))

	if !item.Border.Border.IsZero() {
		s = s.Border(b.border(item.Border.Border), item.Border.Sides...).
			BorderForeground(b.color(item.Border.Foreground)).
			BorderBackground(b.color(item.Border.Background))
	}

	return s
}

func (b builder) focusedBlurred(cfg config.FocusedBlurred) FocusedBlurred {
	return FocusedBlurred{
		Focused: b.style(cfg.Focused),
		Blurred: b.style(cfg.Blurred),
	}
}

func (b builder) focusedBlurredSelected(cfg config.FocusedBlurredSelected) FocusedBlurredSelected {
	return FocusedBlurredSelected{
		Focused:  b.style(cfg.Focused),
		Blurred:  b.style(cfg.Blurred),
		Selected: b.style(cfg.Selected),
	}
}

func (b builder) dialog(cfg config.DialogTheme) Dialog {
	return Dialog{
		Window:    b.focusedBlurred(cfg.Window),
		Titlebar:  b.focusedBlurred(cfg.Titlebar),
		Bottombar: b.style(cfg.Bottombar),
	}
}

func (b builder) list(cfg config.ListTheme) List {
	return List{
		List:       b.focusedBlurred(cfg.List),
		Item:       b.focusedBlurredSelected(cfg.Item),
		ItemDetail: b.focusedBlurredSelected(cfg.ItemDetail),
	}
}
