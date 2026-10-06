package postcreate

import (
	"errors"
	"net/url"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
)

func handleTab(mi interface{}) (bool, []tea.Cmd) {
	m := mi.(*Model)

	if m.action == msgs.ComposeReply || m.submitting {
		return true, nil
	}

	if m.inputFocused == 0 {
		m.inputFocused = 1
		m.textinput.Blur()
		return true, []tea.Cmd{m.textarea.Focus()}
	}

	m.inputFocused = 0
	m.textarea.Blur()
	return true, []tea.Cmd{m.textinput.Focus()}
}

func IsLink(body string) bool {
	fields := strings.Fields(body)
	if len(fields) != 1 {
		return false
	}
	u, err := url.Parse(fields[0])
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func rejected(text string) (bool, []tea.Cmd) {
	return true, []tea.Cmd{msgs.Send(msgs.Error(errors.New(text)))}
}

func handleSubmit(mi interface{}) (bool, []tea.Cmd) {
	m := mi.(*Model)

	if m.submitting {
		return true, nil
	}

	switch m.action {
	case msgs.ComposePost:
		subject := strings.TrimSpace(m.textinput.Value())
		body := strings.TrimSpace(m.textarea.Value())
		if subject == "" {
			return rejected("The post needs a subject.")
		}
		if body == "" {
			return rejected("The post needs a body or a link.")
		}

		kind := post.KindText
		if IsLink(body) {
			kind = post.KindLink
		}

		p := post.Post{
			Subject: subject,
			Body:    body,
			Kind:    kind,
			Forum:   m.post.Forum,
			SysIDX:  m.post.SysIDX,
		}

		m.submitting = true
		m.ctx.Loading = true
		return true, []tea.Cmd{m.createPost(p)}

	case msgs.ComposeReply:
		body := strings.TrimSpace(m.textarea.Value())
		if body == "" {
			return rejected("The reply needs some text.")
		}

		r := reply.Reply{
			PostID: m.post.ID,
			Body:   body,
			SysIDX: m.post.SysIDX,
		}
		if m.parent != nil {
			r.ParentID = m.parent.ID
			if m.parent.PostID != "" {
				r.PostID = m.parent.PostID
			}
		}

		m.submitting = true
		m.ctx.Loading = true
		return true, []tea.Cmd{m.createReply(r)}
	}

	return true, nil
}

func handleViewResize(mi interface{}) (bool, []tea.Cmd) {
	m := mi.(*Model)

	width := m.tk.ViewWidth() - 4
	if width < 10 {
		width = 10
	}
	m.textinput.SetWidth(width)
	m.textarea.SetWidth(width)
	m.textarea.SetHeight(6)

	return false, nil
}
