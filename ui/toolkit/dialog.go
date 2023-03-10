package toolkit

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (tk *ToolKit) Dialog(title string, content string, bbar bool) string {
	var view strings.Builder = strings.Builder{}

	var style lipgloss.Style
	if tk.IsFocused() {
		style = tk.theme.DialogBox.Titlebar.Focused
	} else {
		style = tk.theme.DialogBox.Titlebar.Blurred
	}
	titlebar := style.Align(lipgloss.Center).
		Width(tk.ViewWidth()).
		Render(title)

	bindings := tk.KeymapHelpStrings()

	var ui string
	if bbar {
		bottombar := tk.theme.DialogBox.Bottombar.
			Width(tk.ViewWidth()).
			Render(strings.Join(bindings, " · "))

		ui = lipgloss.JoinVertical(
			lipgloss.Center,
			titlebar,
			content,
			bottombar,
		)
	} else {
		ui = lipgloss.JoinVertical(
			lipgloss.Center,
			titlebar,
			content,
		)
	}

	var tmp string
	if tk.IsFocused() {
		tmp = tk.theme.DialogBox.Window.Focused.Render(ui)
	} else {
		tmp = tk.theme.DialogBox.Window.Blurred.Render(ui)
	}

	view.WriteString(tmp)

	return view.String()
}

func (tk *ToolKit) ErrorDialog(title string, content string) string {
	var view strings.Builder = strings.Builder{}

	var style lipgloss.Style
	if tk.IsFocused() {
		style = tk.theme.ErrorDialogBox.Titlebar.Focused
	} else {
		style = tk.theme.ErrorDialogBox.Titlebar.Blurred
	}
	titlebar := style.Align(lipgloss.Center).
		Width(tk.ViewWidth()).
		Render(title)

	bindings := tk.KeymapHelpStrings()

	bottombar := tk.theme.ErrorDialogBox.Bottombar.
		Width(tk.ViewWidth()).
		Render(strings.Join(bindings, " · "))

	ui := lipgloss.JoinVertical(
		lipgloss.Center,
		titlebar,
		content,
		bottombar,
	)

	var tmp string
	if tk.IsFocused() {
		tmp = tk.theme.ErrorDialogBox.Window.Focused.Render(ui)
	} else {
		tmp = tk.theme.ErrorDialogBox.Window.Blurred.Render(ui)
	}

	view.WriteString(tmp)

	return view.String()
}
