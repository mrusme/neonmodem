package toolkit

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mrusme/neonmodem/internal/ui/theme"
)

func (tk *ToolKit) SetErrorDialog(isError bool) {
	tk.errorDialog = isError
}

func (tk *ToolKit) dialogTheme() theme.Dialog {
	if tk.errorDialog {
		return tk.Theme().ErrorDialogBox
	}
	return tk.Theme().DialogBox
}

func (tk *ToolKit) InnerWidth() int {
	t := tk.dialogTheme()
	frame := max(t.Window.Focused.GetHorizontalFrameSize(), t.Window.Blurred.GetHorizontalFrameSize())
	return max(tk.ViewWidth()-frame, 1)
}

func (tk *ToolKit) InnerHeight() int {
	t := tk.dialogTheme()
	frame := max(t.Window.Focused.GetVerticalFrameSize(), t.Window.Blurred.GetVerticalFrameSize())
	title := max(t.Titlebar.Focused.GetVerticalFrameSize(), t.Titlebar.Blurred.GetVerticalFrameSize()) + 1
	return max(tk.ViewHeight()-frame-title-tk.barHeight(t), 1)
}

func (tk *ToolKit) barHeight(t theme.Dialog) int {
	return lipgloss.Height(tk.bar(t, tk.helpText()))
}

func (tk *ToolKit) helpText() string {
	return strings.Join(tk.KeymapHelpStrings(), " · ")
}

func (tk *ToolKit) bar(t theme.Dialog, text string) string {
	style := t.Bottombar
	return style.Width(tk.InnerWidth() - style.GetHorizontalMargins()).Render(text)
}

func (tk *ToolKit) Dialog(title string, content string) string {
	return tk.dialog(title, content, "")
}

func (tk *ToolKit) DialogWithStatus(title string, content string, status string) string {
	return tk.dialog(title, content, status)
}

func (tk *ToolKit) dialog(title string, content string, status string) string {
	t := tk.dialogTheme()
	titleStyle := t.Titlebar.Blurred
	windowStyle := t.Window.Blurred
	if tk.IsFocused() {
		titleStyle = t.Titlebar.Focused
		windowStyle = t.Window.Focused
	}

	width := tk.InnerWidth()
	titleWidth := width - titleStyle.GetHorizontalMargins()
	titleText := ansi.Truncate(title,
		titleWidth-titleStyle.GetHorizontalPadding()-titleStyle.GetHorizontalBorderSize(), "…")
	titlebar := titleStyle.Align(lipgloss.Center).Width(titleWidth).Render(titleText)

	bar := tk.bar(t, tk.helpText())
	if status != "" {
		bar = fit(tk.bar(t, status), width, lipgloss.Height(bar))
	}

	return windowStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		titlebar,
		fit(content, width, tk.InnerHeight()),
		bar,
	))
}

func fit(block string, width int, height int) string {
	lines := strings.Split(block, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}

	for i, line := range lines {
		line = ansi.Truncate(line, width, "")
		if pad := width - ansi.StringWidth(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		lines[i] = line
	}

	return strings.Join(lines, "\n")
}
