package text

import (
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/araddon/dateparse"
	"github.com/charmbracelet/x/ansi"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	gtext "github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
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

func Printable(s string) string {
	return printable(s, false)
}

func PrintableLines(s string) string {
	return printable(s, true)
}

func printable(s string, lines bool) string {
	s = ansi.Strip(s)
	if lines {
		s = strings.ReplaceAll(s, "\r\n", "\n")
	}
	return strings.Map(func(r rune) rune {
		switch {
		case lines && r == '\n':
			return r
		case r == '\t' || r == '\n' || r == '\r':
			return ' '
		case unicode.IsControl(r):
			return -1
		}
		return r
	}, s)
}

func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func FirstParagraph(markdown string) string {
	source := []byte(markdown)
	doc := goldmark.New().Parser().Parse(gtext.NewReader(source))

	for node := doc.FirstChild(); node != nil; node = node.NextSibling() {
		if _, ok := node.(*ast.Paragraph); !ok {
			continue
		}
		paragraph := InlineText(node, source)
		if paragraph != "" && !strings.HasPrefix(paragraph, ":::") {
			return paragraph
		}
	}
	return ""
}

func InlineText(node ast.Node, source []byte) string {
	var b strings.Builder
	writeInline(&b, node, source, false)
	return strings.Join(strings.Fields(b.String()), " ")
}

func writeInline(b *strings.Builder, parent ast.Node, source []byte, code bool) {
	for node := parent.FirstChild(); node != nil; node = node.NextSibling() {
		switch node := node.(type) {
		case *ast.Text:
			value := node.Segment.Value(source)
			if !code {
				value = util.ResolveEntityNames(util.ResolveNumericReferences(util.UnescapePunctuations(value)))
			}
			b.Write(value)
			if node.SoftLineBreak() || node.HardLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(node.Value)
		case *ast.AutoLink:
			b.Write(node.Label(source))
		case *ast.CodeSpan:
			writeInline(b, node, source, true)
		case *ast.Image, *ast.RawHTML:
		default:
			writeInline(b, node, source, code)
		}
	}
}
