package toolkit

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/mrusme/neonmodem/internal/ui/theme"
)

func (tk *ToolKit) Dialog(title string, content string, bottombar bool) string {
	return tk.dialog(tk.Theme().DialogBox, title, content, bottombar, "")
}

func (tk *ToolKit) DialogWithStatus(title string, content string, status string) string {
	return tk.dialog(tk.Theme().DialogBox, title, content, true, status)
}

func (tk *ToolKit) ErrorDialog(title string, content string) string {
	return tk.dialog(tk.Theme().ErrorDialogBox, title, content, true, "")
}

func (tk *ToolKit) dialog(
	t theme.Dialog,
	title string,
	content string,
	bottombar bool,
	status string,
) string {
	titleStyle := t.Titlebar.Blurred
	windowStyle := t.Window.Blurred
	if tk.IsFocused() {
		titleStyle = t.Titlebar.Focused
		windowStyle = t.Window.Focused
	}

	titlebar := titleStyle.Align(lipgloss.Center).
		Width(tk.ViewWidth()).
		Render(title)

	parts := []string{titlebar, content}
	if bottombar {
		text := strings.Join(tk.KeymapHelpStrings(), " · ")
		if status != "" {
			text = status
		}
		parts = append(parts, t.Bottombar.Width(tk.ViewWidth()).Render(text))
	}

	return windowStyle.Render(lipgloss.JoinVertical(lipgloss.Center, parts...))
}
