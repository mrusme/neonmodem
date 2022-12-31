package theme

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/mrusme/gobbs/config"
)

type Theme struct {
	DialogBox struct {
		Window lipgloss.Style
		Titlebar lipgloss.Style
		Bottombar lipgloss.Style
	}

	PostsList struct {
		List lipgloss.Style
		Item lipgloss.Style
	}

	Post struct {
		Author lipgloss.Style
		Subject lipgloss.Style
	}

	Reply struct {
		Author lipgloss.Style
	}
}

func New(cfg *config.Config) (*Theme) {
	t := new(Theme)
	// viewportStyle = lipgloss.NewStyle().
	// 		Margin(0, 0, 0, 0).
	// 		Padding(0, 0).
	// 		BorderTop(false).
	// 		BorderLeft(false).
	// 		BorderRight(false).
	// 		BorderBottom(false)
	//

	t.PostsList.List = lipgloss.NewStyle().
		Margin(cfg.Theme.PostsList.List.Margin...).
		Padding(cfg.Theme.PostsList.List.Padding...).
		Border(cfg.Theme.PostsList.List.Border.Border, cfg.Theme.PostsList.List.Border.Sides...).
		BorderForeground(cfg.Theme.PostsList.List.Border.Foreground).
		BorderBackground(cfg.Theme.PostsList.List.Border.Background).
		Foreground(cfg.Theme.PostsList.List.Foreground).
		Background(cfg.Theme.PostsList.List.Background)

	t.PostsList.Item = lipgloss.NewStyle().
		Margin(cfg.Theme.PostsList.Item.Margin...).
		Padding(cfg.Theme.PostsList.Item.Padding...).
		Border(cfg.Theme.PostsList.Item.Border.Border, cfg.Theme.PostsList.Item.Border.Sides...).
		BorderForeground(cfg.Theme.PostsList.Item.Border.Foreground).
		BorderBackground(cfg.Theme.PostsList.Item.Border.Background).
		Foreground(cfg.Theme.PostsList.Item.Foreground).
		Background(cfg.Theme.PostsList.Item.Background)

	t.DialogBox.Window = lipgloss.NewStyle().
		Margin(cfg.Theme.DialogBox.Window.Margin...).
		Padding(cfg.Theme.DialogBox.Window.Padding...).
		Border(cfg.Theme.DialogBox.Window.Border.Border, cfg.Theme.DialogBox.Window.Border.Sides...).
		BorderForeground(cfg.Theme.DialogBox.Window.Border.Foreground).
		BorderBackground(cfg.Theme.DialogBox.Window.Border.Background).
		Foreground(cfg.Theme.DialogBox.Window.Foreground).
		Background(cfg.Theme.DialogBox.Window.Background)

	t.DialogBox.Titlebar = lipgloss.NewStyle().
		Margin(cfg.Theme.DialogBox.Titlebar.Margin...).
		Padding(cfg.Theme.DialogBox.Titlebar.Padding...).
		Border(cfg.Theme.DialogBox.Titlebar.Border.Border, cfg.Theme.DialogBox.Titlebar.Border.Sides...).
		BorderForeground(cfg.Theme.DialogBox.Titlebar.Border.Foreground).
		BorderBackground(cfg.Theme.DialogBox.Titlebar.Border.Background).
		Foreground(cfg.Theme.DialogBox.Titlebar.Foreground).
		Background(cfg.Theme.DialogBox.Titlebar.Background)

	t.DialogBox.Bottombar = lipgloss.NewStyle().
		Margin(cfg.Theme.DialogBox.Bottombar.Margin...).
		Padding(cfg.Theme.DialogBox.Bottombar.Padding...).
		Border(cfg.Theme.DialogBox.Bottombar.Border.Border, cfg.Theme.DialogBox.Bottombar.Border.Sides...).
		BorderForeground(cfg.Theme.DialogBox.Bottombar.Border.Foreground).
		BorderBackground(cfg.Theme.DialogBox.Bottombar.Border.Background).
		Foreground(cfg.Theme.DialogBox.Bottombar.Foreground).
		Background(cfg.Theme.DialogBox.Bottombar.Background)

	t.Post.Author = lipgloss.NewStyle().
		Margin(cfg.Theme.Post.Author.Margin...).
		Padding(cfg.Theme.Post.Author.Padding...).
		Border(cfg.Theme.Post.Author.Border.Border, cfg.Theme.Post.Author.Border.Sides...).
		BorderForeground(cfg.Theme.Post.Author.Border.Foreground).
		BorderBackground(cfg.Theme.Post.Author.Border.Background).
		Foreground(cfg.Theme.Post.Author.Foreground).
		Background(cfg.Theme.Post.Author.Background)

	t.Post.Subject = lipgloss.NewStyle().
		Margin(cfg.Theme.Post.Subject.Margin...).
		Padding(cfg.Theme.Post.Subject.Padding...).
		Border(cfg.Theme.Post.Subject.Border.Border, cfg.Theme.Post.Subject.Border.Sides...).
		BorderForeground(cfg.Theme.Post.Subject.Border.Foreground).
		BorderBackground(cfg.Theme.Post.Subject.Border.Background).
		Foreground(cfg.Theme.Post.Subject.Foreground).
		Background(cfg.Theme.Post.Subject.Background)

	t.Reply.Author = lipgloss.NewStyle().
		Margin(cfg.Theme.Reply.Author.Margin...).
		Padding(cfg.Theme.Reply.Author.Padding...).
		Border(cfg.Theme.Reply.Author.Border.Border, cfg.Theme.Reply.Author.Border.Sides...).
		BorderForeground(cfg.Theme.Reply.Author.Border.Foreground).
		BorderBackground(cfg.Theme.Reply.Author.Border.Background).
		Foreground(cfg.Theme.Reply.Author.Foreground).
		Background(cfg.Theme.Reply.Author.Background)

	return t
}
