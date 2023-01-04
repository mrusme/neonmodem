package toolkit

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (tk *ToolKit) Dialog(title string, content string) string {
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

	var bindings []string
	for _, binding := range tk.keybindings {
		var tmp string = ""
		tmp = binding.Help().Key + " " + binding.Help().Desc
		bindings = append(bindings, tmp)
	}
	bindings = append(bindings, "esc close")

	bottombar := tk.theme.DialogBox.Bottombar.
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
		tmp = tk.theme.DialogBox.Window.Focused.Render(ui)
	} else {
		tmp = tk.theme.DialogBox.Window.Blurred.Render(ui)
	}

	view.WriteString(tmp)

	return view.String()
}
