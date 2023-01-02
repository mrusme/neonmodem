package posts

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/mrusme/gobbs/models/post"
	"github.com/mrusme/gobbs/models/reply"
	"github.com/mrusme/gobbs/ui/helpers"
)

func (m Model) View() string {
	return m.buildView(true)
}

func (m Model) buildView(cached bool) string {
	var view strings.Builder = strings.Builder{}

	if cached && m.WMisFocused("reply") && m.viewcache != "" {
		m.ctx.Logger.Debugln("Cached View()")

		m.textarea.SetWidth(m.viewcacheTextareaXY[2])
		m.textarea.SetHeight(m.viewcacheTextareaXY[3])

		return helpers.PlaceOverlay(
			m.viewcacheTextareaXY[0], m.viewcacheTextareaXY[1],
			m.textarea.View(), m.viewcache,
			false)
	}

	m.ctx.Logger.Debugln("View()")
	var l string = ""
	if m.WMisFocused("list") {
		l = m.ctx.Theme.PostsList.List.Focused.Render(m.list.View())
	} else {
		l = m.ctx.Theme.PostsList.List.Blurred.Render(m.list.View())
	}
	view.WriteString(lipgloss.JoinHorizontal(
		lipgloss.Top,
		l,
	))

	if m.WMisOpen("post") {
		var style lipgloss.Style
		if m.WMisFocused("post") {
			style = m.ctx.Theme.DialogBox.Titlebar.Focused
		} else {
			style = m.ctx.Theme.DialogBox.Titlebar.Blurred
		}
		titlebar := style.Align(lipgloss.Center).
			Width(m.viewport.Width + 4).
			Render("Post")

		bottombar := m.ctx.Theme.DialogBox.Bottombar.
			Width(m.viewport.Width + 4).
			Render("[#]r reply · esc close")

		ui := lipgloss.JoinVertical(
			lipgloss.Center,
			titlebar,
			viewportStyle.Render(m.viewport.View()),
			bottombar,
		)

		var tmp string
		if m.WMisFocused("post") {
			tmp = helpers.PlaceOverlay(3, 2,
				m.ctx.Theme.DialogBox.Window.Focused.Render(ui),
				view.String(), true)
		} else {
			tmp = helpers.PlaceOverlay(3, 2,
				m.ctx.Theme.DialogBox.Window.Blurred.Render(ui),
				view.String(), true)
		}

		view = strings.Builder{}
		view.WriteString(tmp)
	}

	if m.WMisOpen("reply") {
		title := "Reply"
		if m.buffer != "" && m.buffer != "0" {
			title += " to reply #" + m.buffer
		}
		titlebar := m.ctx.Theme.DialogBox.Titlebar.Focused.
			Align(lipgloss.Center).
			Width(m.viewport.Width - 2).
			Render(title)

		textareaWidth := m.viewport.Width - 2
		textareaHeight := 6
		m.textarea.SetWidth(textareaWidth)
		m.textarea.SetHeight(textareaHeight)

		bottombar := m.ctx.Theme.DialogBox.Bottombar.
			Width(m.viewport.Width - 2).
			Render("ctrl+enter reply · esc close")

		replyWindow := lipgloss.JoinVertical(
			lipgloss.Center,
			titlebar,
			m.textarea.View(),
			bottombar,
		)

		replyWindowX := 5
		replyWindowY := m.ctx.Screen[1] - 21

		tmp := helpers.PlaceOverlay(replyWindowX, replyWindowY,
			m.ctx.Theme.DialogBox.Window.Focused.Render(replyWindow),
			view.String(), true)

		m.viewcacheTextareaXY[0] = replyWindowX + 1
		m.viewcacheTextareaXY[1] = replyWindowY + 2
		m.viewcacheTextareaXY[2] = textareaWidth
		m.viewcacheTextareaXY[3] = textareaHeight

		view = strings.Builder{}
		view.WriteString(tmp)
	}

	m.viewcache = view.String()
	return m.viewcache
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

	for _, re := range *replies {
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
		idx := len(m.replyIDs) - 1

		out += fmt.Sprintf(
			"\n\n %s %s%s%s\n%s",
			m.ctx.Theme.Reply.Author.Render(
				author,
			),
			lipgloss.NewStyle().
				Foreground(m.ctx.Theme.Reply.Author.GetBackground()).
				Render(fmt.Sprintf("writes in reply to %s:", inReplyTo)),
			strings.Repeat(" ", (m.viewport.Width-len(author)-len(inReplyTo)-28)),
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
