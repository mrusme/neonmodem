package lobsters

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/lobsters/api"
)

func fixture(t *testing.T, dir string, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestToPostLinkAndText(t *testing.T) {
	sys := &System{idx: 1, settings: system.Settings{URL: "https://lobste.rs"}}

	link := &api.StoryModel{
		ShortID: "abc", Title: "A link", URL: "https://example.com/a",
		SubmitterUser: "vera", Tags: []string{"go", "programming"},
		CommentCount: 3, CommentsURL: "https://lobste.rs/s/abc/a_link",
		CreatedAt: "2026-10-04T12:23:41.000-05:00",
	}
	p := sys.toPost(link)
	if p.Kind != post.KindLink || p.Body != "https://example.com/a" || p.Link != "https://example.com/a" {
		t.Errorf("link story: kind=%v body=%q link=%q", p.Kind, p.Body, p.Link)
	}
	if p.Forum.ID != "go" || p.Author.Name != "vera" || p.ReplyCount != 3 || p.SysIDX != 1 {
		t.Errorf("unexpected post: %+v", p)
	}
	if p.CreatedAt.IsZero() {
		t.Error("created time not parsed")
	}

	text := &api.StoryModel{
		ShortID: "def", Title: "A text story", Description: "<p>Hello <em>world</em></p>",
		SubmitterUser: "vera",
	}
	p = sys.toPost(text)
	if p.Kind != post.KindText || !strings.Contains(p.Body, "_world_") || p.Link != "" {
		t.Errorf("text story: kind=%v body=%q link=%q", p.Kind, p.Body, p.Link)
	}
	if p.Forum.ID != "" {
		t.Errorf("a story without tags must not panic and has no forum, got %q", p.Forum.ID)
	}
}

func TestBuildTreeFromFixture(t *testing.T) {
	var story api.StoryModel
	if err := json.Unmarshal(fixture(t, filepath.Join("api", "testdata"), "story.json"), &story); err != nil {
		t.Fatal(err)
	}

	sys := &System{idx: 0}
	tree := sys.buildTree(story.ShortID, story.Comments)

	if reply.Count(tree) != len(story.Comments) {
		t.Fatalf("tree holds %d comments, fixture has %d", reply.Count(tree), len(story.Comments))
	}
	topLevel := 0
	for _, c := range story.Comments {
		if c.ParentComment == "" {
			topLevel++
		}
	}
	if len(tree) != topLevel {
		t.Errorf("expected %d top-level replies, got %d", topLevel, len(tree))
	}

	nested := false
	var walk func(rs []reply.Reply, parent string)
	walk = func(rs []reply.Reply, parent string) {
		for _, r := range rs {
			if r.ParentID != parent {
				t.Errorf("reply %s has parent %q, expected %q", r.ID, r.ParentID, parent)
			}
			if r.PostID != story.ShortID {
				t.Errorf("reply %s has post %q", r.ID, r.PostID)
			}
			if len(r.Replies) > 0 {
				nested = true
			}
			walk(r.Replies, r.ID)
		}
	}
	walk(tree, "")
	if !nested {
		t.Error("expected nested replies in the fixture")
	}
}

type fakeLobsters struct {
	mu       sync.Mutex
	loggedIn bool
	comments []url.Values
	stories  []url.Values
	twoFA    bool
}

func newFakeLobsters(t *testing.T) (*fakeLobsters, *httptest.Server) {
	t.Helper()
	f := &fakeLobsters{}
	mux := http.NewServeMux()
	html := func(w http.ResponseWriter, body string) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(body))
	}
	const meta = `<meta name="csrf-token" content="TOKEN123">`

	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			html(w, string(fixture(t, "testdata", "login_page.html")))
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("parsing the form failed: %v", err)
		}
		if r.Form.Get("authenticity_token") == "" {
			html(w, `<html><body><div class="flash-error">missing token</div></body></html>`)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.twoFA {
			http.Redirect(w, r, "/login/2fa", http.StatusFound)
			return
		}
		if r.Form.Get("email") != "test" || r.Form.Get("password") != "test" {
			html(w, `<html><head>`+meta+`</head><body><div class="flash-error">Invalid e-mail address and/or password.</div></body></html>`)
			return
		}
		f.loggedIn = true
		html(w, `<html><head>`+meta+`</head><body><form action="/logout" method="post"><button>Logout</button></form></body></html>`)
	})
	mux.HandleFunc("/login/2fa", func(w http.ResponseWriter, r *http.Request) {
		html(w, `<html><body>Enter your TOTP code</body></html>`)
	})
	mux.HandleFunc("/s/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/story_title") {
			html(w, `<html><head>`+meta+`</head><body>story</body></html>`)
			return
		}
		html(w, string(fixture(t, "testdata", "story_page.html")))
	})
	mux.HandleFunc("/stories/new", func(w http.ResponseWriter, r *http.Request) {
		html(w, `<html><head>`+meta+`</head><body><form></form></body></html>`)
	})
	mux.HandleFunc("/stories", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parsing the form failed: %v", err)
		}
		f.mu.Lock()
		f.stories = append(f.stories, r.Form)
		f.mu.Unlock()
		if r.Form.Get("story[title]") == "" {
			html(w, `<html><body><div class="flash-error">Title is too short</div></body></html>`)
			return
		}
		http.Redirect(w, r, "/s/newid1/story_title", http.StatusFound)
	})
	mux.HandleFunc("/comments", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parsing the form failed: %v", err)
		}
		f.mu.Lock()
		f.comments = append(f.comments, r.Form)
		f.mu.Unlock()
		parent := r.Form.Get("parent_comment_short_id")
		if parent != "" {
			html(w, `<div class="comment" data-shortid="`+parent+`"></div><div class="comment" data-shortid="child77"></div>`)
			return
		}
		html(w, `<div class="comment" data-shortid="top55"></div>`)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return f, srv
}

