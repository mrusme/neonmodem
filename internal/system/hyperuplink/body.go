package hyperuplink

import (
	"fmt"
	"strings"

	"github.com/mrusme/neonmodem/internal/system/hyperuplink/api"
	"github.com/mrusme/neonmodem/internal/system/text"
)

const pollTimeLayout = "2006-01-02 15:04"

var altEscaper = strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`, "\r\n", " ", "\n", " ", "\r", " ")

func describe(origin string, body string, poll *api.Poll, attachments []api.Attachment) string {
	var blocks []string
	if poll != nil {
		blocks = append(blocks, pollSummary(poll))
	}
	if len(attachments) > 0 {
		blocks = append(blocks, attachmentList(origin, attachments))
	}
	if len(blocks) == 0 {
		return body
	}

	if body = strings.TrimRight(body, "\n"); body != "" {
		blocks = append([]string{body}, blocks...)
	}
	return strings.Join(blocks, "\n\n")
}

func pollSummary(poll *api.Poll) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Poll, %s", votes(poll.Total))
	switch {
	case poll.Ended:
		b.WriteString(", ended")
	case poll.EndsAt != "":
		if ends := text.Time(poll.EndsAt); !ends.IsZero() {
			fmt.Fprintf(&b, ", open until %s", ends.Local().Format(pollTimeLayout))
		}
	}
	b.WriteString(":")

	for _, option := range poll.Options {
		fmt.Fprintf(&b, "\n- %s: %s (%d%%)", option.Text, votes(option.Votes), option.Percent)
	}

	return b.String()
}

func votes(n int) string {
	if n == 1 {
		return "1 vote"
	}
	return fmt.Sprintf("%d votes", n)
}

func attachmentList(origin string, attachments []api.Attachment) string {
	var b strings.Builder
	b.WriteString("Attachments:")
	for _, a := range attachments {
		label := a.Filename
		if a.MimeType != "" {
			label += " (" + a.MimeType + ")"
		}
		link := attachmentURL(origin, a)
		if strings.HasPrefix(a.MimeType, "image/") {
			fmt.Fprintf(&b, "\n- ![%s](%s)", altEscaper.Replace(label), link)
			continue
		}
		fmt.Fprintf(&b, "\n- %s: %s", label, link)
	}

	return b.String()
}

func attachmentURL(origin string, a api.Attachment) string {
	if strings.HasPrefix(a.URL, "http://") || strings.HasPrefix(a.URL, "https://") {
		return a.URL
	}
	return strings.TrimRight(origin, "/") + "/" + strings.TrimLeft(a.URL, "/")
}
