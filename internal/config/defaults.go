package config

import (
	"path/filepath"

	"github.com/mrusme/neonmodem/internal/system"
)

var (
	NormalBorder = Border{
		Top: "─", Bottom: "─", Left: "│", Right: "│",
		TopLeft: "┌", TopRight: "┐", BottomLeft: "└", BottomRight: "┘",
		MiddleLeft: "├", MiddleRight: "┤", Middle: "┼", MiddleTop: "┬", MiddleBottom: "┴",
	}
	ThickBorder = Border{
		Top: "━", Bottom: "━", Left: "┃", Right: "┃",
		TopLeft: "┏", TopRight: "┓", BottomLeft: "┗", BottomRight: "┛",
		MiddleLeft: "┣", MiddleRight: "┫", Middle: "╋", MiddleTop: "┳", MiddleBottom: "┻",
	}
	DoubleBorder = Border{
		Top: "═", Bottom: "═", Left: "║", Right: "║",
		TopLeft: "╔", TopRight: "╗", BottomLeft: "╚", BottomRight: "╝",
		MiddleLeft: "╠", MiddleRight: "╣", Middle: "╬", MiddleTop: "╦", MiddleBottom: "╩",
	}
	HiddenBorder = Border{
		Top: " ", Bottom: " ", Left: " ", Right: " ",
		TopLeft: " ", TopRight: " ", BottomLeft: " ", BottomRight: " ",
		MiddleLeft: " ", MiddleRight: " ", Middle: " ", MiddleTop: " ", MiddleBottom: " ",
	}
)

func same(c string) Adaptive {
	return Adaptive{Light: c, Dark: c}
}

func Defaults(cacheDir string) Config {
	cfg := Config{
		Debug:         false,
		Log:           filepath.Join(cacheDir, "neonmodem.log"),
		Proxy:         "",
		Browser:       "",
		Sort:          string(system.OrderNew),
		RenderShadows: true,
		RenderImages:  true,
		RenderSplash:  true,
		RenderBanner:  true,
	}

	t := &cfg.Theme

	t.Header.Selector = ThemeItem{
		Margin:  []int{0, 0, 0, 0},
		Padding: []int{0, 1, 0, 1},
		Border: BorderConfig{
			Border:     NormalBorder,
			Sides:      []bool{true, true, true, true},
			Foreground: same("#6ca1d0"),
		},
		Foreground: same("#6ca1d0"),
	}
	t.Header.Spinner = ThemeItem{
		Foreground: same("#FF5FAF"),
	}

	t.DialogBox = dialogTheme(same("#82e4dc"))
	t.ErrorDialogBox = dialogTheme(same("#dc143c"))

	t.PostsList = listTheme(DoubleBorder)
	t.PopupList = listTheme(HiddenBorder)

	t.Post.Author = ThemeItem{
		Padding:    []int{0, 1, 0, 1},
		Foreground: same("#f119a0"),
	}
	t.Post.Subject = ThemeItem{
		Padding:    []int{0, 1, 0, 1},
		Foreground: same("#FFFFFF"),
		Background: same("#f119a0"),
	}
	t.Reply.Author = ThemeItem{
		Padding:    []int{0, 1, 0, 1},
		Foreground: same("#000000"),
		Background: same("#ffd500"),
	}

	return cfg
}

func dialogTheme(accent Adaptive) DialogTheme {
	blurred := Adaptive{Light: "#cccccc", Dark: "#333333"}

	window := func(border Adaptive) ThemeItem {
		return ThemeItem{
			Margin:  []int{0, 0, 0, 0},
			Padding: []int{0, 0, 0, 0},
			Border: BorderConfig{
				Border:     ThickBorder,
				Sides:      []bool{false, true, true, true},
				Foreground: border,
			},
		}
	}
	titlebar := func(background Adaptive) ThemeItem {
		return ThemeItem{
			Margin:     []int{0, 0, 1, 0},
			Padding:    []int{0, 1, 0, 1},
			Foreground: Adaptive{Light: "#ffffff", Dark: "#000000"},
			Background: background,
		}
	}

	return DialogTheme{
		Window: FocusedBlurred{
			Focused: window(accent),
			Blurred: window(blurred),
		},
		Titlebar: FocusedBlurred{
			Focused: titlebar(accent),
			Blurred: titlebar(blurred),
		},
		Bottombar: ThemeItem{
			Margin:     []int{1, 0, 0, 0},
			Padding:    []int{0, 1, 0, 1},
			Foreground: Adaptive{Light: "#aaaaaa", Dark: "#999999"},
		},
	}
}

func listTheme(border Border) ListTheme {
	list := func(frame Adaptive) ThemeItem {
		return ThemeItem{
			Margin:  []int{0, 0, 0, 0},
			Padding: []int{1, 1, 1, 1},
			Border: BorderConfig{
				Border:     border,
				Sides:      []bool{true, true, true, true},
				Foreground: frame,
			},
		}
	}
	selected := func(foreground Adaptive) ThemeItem {
		return ThemeItem{
			Padding: []int{0, 0, 0, 1},
			Border: BorderConfig{
				Border:     NormalBorder,
				Sides:      []bool{false, false, false, true},
				Foreground: same("#ffd500"),
			},
			Foreground: foreground,
		}
	}

	return ListTheme{
		List: FocusedBlurred{
			Focused: list(same("#82e4dc")),
			Blurred: list(Adaptive{Light: "#cccccc", Dark: "#333333"}),
		},
		Item: FocusedBlurredSelected{
			Focused: ThemeItem{
				Padding:    []int{0, 0, 0, 2},
				Foreground: Adaptive{Light: "#333333", Dark: "#cccccc"},
			},
			Blurred: ThemeItem{
				Padding:    []int{0, 0, 0, 2},
				Foreground: Adaptive{Light: "#cccccc", Dark: "#333333"},
			},
			Selected: selected(same("#f119a0")),
		},
		ItemDetail: FocusedBlurredSelected{
			Focused: ThemeItem{
				Padding:    []int{0, 0, 0, 2},
				Foreground: Adaptive{Light: "#666666", Dark: "#4d4d4d"},
			},
			Blurred: ThemeItem{
				Padding:    []int{0, 0, 0, 2},
				Foreground: Adaptive{Light: "#666666", Dark: "#4d4d4d"},
			},
			Selected: selected(Adaptive{Light: "#000000", Dark: "#FFFFFF"}),
		},
	}
}
