package postshow

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/mrusme/neonmodem/internal/config"
	"github.com/mrusme/neonmodem/internal/models/author"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/discourse"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
)

const testReplyCount = 400

func testCtx(t *testing.T) *ctx.Ctx {
	t.Helper()

	cfg := config.Defaults("/cache")
	cfg.RenderImages = false // never reach out to the network in a test
	cfg.RenderShadows = false

	// discourse advertises "list:replies", which is what gates reply rendering.
	sys, err := discourse.New(system.Env{})
	if err != nil {
		t.Fatal(err)
	}
	c := ctx.New(nil, &cfg, slog.New(slog.DiscardHandler), []system.System{sys})

	return &c
}

func testPost(replies int) *post.Post {
	p := &post.Post{
		ID:      "post-1",
		Subject: "A post with a great many replies",
		Body:    "# Heading\n\nSome *markdown* body with `code` and a [link](https://example.com).\n",
		Author:  author.Author{ID: "u1", Name: "author"},
		SysIDX:  0,
	}

	for i := range replies {
		p.Replies = append(p.Replies, reply.Reply{
			ID:     fmt.Sprintf("reply-%d", i),
			PostID: p.ID,
			Body: fmt.Sprintf(
				"Reply **%d** with some `markdown`.\n\n- one\n- two\n\n> quoted\n", i),
			Author:  author.Author{ID: "u2", Name: fmt.Sprintf("replier%d", i)},
			Replies: []reply.Reply{},
		})
	}

	return p
}

// TestOpeningAPostDoesNotBlockTheEventLoop is the regression test for the UI
// hanging when a post with many replies was opened. Rendering used to happen
// inside Update; it now happens in a background command, so the handler that
// runs on the event loop must stay trivially fast no matter how many replies
// the post has.
func TestOpeningAPostDoesNotBlockTheEventLoop(t *testing.T) {
	c := testCtx(t)
	p := testPost(testReplyCount)

	// How long the work itself takes; this is what used to run in Update.
	start := time.Now()
	rendered, ok := renderPost(context.Background(), c, p, 100, 92)
	renderTook := time.Since(start)

	if !ok {
		t.Fatal("renderPost reported it was superseded when it was not")
	}
	if len(rendered.allReplies) != testReplyCount {
		t.Fatalf("expected %d replies rendered, got %d",
			testReplyCount, len(rendered.allReplies))
	}
	// replyIDs carries the post itself at index 0.
	if len(rendered.replyIDs) != testReplyCount+1 {
		t.Fatalf("expected %d replyIDs, got %d",
			testReplyCount+1, len(rendered.replyIDs))
	}

	// The output has to actually contain the post and every reply, with the
	// index markers the reply shortcut ("2r") relies on.
	for _, want := range []string{
		"A post with a great many replies",
		"Reply", "#1", fmt.Sprintf("#%d", testReplyCount),
	} {
		if !strings.Contains(rendered.content, want) {
			t.Errorf("rendered content is missing %q", want)
		}
	}
	// The index shown next to a reply must address that same reply.
	if got := rendered.replyIDs[7]; got != "reply-6" {
		t.Errorf("reply #7 maps to %q, expected reply-6", got)
	}
	if rendered.allReplies[6].ID != "reply-6" {
		t.Errorf("allReplies[6] is %q, expected reply-6", rendered.allReplies[6].ID)
	}

	// What actually runs on the event loop when the window opens.
	m := NewModel(c)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	start = time.Now()
	_, cmd := m.Update(msgs.OpenPost{Post: *p})
	handlerTook := time.Since(start)

	if cmd == nil {
		t.Fatal("opening did not schedule the background load")
	}

	t.Logf("render of %d replies took %s; the open handler took %s",
		testReplyCount, renderTook, handlerTook)

	// The handler only builds a cheap placeholder and hands the real work to a
	// command, so it must not be anywhere near the cost of rendering.
	if handlerTook > 100*time.Millisecond {
		t.Fatalf("open handler took %s, which means it is still doing the heavy "+
			"work on the event loop", handlerTook)
	}
	if !c.IsLoading() || !m.loading {
		t.Error("expected the loading state to be set while the post loads")
	}
}

// TestCancelledRenderStopsEarly proves that abandoning a post (closing the
// window, or opening another one) actually stops the render instead of letting
// it run to completion in the background.
func TestCancelledRenderStopsEarly(t *testing.T) {
	c := testCtx(t)
	p := testPost(testReplyCount)

	loadCtx := &cancelAfter{Context: context.Background(), limit: 5}

	rendered, ok := renderPost(loadCtx, c, p, 100, 92)

	if ok {
		t.Fatal("expected the cancelled render to report that it gave up")
	}
	if rendered.content != "" {
		t.Error("a cancelled render should not return content")
	}
	if loadCtx.checks > 20 {
		t.Fatalf("render kept going after cancellation: %d checks for %d replies",
			loadCtx.checks, testReplyCount)
	}
}

