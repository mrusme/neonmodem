package postshow

import (
	"context"
	"fmt"
	"net/http"
	"slices"
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

const (
	minViewportWidth = 20
	replyIndexInset  = 28
	imageIndent      = "  "
	captionLines     = 1
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

type unit struct {
	head  string
	body  string
	plain bool
	tail  string
	refs  []images.Ref
	text  string
}

type renderedPost struct {
	content    string
	replyIDs   []string
	allReplies []*reply.Reply
	units      []unit
	lines      []int
	width      int
}

func (r *renderedPost) assemble() {
	var b strings.Builder
	r.lines = make([]int, len(r.units))
	line := 0
	for i, u := range r.units {
		r.lines[i] = line
		b.WriteString(u.text)
		line += strings.Count(u.text, "\n")
	}
	r.content = b.String()
}

func (r renderedPost) hasImages() bool {
	return slices.ContainsFunc(r.units, func(u unit) bool { return len(u.refs) > 0 })
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
) (renderedPost, bool) {
	viewportWidth = max(viewportWidth, minViewportWidth)
	render := markdownRenderer(c, viewportWidth)

	rendered := renderedPost{
		replyIDs:   []string{p.ID},
		allReplies: []*reply.Reply{},
		units:      []unit{{head: postHeader(c, p), body: p.Body}},
		width:      viewportWidth,
	}
	if listsReplies(c, p) {
		if p.ReplyPage.HasOlder() {
			rendered.units[0].tail = render("\n---\nOlder replies available, press `z` to load\n\n---\n")
		}
		w := replyWalker{loadCtx: loadCtx, c: c, width: viewportWidth}
		if !w.walk(&rendered, p.Author.Name, p.Replies) {
			return renderedPost{}, false
		}
	}

	for i := range rendered.units {
		if loadCtx.Err() != nil {
			return renderedPost{}, false
		}
		u := &rendered.units[i]
		if c.Config.RenderImages && !u.plain {
			u.refs = images.Find(u.body)
		}
		body := u.body
		if !u.plain {
			body = render(u.body)
		}
		u.text = ansi.Wrap(u.head+body+u.tail, viewportWidth, "")
	}

	rendered.assemble()
	return rendered, true
}

type replyWalker struct {
	loadCtx context.Context
	c       *ctx.Ctx
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

		u := unit{head: w.replyHeader(re, inReplyTo, len(rendered.replyIDs)-1), body: re.Body}
		if re.Deleted {
			u.body, u.plain = "\n  DELETED\n\n", true
		}
		rendered.units = append(rendered.units, u)

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

func renderImages(
	loadCtx context.Context,
	c *ctx.Ctx,
	rendered renderedPost,
	height int,
	authorize func(*http.Request),
) (renderedPost, bool) {
	var reqs []images.Request
	for _, u := range rendered.units {
		for _, ref := range u.refs {
			reqs = append(reqs, images.Request{
				URL:       ref.URL,
				Width:     rendered.width - len(imageIndent),
				Height:    height,
				Authorize: authorize,
			})
		}
	}

	results := c.Images.Load(loadCtx, reqs)
	if loadCtx.Err() != nil {
		return renderedPost{}, false
	}

	render := markdownRenderer(c, rendered.width)
	units := slices.Clone(rendered.units)
	next := 0
	for i := range units {
		u := &units[i]
		loaded := results[next : next+len(u.refs)]
		next += len(u.refs)
		if !slices.ContainsFunc(loaded, func(r images.Result) bool { return r.Err == nil }) {
			continue
		}

		marked := images.Mark(u.body, u.refs, func(j int) bool { return loaded[j].Err == nil })
		body := marked.Fill(render(marked.Text), func(j int) string {
			return imageBlock(c, loaded[j].Image, u.refs[j].Caption())
		})
		u.text = ansi.Wrap(u.head+body+u.tail, rendered.width, "")

		if loadCtx.Err() != nil {
			return renderedPost{}, false
		}
	}

	rendered.units = units
	rendered.assemble()
	return rendered, true
}

func imageBlock(c *ctx.Ctx, image string, caption string) string {
	lines := strings.Split(image, "\n")
	for i := range lines {
		lines[i] = imageIndent + lines[i]
	}
	return strings.Join(lines, "\n") + "\n" + imageIndent + c.Theme.Muted.Render(caption)
}

func anchor(old []int, fresh []int, top int) int {
	at := 0
	for i, line := range old {
		if line > top {
			break
		}
		at = i
	}
	if at >= len(old) || at >= len(fresh) {
		return top
	}
	return fresh[at] + top - old[at]
}
