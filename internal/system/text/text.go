package text

import (
	"regexp"
	"strings"
	"sync"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/araddon/dateparse"
	"github.com/charmbracelet/x/ansi"
)

var converter = sync.OnceValue(func() *md.Converter {
	return md.NewConverter("", true, nil)
})

var (
	emojiImage = regexp.MustCompile(`(?is)<img\b[^>]*\bclass="[^"]*\bemoji\b[^"]*"[^>]*>`)
	altText    = regexp.MustCompile(`(?is)\balt="([^"]*)"`)
)

func replaceEmojiImages(html string) string {
	return emojiImage.ReplaceAllStringFunc(html, func(tag string) string {
		if m := altText.FindStringSubmatch(tag); m != nil {
			return m[1]
		}
		return ""
	})
}

func Markdown(html string) string {
	if strings.TrimSpace(html) == "" {
		return ""
	}
	out, err := converter().ConvertString(replaceEmojiImages(html))
	if err != nil {
		return html
	}
	return out
}

func Time(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := dateparse.ParseAny(s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func Truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "")
}

func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