func testWeb(t *testing.T, srv *httptest.Server, pass string) *webSession {
	t.Helper()
	w, err := newWebSession(srv.URL, "test", pass, "", slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func TestCSRFTokenFromFixture(t *testing.T) {
	_, srv := newFakeLobsters(t)
	w := testWeb(t, srv, "test")

	token, err := w.csrfToken(context.Background(), "/login")
	if err != nil {
		t.Fatalf("csrfToken: %v", err)
	}
	if token == "" {
		t.Fatal("empty token")
	}
}

func TestLoginAndComment(t *testing.T) {
	f, srv := newFakeLobsters(t)
	w := testWeb(t, srv, "test")

	id, err := w.PostComment(context.Background(), "abc123", "", "hello")
	if err != nil {
		t.Fatalf("PostComment: %v", err)
	}
	if id != "top55" {
		t.Errorf("expected top55, got %q", id)
	}

	nested, err := w.PostComment(context.Background(), "abc123", "top55", "nested")
	if err != nil {
		t.Fatalf("nested PostComment: %v", err)
	}
	if nested != "child77" {
		t.Errorf("expected the child id, got %q", nested)
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.loggedIn {
		t.Fatal("never logged in")
	}
	if len(f.comments) != 2 {
		t.Fatalf("expected 2 comment posts, got %d", len(f.comments))
	}
	first := f.comments[0]
	if first.Get("story_id") != "abc123" || first.Get("comment") != "hello" || first.Get("authenticity_token") == "" {
		t.Errorf("unexpected comment form: %v", first)
	}
	if f.comments[1].Get("parent_comment_short_id") != "top55" {
		t.Errorf("nested comment did not carry the parent: %v", f.comments[1])
	}
}

func TestBadLoginAndTwoFactor(t *testing.T) {
	f, srv := newFakeLobsters(t)

	bad := testWeb(t, srv, "wrong")
	if err := bad.Verify(context.Background()); !errors.Is(err, ErrBadLogin) {
		t.Fatalf("expected ErrBadLogin, got %v", err)
	}

	f.mu.Lock()
	f.twoFA = true
	f.mu.Unlock()
	twofa := testWeb(t, srv, "test")
	if err := twofa.Verify(context.Background()); !errors.Is(err, ErrTwoFactor) {
		t.Fatalf("expected ErrTwoFactor, got %v", err)
	}
}

func TestSubmitStory(t *testing.T) {
	f, srv := newFakeLobsters(t)
	w := testWeb(t, srv, "test")

	id, err := w.SubmitStory(context.Background(), "A title", "https://example.com/x", true, "programming")
	if err != nil {
		t.Fatalf("SubmitStory: %v", err)
	}
	if id != "newid1" {
		t.Errorf("expected newid1, got %q", id)
	}

	f.mu.Lock()
	form := f.stories[0]
	f.mu.Unlock()
	if form.Get("story[url]") != "https://example.com/x" || form.Get("story[title]") != "A title" {
		t.Errorf("unexpected story form: %v", form)
	}
	if got := form["story[tags][]"]; len(got) != 1 || got[0] != "programming" {
		t.Errorf("tags not sent: %v", form)
	}

	if _, err := w.SubmitStory(context.Background(), "", "body text", false, "programming"); err == nil || !strings.Contains(err.Error(), "too short") {
		t.Errorf("expected the flash error to be reported, got %v", err)
	}
}

func TestCreatedCommentIDSkipsParent(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(
		`<div class="comment" data-shortid="p1"></div><div class="comment" data-shortid="c2"></div>`))
	if err != nil {
		t.Fatal(err)
	}
	if got := createdCommentID(doc, "p1"); got != "c2" {
		t.Errorf("expected c2, got %q", got)
	}
}

func TestAnonymousWritesAreRejected(t *testing.T) {
	sys := &System{settings: system.Settings{URL: "https://lobste.rs"}}
	if err := sys.CreateReply(context.Background(), &reply.Reply{PostID: "x", Body: "y"}); !errors.Is(err, system.ErrNoCredentials) {
		t.Errorf("expected ErrNoCredentials, got %v", err)
	}
	if sys.Capabilities().Has(system.CapWrite) {
		t.Error("anonymous lobsters reports write capabilities")
	}
}
