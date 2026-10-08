package postcreate

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/mrusme/neonmodem/internal/config"
	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/prompt"
	"github.com/mrusme/neonmodem/internal/ui/ctx"
	"github.com/mrusme/neonmodem/internal/ui/msgs"
)

// fakeSystem stands in for a connected board so the tests can see exactly when
// the network call happens, and make it fail on demand.
type fakeSystem struct {
	createPostCalls  int
	createReplyCalls int
	lastReply        reply.Reply
	lastPost         post.Post
	err              error
}

func (s *fakeSystem) Kind() string                      { return "fake" }
func (s *fakeSystem) URL() string                       { return "" }
func (s *fakeSystem) Title() string                     { return "fake" }
func (s *fakeSystem) Description() string               { return "fake" }
func (s *fakeSystem) Capabilities() system.Capabilities { return system.CapRead | system.CapWrite }
func (s *fakeSystem) Connect(ctx context.Context, p prompt.Prompter, u string) (system.Settings, error) {
	return system.Settings{}, nil
}
func (s *fakeSystem) ListForums(ctx context.Context) ([]forum.Forum, error) { return nil, nil }
func (s *fakeSystem) Orders(string) system.Ordering                         { return system.Only(system.OrderNew) }
func (s *fakeSystem) ListPosts(ctx context.Context, forumID string, order system.Order) ([]post.Post, error) {
	return nil, nil
}
func (s *fakeSystem) LoadPost(ctx context.Context, p *post.Post) error { return nil }
func (s *fakeSystem) CreatePost(ctx context.Context, p *post.Post) error {
	s.createPostCalls++
	s.lastPost = *p
	return s.err
}
func (s *fakeSystem) CreateReply(ctx context.Context, r *reply.Reply) error {
	s.createReplyCalls++
	s.lastReply = *r
	return s.err
}

func testModel(t *testing.T, fake *fakeSystem) (*Model, *ctx.Ctx) {
	t.Helper()

	cfg := config.Defaults("/cache")
	c := ctx.New(nil, &cfg, slog.New(slog.DiscardHandler), []system.System{fake})
	m := NewModel(&c)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 14})

	return m, &c
}

func collect(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, collect(c)...)
		}
		return out
	}
	if msg == nil {
		return nil
	}
	return []tea.Msg{msg}
}

func openPost(m *Model) {
	m.Update(msgs.Compose{Action: msgs.ComposePost, Post: post.Post{Forum: forum.Forum{ID: "f1", Name: "f"}, SysIDX: 0}})
}

// TestSubmitDoesNotPostOnTheEventLoop is the regression test: handleSubmit runs
// in Update, so it must only schedule the request, never perform it.
func TestSubmitDoesNotPostOnTheEventLoop(t *testing.T) {
	fake := &fakeSystem{}
	m, c := testModel(t, fake)
	openPost(m)
	m.textinput.SetValue("A subject")
	m.textarea.SetValue("A body")

	handled, cmds := m.handleSubmit()
	if !handled {
		t.Fatal("submit was not handled")
	}
	if fake.createPostCalls != 0 {
		t.Fatal("CreatePost ran on the event loop instead of in a command")
	}
	if len(cmds) != 1 {
		t.Fatalf("expected one background command, got %d", len(cmds))
	}
	if !m.submitting || !c.IsLoading() {
		t.Error("expected the window to be marked as submitting and loading")
	}

	// Running the command is what actually posts.
	msg := cmds[0]()
	if fake.createPostCalls != 1 {
		t.Fatalf("expected CreatePost to run once, got %d", fake.createPostCalls)
	}
	if fake.lastPost.Kind != post.KindText || fake.lastPost.Forum.ID != "f1" {
		t.Errorf("unexpected post sent: %+v", fake.lastPost)
	}

	result, ok := msg.(submittedMsg)
	if !ok {
		t.Fatalf("unexpected result message: %#v", msg)
	}

	// A successful submit closes the window and asks for a refresh.
	_, cmd := m.Update(result)
	if m.submitting || c.IsLoading() {
		t.Error("submitting/loading should be cleared once the result is in")
	}
	out := collect(cmd)
	var closed, reload, notice bool
	for _, o := range out {
		switch o.(type) {
		case msgs.CloseWindow:
			closed = true
		case msgs.ReloadPost:
			reload = true
		case msgs.Notice:
			notice = true
		}
	}
	if !closed || !reload || !notice {
		t.Errorf("expected close, reload and notice after a successful post, got %#v", out)
	}
	if m.textarea.Value() != "" {
		t.Error("the composer should be reset after a successful post")
	}
}

