package postcreate

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
)

func (m *Model) View() string {
	return m.tk.View(m, true)
}

func buildView(mi interface{}, cached bool) string {
	m := mi.(*Model)

	if vcache := m.tk.DefaultCaching(cached); vcache != "" {
		return vcache
	}

	title := ""
	switch m.action {
	case msgs.ComposeReply:
		title = "Reply"
		if m.index != 0 && m.parent != nil {
			title = fmt.Sprintf("Reply to #%d by %s", m.index, m.parent.Author.Name)
		}
	case msgs.ComposePost:
		title = fmt.Sprintf("New post in %s", m.post.Forum.Name)
		if m.post.SysIDX >= 0 && m.post.SysIDX < len(m.ctx.Systems) {
			title += " on " + m.ctx.Systems[m.post.SysIDX].Title()
		}
	}

	var body string
	if m.action == msgs.ComposePost {
		body = lipgloss.JoinVertical(
			lipgloss.Left,
			m.textinput.View(),
			"",
			m.textarea.View(),
		)
	} else {
		body = m.textarea.View()
	}
	body = lipgloss.NewStyle().Padding(0, 1).Render(body)

	if m.submitting {
		return m.tk.DialogWithStatus(title, body, m.ctx.Theme.Muted.Render("Posting, please wait"))
	}

	return m.tk.Dialog(title, body, true)
}