// TestStaleResultIsDropped covers a load that finishes after the user moved on:
// its result must not overwrite what is on screen.
func TestStaleResultIsDropped(t *testing.T) {
	c := testCtx(t)
	m := NewModel(c)
	m.activePost = testPost(1)

	_, staleGen := c.NextLoad()
	c.NextLoad() // the user opened something else / cancelled

	m.Update(loadedMsg{
		gen:  staleGen,
		post: testPost(2),
		rendered: renderedPost{
			content:    "stale content",
			replyIDs:   []string{"a", "b"},
			allReplies: []*reply.Reply{{}, {}},
		},
	})
	if len(m.allReplies) != 0 {
		t.Fatalf("stale result was applied: got %d replies", len(m.allReplies))
	}

	_, currentGen := c.NextLoad()
	m.Update(loadedMsg{
		gen:  currentGen,
		post: testPost(2),
		rendered: renderedPost{
			content:    "fresh content",
			replyIDs:   []string{"post-1", "reply-0", "reply-1"},
			allReplies: []*reply.Reply{{ID: "reply-0"}, {ID: "reply-1"}},
		},
	})
	if len(m.allReplies) != 2 || c.IsLoading() {
		t.Fatal("the current result should be applied and end the loading state")
	}
}

// TestRepeatedRendersDoNotAccumulate guards the reply bookkeeping: the indices
// shown next to replies come from these slices, so a re-render (a refresh after
// posting a reply, for instance) must rebuild them rather than append to them.
func TestRepeatedRendersDoNotAccumulate(t *testing.T) {
	c := testCtx(t)
	p := testPost(10)

	first, ok := renderPost(context.Background(), c, p, 100, 92)
	if !ok {
		t.Fatal("first render was superseded")
	}
	second, ok := renderPost(context.Background(), c, p, 100, 92)
	if !ok {
		t.Fatal("second render was superseded")
	}

	if len(first.allReplies) != len(second.allReplies) ||
		len(first.replyIDs) != len(second.replyIDs) {
		t.Fatalf("reply bookkeeping grew across renders: %d/%d then %d/%d",
			len(first.allReplies), len(first.replyIDs),
			len(second.allReplies), len(second.replyIDs))
	}
}

func TestOlderRepliesNoticeFollowsThePage(t *testing.T) {
	c := testCtx(t)
	p := testPost(2)

	rendered, _ := renderPost(context.Background(), c, p, 100, 92)
	if strings.Contains(rendered.content, "Older replies available") {
		t.Error("no older-replies notice expected when everything is loaded")
	}

	p.ReplyPage = post.ReplyPage{Offset: 20, Size: 20, Total: 60}
	rendered, _ = renderPost(context.Background(), c, p, 100, 92)
	if !strings.Contains(rendered.content, "Older replies available") {
		t.Error("expected the older-replies notice for a later page")
	}
}

type cancelAfter struct {
	context.Context
	checks int
	limit  int
}

func (c *cancelAfter) Err() error {
	c.checks++
	if c.checks > c.limit {
		return context.Canceled
	}
	return nil
}

type writableSystem struct{ system.System }

func (writableSystem) Capabilities() system.Capabilities { return system.CapRead | system.CapWrite }

func TestReplyShortcutTargetsTheNumberedReply(t *testing.T) {
	c := testCtx(t)
	c.Systems = []system.System{writableSystem{c.Systems[0]}}
	m := NewModel(c)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	p := testPost(3)
	m.activePost = p
	m.replyIDs = []string{p.ID, "reply-0", "reply-1", "reply-2"}
	m.allReplies = []*reply.Reply{&p.Replies[0], &p.Replies[1], &p.Replies[2]}

	m.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if cmd == nil {
		t.Fatal("expected a compose message")
	}
	compose, ok := cmd().(msgs.Compose)
	if !ok {
		t.Fatalf("expected msgs.Compose, got %T", cmd())
	}
	if compose.Action != msgs.ComposeReply || compose.Parent == nil || compose.Parent.ID != "reply-1" {
		t.Errorf("unexpected compose target: %+v", compose)
	}

	_, cmd = m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	compose = cmd().(msgs.Compose)
	if compose.Parent != nil {
		t.Error("a plain r must reply to the post itself")
	}
}
