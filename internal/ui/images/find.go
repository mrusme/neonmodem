package images

import (
	"bytes"
	"math/rand/v2"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/mrusme/neonmodem/internal/system/text"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	gtext "github.com/yuin/goldmark/text"
)

const (
	tokenStart  = "nmimg"
	nonceLength = 5
	nonceChars  = "0123456789abcdefghijklmnopqrstuvwxyz"
)

var anyHostURL = regexp.MustCompile(`^(?:http|https|ftp)://(?:\[[0-9a-fA-F:.]+\]|[-a-zA-Z0-9@:%._\+~#=]{1,256})` +
	`(?::\d+)?(?:[/#?][-a-zA-Z0-9@:%_+.~#$!?&/=\(\);,'">\^{}\[\]` + "`" + `]*)?`)

var extensions = map[string]bool{
	".bmp":  true,
	".gif":  true,
	".jpeg": true,
	".jpg":  true,
	".png":  true,
	".tif":  true,
	".tiff": true,
	".webp": true,
}

type Ref struct {
	URL string
	Alt string
	At  int
}

func (r Ref) Caption() string {
	if r.Alt != "" {
		return text.Printable(r.Alt)
	}
	u, err := url.Parse(r.URL)
	if err != nil {
		return text.Printable(r.URL)
	}
	name := path.Base(u.Path)
	if name == "/" || name == "." {
		return text.Printable(u.Host)
	}
	return text.Printable(name)
}

func Find(markdown string) []Ref {
	source := []byte(markdown)
	doc := goldmark.New(goldmark.WithExtensions(
		extension.NewLinkify(extension.WithLinkifyURLRegexp(anyHostURL)),
		extension.Table,
		extension.Strikethrough,
		extension.TaskList,
		extension.DefinitionList,
	)).Parser().Parse(gtext.NewReader(source))

	var refs []Ref
	for block := doc.FirstChild(); block != nil; block = block.NextSibling() {
		at := insertAt(block.NextSibling(), source)
		seen := map[string]bool{}
		_ = ast.Walk(block, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering {
				return ast.WalkContinue, nil
			}
			link, alt, ok := imageLink(node, source)
			if !ok {
				return ast.WalkContinue, nil
			}
			if !seen[link] {
				seen[link] = true
				refs = append(refs, Ref{URL: link, Alt: alt, At: at})
			}
			return ast.WalkSkipChildren, nil
		})
	}
	return refs
}

func imageLink(node ast.Node, source []byte) (string, string, bool) {
	switch node := node.(type) {
	case *ast.Image:
		link := string(node.Destination)
		fetch, _ := classify(link)
		return link, text.InlineText(node, source), fetch
	case *ast.AutoLink:
		link := string(node.URL(source))
		fetch, picture := classify(link)
		return link, "", node.AutoLinkType == ast.AutoLinkURL && fetch && picture
	case *ast.Link:
		link := string(node.Destination)
		fetch, picture := classify(link)
		return link, text.InlineText(node, source), fetch && picture && !holdsImage(node)
	}
	return "", "", false
}

func classify(link string) (fetch bool, picture bool) {
	u, err := url.Parse(link)
	if err != nil {
		return false, false
	}
	fetch = (strings.EqualFold(u.Scheme, "http") || strings.EqualFold(u.Scheme, "https")) && u.Host != ""
	return fetch, extensions[strings.ToLower(path.Ext(u.Path))]
}

func holdsImage(node ast.Node) bool {
	found := false
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if _, ok := n.(*ast.Image); ok && entering {
			found = true
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	return found
}

func insertAt(next ast.Node, source []byte) int {
	if next == nil {
		return len(source)
	}
	start := next.Pos()
	if start < 0 {
		start = firstOffset(next)
	}
	if start < 0 || start > len(source) {
		return len(source)
	}
	return bytes.LastIndexByte(source[:start], '\n') + 1
}

func firstOffset(node ast.Node) int {
	offset := -1
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if n.Type() == ast.TypeBlock && n.Lines().Len() > 0 {
			offset = n.Lines().At(0).Start
			return ast.WalkStop, nil
		}
		if t, ok := n.(*ast.Text); ok {
			offset = t.Segment.Start
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	return offset
}

type Marked struct {
	Text   string
	prefix string
	tokens map[string]int
}

func Mark(markdown string, refs []Ref, keep func(i int) bool) Marked {
	m := Marked{prefix: tokenPrefix(markdown), tokens: map[string]int{}}

	var b strings.Builder
	last := 0
	for i, ref := range refs {
		if !keep(i) {
			continue
		}
		at := min(max(ref.At, last), len(markdown))
		b.WriteString(markdown[last:at])
		token := m.prefix + strconv.FormatInt(int64(i), 36)
		m.tokens[token] = i
		b.WriteString("\n\n" + token + "\n\n")
		last = at
	}
	b.WriteString(markdown[last:])

	m.Text = b.String()
	return m
}

func tokenPrefix(markdown string) string {
	for {
		nonce := make([]byte, nonceLength)
		for i := range nonce {
			nonce[i] = nonceChars[rand.IntN(len(nonceChars))]
		}
		prefix := tokenStart + string(nonce)
		if !strings.Contains(markdown, prefix) {
			return prefix
		}
	}
}

func (m Marked) Fill(rendered string, block func(i int) string) string {
	lines := strings.Split(rendered, "\n")
	out := lines[:0]
	for _, line := range lines {
		plain := strings.TrimSpace(ansi.Strip(line))
		if i, ok := m.tokens[plain]; ok {
			out = append(out, block(i))
			continue
		}
		if strings.Contains(plain, m.prefix) {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
