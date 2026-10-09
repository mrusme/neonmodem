package postshow

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mrusme/neonmodem/internal/config"
	"github.com/mrusme/neonmodem/internal/models/author"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
)

const (
	red   = "200;40;40"
	block = "\u2584"
)

func imageServer(t *testing.T) string {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			img.Set(x, y, color.RGBA{R: 200, G: 40, B: 40, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(buf.Bytes())
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func imageCtx(t *testing.T, systems ...system.System) *ctx.Ctx {
	t.Helper()

	cfg := config.Defaults("/cache")
	cfg.RenderShadows = false
	if len(systems) == 0 {
		systems = []system.System{&countingSystem{}}
	}
	c := ctx.New(nil, &cfg, slog.New(slog.DiscardHandler), systems)
	return &c
}

func postWithImages(base string) *post.Post {
	return &post.Post{
		ID:      "p",
		Subject: "pictures",
		Body:    "Look at this:\n" + base + "/a.png\n\nAfter the image.",
		Author:  author.Author{Name: "author"},
		Replies: []reply.Reply{
			{ID: "r0", Body: "No picture here.", Author: author.Author{Name: "first"}},
			{ID: "r1", Body: "![a reply picture](" + base + "/b)\n\nReply text after.", Author: author.Author{Name: "second"}},
		},
	}
}

func before(t *testing.T, text string, parts ...string) {
	t.Helper()

	from := 0
	for _, part := range parts {
		i := strings.Index(text[from:], part)
		if i < 0 {
			t.Fatalf("%q isn't after the previous part in:\n%s", part, text)
		}
		from += i + len(part)
	}
}

func TestImagesFollowTheirBlocksInPostAndReplies(t *testing.T) {
	c := imageCtx(t)
	p := postWithImages(imageServer(t))

	text, ok := renderPost(context.Background(), c, p, 80)
	if !ok {
		t.Fatal("renderPost was superseded")
	}
	if strings.Contains(text.content, red) || !text.hasImages() {
		t.Fatal("the text phase must find the images without rendering them")
	}
	if len(text.units) != 3 || len(text.units[0].refs) != 1 || len(text.units[1].refs) != 0 || len(text.units[2].refs) != 1 {
		t.Fatalf("the refs per unit are wrong: %+v", text.units)
	}

	withImages, ok := renderImages(context.Background(), c, text, 10, nil)
	if !ok {
		t.Fatal("renderImages was superseded")
	}
	if strings.Count(withImages.content, red) == 0 {
		t.Fatal("no image was rendered")
	}

	plain := ansi.Strip(withImages.content)
	before(t, plain, "Look at this:", "a.png", block, "a.png", "After the image.", "No picture here.",
		"a reply picture", block, "a reply picture", "Reply text after.")

	if withImages.lines[0] != 0 || withImages.lines[1] <= text.lines[1] || withImages.lines[2] <= text.lines[2] {
		t.Errorf("the units start at %v, before the images at %v", withImages.lines, text.lines)
	}
	if withImages.lines[2]-withImages.lines[1] != text.lines[2]-text.lines[1] {
		t.Error("a reply without images keeps its length")
	}
}

func TestFailedImagesStayText(t *testing.T) {
	c := imageCtx(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	p := postWithImages(srv.URL)

	text, _ := renderPost(context.Background(), c, p, 80)
	withImages, ok := renderImages(context.Background(), c, text, 10, nil)
	if !ok || withImages.content != text.content {
		t.Error("images that fail must leave the text as it is")
	}
}

func TestAnchorKeepsTheReadingPosition(t *testing.T) {
	old := []int{0, 10, 20}
	fresh := []int{0, 25, 40}
	for _, c := range []struct{ top, want int }{
		{0, 0},
		{5, 5},
		{10, 25},
		{12, 27},
		{25, 45},
	} {
		if got := anchor(old, fresh, c.top); got != c.want {
			t.Errorf("anchor(%d) = %d, want %d", c.top, got, c.want)
		}
	}
	if got := anchor(nil, nil, 7); got != 7 {
		t.Errorf("without units the position stays, got %d", got)
	}
}

func openedModel(t *testing.T, c *ctx.Ctx, height int) *Model {
	t.Helper()

	m := NewModel(c)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: height})
	m.Update(msgs.FocusWindow{ID: WIN_ID})
	return m
}

func TestTheImagePhaseFollowsTheText(t *testing.T) {
	c := imageCtx(t)
	m := openedModel(t, c, 12)
	p := postWithImages(imageServer(t))
	m.activePost = p

	loadCtx, gen := c.NextLoad()
	text, _ := renderPost(loadCtx, c, p, m.viewport.Width())
	_, cmd := m.Update(loadedMsg{gen: gen, loadCtx: loadCtx, post: p, rendered: text})
	if cmd == nil || !c.IsLoading() || m.loading {
		t.Fatal("the text must show at once and the image phase start")
	}
	if strings.Contains(m.viewport.GetContent(), red) {
		t.Fatal("the text phase showed an image")
	}

	reading := text.lines[1] + 1
	m.viewport.SetYOffset(reading)
	if m.viewport.YOffset() != reading {
		t.Fatalf("the test needs a scrollable window, the offset is %d", m.viewport.YOffset())
	}
	msg := cmd()
	images, ok := msg.(imagesMsg)
	if !ok {
		t.Fatalf("the image phase returned %T", msg)
	}
	m.Update(images)
	if !strings.Contains(m.viewport.GetContent(), red) || c.IsLoading() {
		t.Error("the images must show and the image phase end")
	}
	if got, want := m.viewport.YOffset(), images.rendered.lines[1]+1; got != want || got <= reading {
		t.Errorf("the top line moved to %d, want %d, below the image above it", got, want)
	}
}

func TestAStaleImagePhaseChangesNothing(t *testing.T) {
	c := imageCtx(t)
	m := openedModel(t, c, 40)
	p := postWithImages(imageServer(t))
	m.activePost = p

	loadCtx, gen := c.NextLoad()
	text, _ := renderPost(loadCtx, c, p, m.viewport.Width())
	_, cmd := m.Update(loadedMsg{gen: gen, loadCtx: loadCtx, post: p, rendered: text})
	msg := cmd()

	c.NextLoad()
	m.Update(msg)
	if strings.Contains(m.viewport.GetContent(), red) {
		t.Error("a stale image phase replaced the content")
	}
}

func TestAReloadEndsTheImagePhase(t *testing.T) {
	sys := &countingSystem{}
	c := imageCtx(t, sys)
	m := openedModel(t, c, 40)
	m.activePost = &post.Post{ID: "p", SysIDX: 0}

	c.StartLoading(ctx.LoadImages)
	_, cmd := m.Update(msgs.ReloadPost{})
	run(t, m, cmd)
	if c.IsLoading() {
		t.Error("the reload must end the image phase it replaced")
	}
}

func TestNoImagePhaseWithoutRenderImages(t *testing.T) {
	c := imageCtx(t)
	c.Config.RenderImages = false
	m := openedModel(t, c, 40)
	p := postWithImages(imageServer(t))

	loadCtx, gen := c.NextLoad()
	text, _ := renderPost(loadCtx, c, p, m.viewport.Width())
	if text.hasImages() {
		t.Fatal("refs were collected with RenderImages off")
	}
	if _, cmd := m.Update(loadedMsg{gen: gen, loadCtx: loadCtx, post: p, rendered: text}); cmd != nil || c.IsLoading() {
		t.Error("no image phase may start with RenderImages off")
	}
}

type mediaSystem struct {
	countingSystem
	header string
}

func (s *mediaSystem) AuthorizeMedia(req *http.Request) {
	req.Header.Set("Authorization", s.header)
}

func TestThePostsSystemAuthorizesItsImages(t *testing.T) {
	base := imageServer(t)
	sys := &mediaSystem{header: "Bearer hup_test"}
	c := imageCtx(t, sys)

	authorize := mediaAuthorizer(c, &post.Post{SysIDX: 0})
	if authorize == nil {
		t.Fatal("the system's authorizer was not used")
	}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, base+"/a.png", nil)
	authorize(req)
	if req.Header.Get("Authorization") != "Bearer hup_test" {
		t.Error("the authorizer didn't run")
	}

	if mediaAuthorizer(imageCtx(t), &post.Post{SysIDX: 0}) != nil {
		t.Error("a system without an authorizer got one")
	}
	if mediaAuthorizer(c, &post.Post{SysIDX: 3}) != nil || mediaAuthorizer(c, nil) != nil {
		t.Error("a post without a system got an authorizer")
	}
}
