package postshow

import (
	"fmt"
	"strings"

	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/images"
)

func (m *Model) View() string {
	return m.tk.View(m, true)
}

func buildView(mi interface{}, cached bool) string {
	m := mi.(*Model)

	if vcache := m.tk.DefaultCaching(cached); vcache != "" {
		return vcache
	}

	title := "Post"
	if m.activePost != nil && m.activePost.SysIDX >= 0 && m.activePost.SysIDX < len(m.ctx.Systems) {
		title = "Post on " + m.ctx.Systems[m.activePost.SysIDX].Title()
	}

	if m.loading {
		status := m.ctx.Theme.Muted.Render("Loading post, press esc to go back")
		return m.tk.DialogWithStatus(title, m.viewport.View(), status)
	}

	return m.tk.Dialog(title, m.viewport.View())
}

type renderedPost struct {
	content    string
	replyIDs   []string
	allReplies []*reply.Reply
}

func writesOrAsks(subject string) string {
	if strings.HasSuffix(subject, "?") {
		return "asks"
	}
	return "writes"
}

func renderLoadingPlaceholder(c *ctx.Ctx, p *post.Post, width int) string {
	return ansi.Wrap(fmt.Sprintf(
		" %s\n\n %s\n\n %s\n",
		c.Theme.Post.Author.Render(
			fmt.Sprintf("%s %s:", p.Author.Name, writesOrAsks(p.Subject)),
		),
		c.Theme.Post.Subject.Render(p.Subject),
		c.Theme.Muted.Render("Loading post, please wait (press esc to go back) ..."),
	), width, "")
}

func renderPost(
	c *ctx.Ctx,
	p *post.Post,
	viewportWidth int,
	imageWidth int,
	isCurrent func() bool,
) (rendered renderedPost, ok bool) {
	style := "light"
	if c.DarkBackground {
		style = "dark"
	}
	if viewportWidth < 20 {
		viewportWidth = 20
	}

	glam, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(style),
		glamour.WithWordWrap(viewportWidth),
	)
	if err != nil {
		c.Logger.Error("creating the markdown renderer failed", "error", err)
		glam = nil
	}

	render := func(s string) string {
		if glam == nil {
			return s
		}
		out, rerr := glam.Render(s)
		if rerr != nil {
			c.Logger.Error("rendering markdown failed", "error", rerr)
			return s
		}
		return out
	}

	body := render(p.Body)
	if c.Config.RenderImages {
		body = images.RenderInline(c.Logger, body, imageWidth)
	}

	if !isCurrent() {
		return renderedPost{}, false
	}

	var out strings.Builder
	out.WriteString(fmt.Sprintf(
		" %s\n\n %s\n%s",
		c.Theme.Post.Author.Render(
			fmt.Sprintf("%s %s:", p.Author.Name, writesOrAsks(p.Subject)),
		),
		c.Theme.Post.Subject.Render(p.Subject),
		body,
	))

	rendered.replyIDs = []string{p.ID}
	rendered.allReplies = []*reply.Reply{}

	if p.SysIDX >= 0 && p.SysIDX < len(c.Systems) {
		if !c.Systems[p.SysIDX].Capabilities().Has(system.CapListReplies) {
			rendered.content = ansi.Wrap(out.String(), viewportWidth, "")
			return rendered, true
		}
	}

	if p.ReplyPage.HasOlder() {
		out.WriteString(render(
			"\n---\nOlder replies available, press `z` to load\n\n---\n"))
	}

	indexStyle := c.Theme.Muted
	inReplyStyle := lipgloss.NewStyle().Foreground(c.Theme.Reply.Author.GetBackground())

	var walk func(inReplyTo string, replies []reply.Reply) bool
	walk = func(inReplyTo string, replies []reply.Reply) bool {
		for ri := range replies {
			if !isCurrent() {
				return false
			}

			re := &replies[ri]

			var body string
			var authorName string
			if re.Deleted {
				body = "\n  DELETED\n\n"
				authorName = "DELETED"
			} else {
				body = render(re.Body)
				authorName = re.Author.Name
			}

			rendered.replyIDs = append(rendered.replyIDs, re.ID)
			rendered.allReplies = append(rendered.allReplies, re)
			idx := len(rendered.replyIDs) - 1

			replyIdPadding := viewportWidth - lipgloss.Width(authorName) - lipgloss.Width(inReplyTo) - 28
			if replyIdPadding < 1 {
				replyIdPadding = 1
			}

			out.WriteString(fmt.Sprintf(
				"\n\n %s %s%s%s\n%s",
				c.Theme.Reply.Author.Render(authorName),
				inReplyStyle.Render(fmt.Sprintf("writes in reply to %s:", inReplyTo)),
				strings.Repeat(" ", replyIdPadding),
				indexStyle.Render(fmt.Sprintf("#%d", idx)),
				body,
			))

			if !walk(re.Author.Name, re.Replies) {
				return false
			}
		}

		return true
	}

	if !walk(p.Author.Name, p.Replies) {
		return renderedPost{}, false
	}

	rendered.content = ansi.Wrap(out.String(), viewportWidth, "")
	return rendered, true
}
