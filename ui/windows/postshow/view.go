package postshow

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/mrusme/gobbs/models/post"
	"github.com/mrusme/gobbs/models/reply"
)

func (m Model) View() string {
	return m.tk.View(&m, true)
}

func buildView(mi interface{}, cached bool) string {
	var m *Model = mi.(*Model)

	if vcache := m.tk.DefaultCaching(cached); vcache != "" {
		m.ctx.Logger.Debugln("Cached View()")
		return vcache
	}
	m.ctx.Logger.Debugln("View()")
	m.ctx.Logger.Debugf("IsFocused: %v\n", m.tk.IsFocused())

	return m.tk.Dialog(
		"Post",
		viewportStyle.Render(m.viewport.View()),
		true,
	)
}

func (m *Model) renderViewport(p *post.Post) string {
	var out string = ""

	var err error
	m.glam, err = glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(m.viewport.Width),
	)
	if err != nil {
		m.ctx.Logger.Error(err)
		m.glam = nil
	}

	adj := "writes"
	if p.Subject[len(p.Subject)-1:] == "?" {
		adj = "asks"
	}

	body, err := m.glam.Render(p.Body)
	if err != nil {
		m.ctx.Logger.Error(err)
		body = p.Body
	}
	out += fmt.Sprintf(
		" %s\n\n %s\n%s",
		m.ctx.Theme.Post.Author.Render(
			fmt.Sprintf("%s %s:", p.Author.Name, adj),
		),
		m.ctx.Theme.Post.Subject.Render(p.Subject),
		body,
	)

	m.replyIDs = []string{p.ID}
	m.activePost = p
	out += m.renderReplies(0, p.Author.Name, &p.Replies)

	return out
}

func (m *Model) renderReplies(
	level int,
	inReplyTo string,
	replies *[]reply.Reply,
) string {
	var out string = ""

	if replies == nil {
		return ""
	}

	for ri, re := range *replies {
		var err error = nil
		var body string = ""
		var author string = ""

		if re.Deleted {
			body = "\n  DELETED\n\n"
			author = "DELETED"
		} else {
			body, err = m.glam.Render(re.Body)
			if err != nil {
				m.ctx.Logger.Error(err)
				body = re.Body
			}

			author = re.Author.Name
		}

		m.replyIDs = append(m.replyIDs, re.ID)
		m.allReplies = append(m.allReplies, &(*replies)[ri])
		idx := len(m.replyIDs) - 1

		replyIdPadding := (m.viewport.Width - len(author) - len(inReplyTo) - 28)
		if replyIdPadding < 0 {
			replyIdPadding = 0
		}

		out += fmt.Sprintf(
			"\n\n %s %s%s%s\n%s",
			m.ctx.Theme.Reply.Author.Render(
				author,
			),
			lipgloss.NewStyle().
				Foreground(m.ctx.Theme.Reply.Author.GetBackground()).
				Render(fmt.Sprintf("writes in reply to %s:", inReplyTo)),
			strings.Repeat(" ", replyIdPadding),
			lipgloss.NewStyle().
				Foreground(lipgloss.Color("#777777")).
				Render(fmt.Sprintf("#%d", idx)),
			body,
		)

		idx++
		out += m.renderReplies(level+1, re.Author.Name, &re.Replies)
	}

	return out
}
