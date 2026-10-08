package postshow

import (
	"context"
	"fmt"
	"strings"

	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
)

const (
	minViewportWidth = 20
	replyIndexInset  = 28
)

func (m *Model) View() string {
	return m.tk.View(true)
}

func (m *Model) buildView(cached bool) string {
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

func postHeader(c *ctx.Ctx, p *post.Post) string {
	return fmt.Sprintf(" %s\n\n %s\n",
		c.Theme.Post.Author.Render(fmt.Sprintf("%s %s:", p.Author.Name, writesOrAsks(p.Subject))),
		c.Theme.Post.Subject.Render(p.Subject))
}

func renderLoadingPlaceholder(c *ctx.Ctx, p *post.Post, width int) string {
	loading := c.Theme.Muted.Render("Loading post, please wait (press esc to go back) ...")
	return ansi.Wrap(postHeader(c, p)+"\n "+loading+"\n", width, "")
}

func markdownRenderer(c *ctx.Ctx, width int) func(string) string {
	style := "light"
	if c.DarkBackground {
		style = "dark"
	}

	glam, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(style),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		c.Logger.Error("creating the markdown renderer failed", "error", err)
		return func(s string) string { return s }
	}

	return func(s string) string {
		out, err := glam.Render(s)
		if err != nil {
			c.Logger.Error("rendering markdown failed", "error", err)
			return s
		}
		return out
	}
}

func listsReplies(c *ctx.Ctx, p *post.Post) bool {
	if p.SysIDX < 0 || p.SysIDX >= len(c.Systems) {
		return true
	}
	return c.Systems[p.SysIDX].Capabilities().Has(system.CapListReplies)
}

func renderPost(
	loadCtx context.Context,
	c *ctx.Ctx,
	p *post.Post,
	viewportWidth int,
	imageWidth int,
) (renderedPost, bool) {
	viewportWidth = max(viewportWidth, minViewportWidth)
	render := markdownRenderer(c, viewportWidth)

	body := render(p.Body)
	if c.Config.RenderImages {
		body = c.Images.RenderInline(loadCtx, body, imageWidth)
	}
	if loadCtx.Err() != nil {
		return renderedPost{}, false
	}

	var out strings.Builder
	out.WriteString(postHeader(c, p))
	out.WriteString(body)

	rendered := renderedPost{replyIDs: []string{p.ID}, allReplies: []*reply.Reply{}}
	if listsReplies(c, p) {
		if p.ReplyPage.HasOlder() {
			out.WriteString(render("\n---\nOlder replies available, press `z` to load\n\n---\n"))
		}
		w := replyWalker{loadCtx: loadCtx, c: c, out: &out, render: render, width: viewportWidth}
		if !w.walk(&rendered, p.Author.Name, p.Replies) {
			return renderedPost{}, false
		}
	}

	rendered.content = ansi.Wrap(out.String(), viewportWidth, "")
	return rendered, true
}

type replyWalker struct {
	loadCtx context.Context
	c       *ctx.Ctx
	out     *strings.Builder
	render  func(string) string
	width   int
}

func (w replyWalker) walk(rendered *renderedPost, inReplyTo string, replies []reply.Reply) bool {
	for i := range replies {
		if w.loadCtx.Err() != nil {
			return false
		}

		re := &replies[i]
		rendered.replyIDs = append(rendered.replyIDs, re.ID)
		rendered.allReplies = append(rendered.allReplies, re)

		w.out.WriteString(w.replyHeader(re, inReplyTo, len(rendered.replyIDs)-1))
		if re.Deleted {
			w.out.WriteString("\n  DELETED\n\n")
		} else {
			w.out.WriteString(w.render(re.Body))
		}

		if !w.walk(rendered, re.Author.Name, re.Replies) {
			return false
		}
	}
	return true
}

func (w replyWalker) replyHeader(re *reply.Reply, inReplyTo string, index int) string {
	name := re.Author.Name
	if re.Deleted {
		name = "DELETED"
	}
	inReplyStyle := lipgloss.NewStyle().Foreground(w.c.Theme.Reply.Author.GetBackground())
	padding := max(w.width-lipgloss.Width(name)-lipgloss.Width(inReplyTo)-replyIndexInset, 1)

	return fmt.Sprintf("\n\n %s %s%s%s\n",
		w.c.Theme.Reply.Author.Render(name),
		inReplyStyle.Render(fmt.Sprintf("writes in reply to %s:", inReplyTo)),
		strings.Repeat(" ", padding),
		w.c.Theme.Muted.Render(fmt.Sprintf("#%d", index)))
}
