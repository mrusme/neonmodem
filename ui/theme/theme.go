package theme

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/mrusme/neonmodem/config"
)

type Theme struct {
	Header struct {
		Selector lipgloss.Style
		Spinner lipgloss.Style
	}

	DialogBox struct {
		Window struct {
			Focused lipgloss.Style
			Blurred lipgloss.Style
		}
		Titlebar struct {
			Focused lipgloss.Style
			Blurred lipgloss.Style
		}
		Bottombar lipgloss.Style
	}

	ErrorDialogBox struct {
		Window struct {
			Focused lipgloss.Style
			Blurred lipgloss.Style
		}
		Titlebar struct {
			Focused lipgloss.Style
			Blurred lipgloss.Style
		}
		Bottombar lipgloss.Style
	}

	PopupList struct {
		List struct {
			Focused lipgloss.Style
			Blurred lipgloss.Style
		}
		Item struct {
			Focused  lipgloss.Style
			Blurred  lipgloss.Style
			Selected lipgloss.Style
		}
		ItemDetail struct {
			Focused  lipgloss.Style
			Blurred  lipgloss.Style
			Selected lipgloss.Style
		}
	}

	PostsList struct {
		List struct {
			Focused lipgloss.Style
			Blurred lipgloss.Style
		}
		Item struct {
			Focused  lipgloss.Style
			Blurred  lipgloss.Style
			Selected lipgloss.Style
		}
		ItemDetail struct {
			Focused  lipgloss.Style
			Blurred  lipgloss.Style
			Selected lipgloss.Style
		}
	}

	Post struct {
		Author  lipgloss.Style
		Subject lipgloss.Style
	}

	Reply struct {
		Author lipgloss.Style
	}
}

func New(cfg *config.Config) *Theme {
	t := new(Theme)

	t.Header.Selector =
		t.fromConfig(&cfg.Theme.Header.Selector)
	t.Header.Spinner =
		t.fromConfig(&cfg.Theme.Header.Spinner)

	t.DialogBox.Window.Focused =
		t.fromConfig(&cfg.Theme.DialogBox.Window.Focused)
	t.DialogBox.Window.Blurred =
		t.fromConfig(&cfg.Theme.DialogBox.Window.Blurred)
	t.DialogBox.Titlebar.Focused =
		t.fromConfig(&cfg.Theme.DialogBox.Titlebar.Focused)
	t.DialogBox.Titlebar.Blurred =
		t.fromConfig(&cfg.Theme.DialogBox.Titlebar.Blurred)
	t.DialogBox.Bottombar =
		t.fromConfig(&cfg.Theme.DialogBox.Bottombar)

	t.ErrorDialogBox.Window.Focused =
		t.fromConfig(&cfg.Theme.ErrorDialogBox.Window.Focused)
	t.ErrorDialogBox.Window.Blurred =
		t.fromConfig(&cfg.Theme.ErrorDialogBox.Window.Blurred)
	t.ErrorDialogBox.Titlebar.Focused =
		t.fromConfig(&cfg.Theme.ErrorDialogBox.Titlebar.Focused)
	t.ErrorDialogBox.Titlebar.Blurred =
		t.fromConfig(&cfg.Theme.ErrorDialogBox.Titlebar.Blurred)
	t.ErrorDialogBox.Bottombar =
		t.fromConfig(&cfg.Theme.ErrorDialogBox.Bottombar)

	t.PostsList.List.Focused =
		t.fromConfig(&cfg.Theme.PostsList.List.Focused)
	t.PostsList.List.Blurred =
		t.fromConfig(&cfg.Theme.PostsList.List.Blurred)
	t.PostsList.Item.Focused =
		t.fromConfig(&cfg.Theme.PostsList.Item.Focused)
	t.PostsList.Item.Blurred =
		t.fromConfig(&cfg.Theme.PostsList.Item.Blurred)
	t.PostsList.Item.Selected =
		t.fromConfig(&cfg.Theme.PostsList.Item.Selected)
	t.PostsList.ItemDetail.Focused =
		t.fromConfig(&cfg.Theme.PostsList.ItemDetail.Focused)
	t.PostsList.ItemDetail.Blurred =
		t.fromConfig(&cfg.Theme.PostsList.ItemDetail.Blurred)
	t.PostsList.ItemDetail.Selected =
		t.fromConfig(&cfg.Theme.PostsList.ItemDetail.Selected)

	t.PopupList.List.Focused =
		t.fromConfig(&cfg.Theme.PopupList.List.Focused)
	t.PopupList.List.Blurred =
		t.fromConfig(&cfg.Theme.PopupList.List.Blurred)
	t.PopupList.Item.Focused =
		t.fromConfig(&cfg.Theme.PopupList.Item.Focused)
	t.PopupList.Item.Blurred =
		t.fromConfig(&cfg.Theme.PopupList.Item.Blurred)
	t.PopupList.Item.Selected =
		t.fromConfig(&cfg.Theme.PopupList.Item.Selected)
	t.PopupList.ItemDetail.Focused =
		t.fromConfig(&cfg.Theme.PopupList.ItemDetail.Focused)
	t.PopupList.ItemDetail.Blurred =
		t.fromConfig(&cfg.Theme.PopupList.ItemDetail.Blurred)
	t.PopupList.ItemDetail.Selected =
		t.fromConfig(&cfg.Theme.PopupList.ItemDetail.Selected)

	t.Post.Author =
		t.fromConfig(&cfg.Theme.Post.Author)
	t.Post.Subject =
		t.fromConfig(&cfg.Theme.Post.Subject)
	t.Reply.Author =
		t.fromConfig(&cfg.Theme.Reply.Author)
	return t
}

func (t *Theme) fromConfig(itemCfg *config.ThemeItemConfig) lipgloss.Style {
	return lipgloss.NewStyle().
		Margin(itemCfg.Margin...).
		Padding(itemCfg.Padding...).
		Border(itemCfg.Border.Border, itemCfg.Border.Sides...).
		BorderForeground(itemCfg.Border.Foreground).
		BorderBackground(itemCfg.Border.Background).
		Foreground(itemCfg.Foreground).
		Background(itemCfg.Background)
}