func TestLinkBodyMakesALinkPost(t *testing.T) {
	fake := &fakeSystem{}
	m, _ := testModel(t, fake)
	openPost(m)
	m.textinput.SetValue("A link")
	m.textarea.SetValue(" https://example.com/article \n")

	_, cmds := m.handleSubmit()
	cmds[0]()

	if fake.lastPost.Kind != post.KindLink {
		t.Errorf("a single URL body must produce a link post, got kind %v", fake.lastPost.Kind)
	}
	if fake.lastPost.Body != "https://example.com/article" {
		t.Errorf("body should be trimmed, got %q", fake.lastPost.Body)
	}
}

func TestEmptySubmitIsRejectedWithoutPosting(t *testing.T) {
	fake := &fakeSystem{}
	m, _ := testModel(t, fake)
	openPost(m)

	_, cmds := m.handleSubmit()
	if len(cmds) != 1 {
		t.Fatalf("expected a validation message, got %d commands", len(cmds))
	}
	if _, ok := cmds[0]().(msgs.ShowError); !ok {
		t.Errorf("expected an error message, got %T", cmds[0]())
	}
	if fake.createPostCalls != 0 || m.submitting {
		t.Error("an empty post must not be sent")
	}
}

// TestFailedSubmitKeepsTheWindowAndText makes sure a rejected post does not
// throw away what the user typed.
func TestFailedSubmitKeepsTheWindowAndText(t *testing.T) {
	fake := &fakeSystem{err: errors.New("board said no")}
	m, c := testModel(t, fake)
	m.Update(msgs.Compose{Action: msgs.ComposeReply, Post: post.Post{ID: "p1", SysIDX: 0}})
	m.textarea.SetValue("my reply text")

	_, cmds := m.handleSubmit()
	if fake.createReplyCalls != 0 {
		t.Fatal("CreateReply ran on the event loop")
	}

	result := cmds[0]()
	if fake.createReplyCalls != 1 {
		t.Fatalf("expected CreateReply to run once, got %d", fake.createReplyCalls)
	}
	if fake.lastReply.PostID != "p1" || fake.lastReply.ParentID != "" {
		t.Errorf("reply to a post must target the post: %+v", fake.lastReply)
	}

	_, cmd := m.Update(result)
	if m.submitting || c.IsLoading() {
		t.Error("submitting/loading should be cleared even on failure")
	}
	out := collect(cmd)
	if len(out) != 1 {
		t.Fatalf("expected only an error message, got %#v", out)
	}
	if _, ok := out[0].(msgs.ShowError); !ok {
		t.Errorf("expected an error to be surfaced, got %T", out[0])
	}
	if m.textarea.Value() != "my reply text" {
		t.Error("the typed reply must survive a failed submit")
	}
}

func TestReplyToReplyTargetsTheParent(t *testing.T) {
	fake := &fakeSystem{}
	m, _ := testModel(t, fake)
	m.Update(msgs.Compose{
		Action: msgs.ComposeReply,
		Post:   post.Post{ID: "p1", SysIDX: 0},
		Parent: &reply.Reply{ID: "c2", PostID: "p1", SysIDX: 0},
		Index:  2,
	})
	m.textarea.SetValue("nested")

	_, cmds := m.handleSubmit()
	cmds[0]()

	if fake.lastReply.PostID != "p1" || fake.lastReply.ParentID != "c2" {
		t.Errorf("reply to a reply must carry post and parent ids: %+v", fake.lastReply)
	}
	if fake.lastReply.Body != "nested" {
		t.Errorf("unexpected body %q", fake.lastReply.Body)
	}
}

// TestSubmitTwiceOnlyPostsOnce guards the window while a submit is in flight,
// which matters now that the UI stays interactive during it.
func TestSubmitTwiceOnlyPostsOnce(t *testing.T) {
	fake := &fakeSystem{}
	m, _ := testModel(t, fake)
	openPost(m)
	m.textinput.SetValue("subject")
	m.textarea.SetValue("body")

	_, first := m.handleSubmit()
	_, second := m.handleSubmit()

	if len(first) != 1 {
		t.Fatalf("expected the first submit to schedule work, got %d", len(first))
	}
	if len(second) != 0 {
		t.Fatalf("a second submit scheduled another post: %d commands", len(second))
	}
}

func TestTabSwitchesFieldsOnlyForPosts(t *testing.T) {
	m, _ := testModel(t, &fakeSystem{})
	openPost(m)
	if m.inputFocused != 0 {
		t.Fatal("a new post starts in the subject field")
	}
	m.handleTab()
	if m.inputFocused != 1 {
		t.Error("tab should move to the body")
	}

	m.Update(msgs.Compose{Action: msgs.ComposeReply, Post: post.Post{ID: "p1"}})
	if m.inputFocused != 1 {
		t.Fatal("a reply starts in the body field")
	}
	m.handleTab()
	if m.inputFocused != 1 {
		t.Error("tab must not leave the body field for replies")
	}
}
