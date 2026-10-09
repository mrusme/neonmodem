package images

import (
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"charm.land/glamour/v2"
	"github.com/charmbracelet/x/ansi"
)

func urls(refs []Ref) []string {
	out := []string{}
	for _, r := range refs {
		out = append(out, r.URL)
	}
	return out
}

func TestFindRecognizesImages(t *testing.T) {
	cases := []struct {
		name     string
		markdown string
		want     []string
	}{
		{"image syntax without an extension", "![a cat](https://a.example/cat)", []string{"https://a.example/cat"}},
		{"image syntax without alt text", "![](https://a.example/y.png)", []string{"https://a.example/y.png"}},
		{"bare urls of every format", "https://a.example/p.jpg https://a.example/p.jpeg https://a.example/p.png " +
			"https://a.example/p.gif https://a.example/p.webp https://a.example/p.bmp https://a.example/p.tif " +
			"https://a.example/p.tiff", []string{
			"https://a.example/p.jpg", "https://a.example/p.jpeg", "https://a.example/p.png",
			"https://a.example/p.gif", "https://a.example/p.webp", "https://a.example/p.bmp",
			"https://a.example/p.tif", "https://a.example/p.tiff",
		}},
		{"upper case", "see https://a.example/P.PNG now", []string{"https://a.example/P.PNG"}},
		{"query after the extension", "https://a.example/p.png?size=2", []string{"https://a.example/p.png?size=2"}},
		{"hosts without a dot", "http://localhost:3001/a.png and http://127.0.0.1/b.gif and http://[::1]:3001/c.webp",
			[]string{"http://localhost:3001/a.png", "http://127.0.0.1/b.gif", "http://[::1]:3001/c.webp"}},
		{"in parentheses", "(see https://a.example/p.png)", []string{"https://a.example/p.png"}},
		{"angle autolink", "<https://a.example/angle.png>", []string{"https://a.example/angle.png"}},
		{"www link", "www.example.com/w.png", []string{"http://www.example.com/w.png"}},
		{"link with an image extension", "[see](https://a.example/s.png)", []string{"https://a.example/s.png"}},
		{"lightbox counts as its image", `[![thumb](https://a.example/t.jpeg)](https://a.example/o.jpeg "title")`,
			[]string{"https://a.example/t.jpeg"}},
		{"explicit image of another type", "![](https://a.example/v.mp4)", []string{"https://a.example/v.mp4"}},
		{"relative url", "![](/relative.png)", []string{}},
		{"ftp", "ftp://a.example/f.png", []string{}},
		{"non-image links", "[docs](https://a.example/page) https://a.example/page https://a.example/v.mp4", []string{}},
		{"code", "`https://a.example/c.png`\n\n```\n![](https://a.example/d.png)\n```\n\n    https://a.example/e.png\n", []string{}},
		{"repeated in one block", "https://a.example/r.png and https://a.example/r.png", []string{"https://a.example/r.png"}},
		{"repeated in two blocks", "https://a.example/r.png\n\nhttps://a.example/r.png",
			[]string{"https://a.example/r.png", "https://a.example/r.png"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := urls(Find(c.markdown)); !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestFindKeepsAltTextAndCaptions(t *testing.T) {
	refs := Find("![my \\[draft\\]\\\\v2 *plan* (image/png)](https://a.example/a)\n\n[label](https://a.example/b.png)\n\n" +
		"https://a.example/some%20pic.webp\n\nhttps://a.example/")
	if len(refs) != 3 {
		t.Fatalf("got %+v", refs)
	}
	if refs[0].Alt != "my [draft]\\v2 plan (image/png)" || refs[0].Caption() != refs[0].Alt {
		t.Errorf("the image's alt text is %q", refs[0].Alt)
	}
	if refs[1].Caption() != "label" {
		t.Errorf("the link's caption is %q", refs[1].Caption())
	}
	if refs[2].Alt != "" || refs[2].Caption() != "some pic.webp" {
		t.Errorf("the bare URL's caption is %q", refs[2].Caption())
	}
	if (Ref{URL: "https://a.example/"}).Caption() != "a.example" {
		t.Error("a URL without a file name is captioned with its host")
	}
}

func TestFindPlacesRefsBeforeTheNextBlock(t *testing.T) {
	markdown := "para one ![a](https://a.example/1.png)\n" +
		"still para one\n" +
		"\n" +
		"- item ![b](https://a.example/2.png)\n" +
		"- item two\n" +
		"\n" +
		"> quote https://a.example/3.png\n" +
		"\n" +
		"| h |\n" +
		"| - |\n" +
		"| https://a.example/4.png |\n" +
		"\n" +
		"## heading ![c](https://a.example/5.png)\n" +
		"last paragraph https://a.example/6.png"

	refs := Find(markdown)
	lineOf := func(prefix string) int {
		i := strings.Index(markdown, "\n"+prefix)
		if i < 0 {
			t.Fatalf("no line starts with %q", prefix)
		}
		return i + 1
	}
	want := []int{
		lineOf("- item ![b]"),
		lineOf("> quote"),
		lineOf("| h |"),
		lineOf("## heading"),
		lineOf("last paragraph"),
		len(markdown),
	}
	if len(refs) != len(want) {
		t.Fatalf("got %d refs: %+v", len(refs), refs)
	}
	for i, ref := range refs {
		if ref.At != want[i] {
			t.Errorf("ref %d (%s) goes to %d, want %d", i, ref.URL, ref.At, want[i])
		}
	}
}

func TestMarkKeepsOnlyChosenRefsInOrder(t *testing.T) {
	markdown := "one https://a.example/1.png two https://a.example/2.png\n\nthree"
	refs := Find(markdown)
	if len(refs) != 2 || refs[0].At != refs[1].At {
		t.Fatalf("got %+v", refs)
	}

	marked := Mark(markdown, refs, func(i int) bool { return true })
	first := strings.Index(marked.Text, marked.prefix+"0")
	second := strings.Index(marked.Text, marked.prefix+"1")
	if first < 0 || second < first || !strings.HasPrefix(marked.Text, "one ") ||
		!strings.HasSuffix(marked.Text, "\n\nthree") {
		t.Errorf("the marked text is %q", marked.Text)
	}

	only := Mark(markdown, refs, func(i int) bool { return i == 1 })
	if strings.Contains(only.Text, only.prefix+"0") || !strings.Contains(only.Text, only.prefix+"1") {
		t.Errorf("the marked text is %q", only.Text)
	}
	if none := Mark(markdown, refs, func(int) bool { return false }); none.Text != markdown {
		t.Errorf("marking nothing changed the text: %q", none.Text)
	}
}

func TestFillReplacesTokenLinesAndDropsStrayOnes(t *testing.T) {
	marked := Mark("x", []Ref{{URL: "https://a.example/1.png", At: 1}}, func(int) bool { return true })
	token := marked.prefix + "0"
	rendered := "  text\n  \x1b[38;5;252m" + token + "\x1b[m   \n  broken " + marked.prefix + "\n  more"

	got := marked.Fill(rendered, func(i int) string { return "IMAGE-" + strconv.Itoa(i) })
	if got != "  text\nIMAGE-0\n  more" {
		t.Errorf("got %q", got)
	}
}

var osc8 = regexp.MustCompile("\x1b\\]8;[^\a]*\a")

func TestPlaceholdersSurviveGlamour(t *testing.T) {
	markdown := "Here is the new layout:\n" +
		"![](https://programming.dev/pictrs/image/ca20ea57-d800-4fcd-b49c-af43ed594823.png)\n" +
		"It still needs a darker sidebar.\n" +
		"\n" +
		"Attachments:\n" +
		"- ![post.png (image/png)](http://localhost:3001/api/v1/attachments/609a848a-36c2-47eb-968f-4afc0d4160f3)\n" +
		"- notes.txt (text/plain): http://localhost:3001/api/v1/attachments/0192b1c2\n" +
		"\n" +
		"> a quoted https://board.example/quoted.webp\n" +
		"\n" +
		"The end."

	for _, width := range []int{40, 120} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			r, err := glamour.NewTermRenderer(glamour.WithStandardStyle("dark"), glamour.WithWordWrap(width))
			if err != nil {
				t.Fatal(err)
			}

			refs := Find(markdown)
			if len(refs) != 3 {
				t.Fatalf("got %+v", refs)
			}
			marked := Mark(markdown, refs, func(int) bool { return true })
			out, err := r.Render(marked.Text)
			if err != nil {
				t.Fatal(err)
			}
			filled := marked.Fill(out, func(i int) string { return "IMAGE-" + strconv.Itoa(i) })
			plain := ansi.Strip(filled)

			order := []string{"darker sidebar", "IMAGE-0", "notes.txt", "IMAGE-1", "a quoted", "IMAGE-2", "The end."}
			last := -1
			for _, s := range order {
				i := strings.Index(plain, s)
				if i <= last {
					t.Fatalf("%q isn't after the previous part:\n%s", s, plain)
				}
				last = i
			}
			if strings.Contains(plain, tokenStart) {
				t.Errorf("a token is left:\n%s", plain)
			}
			if !strings.Contains(strings.Join(strings.Fields(plain), ""), "ca20ea57-d800-4fcd-b49c-af43ed594823.png") {
				t.Errorf("the image URL text is gone:\n%s", plain)
			}
			for _, esc := range osc8.FindAllString(filled, -1) {
				if strings.Contains(esc, "\n") || strings.Contains(esc, "IMAGE") {
					t.Errorf("an escape sequence was changed: %q", esc)
				}
			}
		})
	}
}

func TestCaptionsCarryNoControlCharacters(t *testing.T) {
	refs := Find("![alt &#27;&#91;2J end](https://a.example/a)\n\n" +
		"https://a.example/%1b%5B2Jname.png\n\n" +
		"https://a.example/100%251bpercent.png\n\n" +
		"![](https://a.example/c1%C2%9Bcsi.png)")
	if len(refs) != 4 {
		t.Fatalf("got %+v", refs)
	}
	want := []string{"alt  end", "name.png", "100%1bpercent.png", "c1csi.png"}
	for i, ref := range refs {
		if got := ref.Caption(); got != want[i] {
			t.Errorf("caption %d is %q, want %q", i, got, want[i])
		}
	}
	if got := (Ref{URL: "https://a.example/\x1b[2J.png"}).Caption(); strings.ContainsRune(got, 0x1b) {
		t.Errorf("an unparsable URL leaked a control character: %q", got)
	}
}
