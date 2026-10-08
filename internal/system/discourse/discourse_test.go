package discourse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("api", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

type fakeDiscourse struct {
	mu         sync.Mutex
	lists      []string
	hotMissing bool
	created    []map[string]any
	catHits    int
	topicIDs   map[int]map[string]any
	stream     []int
}

func newFakeDiscourse(t *testing.T) (*fakeDiscourse, *httptest.Server) {
	t.Helper()
	f := &fakeDiscourse{topicIDs: map[int]map[string]any{}}

	var long map[string]any
	if err := json.Unmarshal(fixture(t, "topic_long.json"), &long); err != nil {
		t.Fatal(err)
	}
	stream := long["post_stream"].(map[string]any)["stream"].([]any)
	for _, id := range stream {
		f.stream = append(f.stream, int(id.(float64)))
	}
	for _, p := range long["post_stream"].(map[string]any)["posts"].([]any) {
		pm := p.(map[string]any)
		f.topicIDs[int(pm["id"].(float64))] = pm
	}

	mux := http.NewServeMux()
	serveJSON := func(w http.ResponseWriter, body []byte) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(body)
	}
	mux.HandleFunc("/categories.json", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.catHits++
		f.mu.Unlock()
		serveJSON(w, fixture(t, "categories.json"))
	})
	topicList := func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.lists = append(f.lists, r.URL.RequestURI())
		missing := f.hotMissing && strings.HasSuffix(r.URL.Path, "/hot.json")
		f.mu.Unlock()
		if missing {
			http.NotFound(w, r)
			return
		}
		serveJSON(w, fixture(t, "latest.json"))
	}
	for _, list := range []string{"latest", "hot", "top"} {
		mux.HandleFunc("/"+list+".json", topicList)
		mux.HandleFunc("/c/testing/5/l/"+list+".json", topicList)
		mux.HandleFunc("/c/testing/nested/6/l/"+list+".json", topicList)
	}
	mux.HandleFunc("/t/9.json", func(w http.ResponseWriter, r *http.Request) {
		serveJSON(w, fixture(t, "topic_long.json"))
	})
	mux.HandleFunc("/t/10.json", func(w http.ResponseWriter, r *http.Request) {
		serveJSON(w, fixture(t, "topic_short.json"))
	})
	mux.HandleFunc("/t/9/posts.json", func(w http.ResponseWriter, r *http.Request) {
		var posts []map[string]any
		for _, raw := range r.URL.Query()["post_ids[]"] {
			id, _ := strconv.Atoi(raw)
			if pm, ok := f.topicIDs[id]; ok {
				posts = append(posts, pm)
				continue
			}
			number := 0
			for i, sid := range f.stream {
				if sid == id {
					number = i + 1
				}
			}
			posts = append(posts, map[string]any{
				"id": id, "post_number": number, "username": "synth", "name": nil,
				"cooked":     fmt.Sprintf("<p>synthesized reply %d</p>", number),
				"created_at": "2026-10-04T17:35:34.000Z", "user_id": 9,
				"reply_to_post_number": nil,
			})
		}
		body, _ := json.Marshal(map[string]any{"post_stream": map[string]any{"posts": posts}})
		serveJSON(w, body)
	})
	mux.HandleFunc("/posts/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/posts/"), ".json")
		serveJSON(w, []byte(fmt.Sprintf(`{"id":%s,"post_number":7,"topic_id":9}`, id)))
	})
	mux.HandleFunc("/posts.json", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Api-Key") == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"errors":["You are not permitted to view the requested resource."]}`))
			return
		}
		body, _ := io.ReadAll(r.Body)
		var m map[string]any
		if err := json.Unmarshal(body, &m); err != nil {
			t.Errorf("decoding the created post failed: %v", err)
		}
		f.mu.Lock()
		f.created = append(f.created, m)
		f.mu.Unlock()
		serveJSON(w, []byte(`{"id":99,"post_number":26,"topic_id":77}`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return f, srv
}

func testSystem(t *testing.T, srv *httptest.Server, creds map[string]string) *System {
	t.Helper()
	sys, err := New(system.Env{
		Index:    1,
		Settings: system.Settings{URL: srv.URL, Credentials: creds},
		Logger:   slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatal(err)
	}
	return sys.(*System)
}

func TestListForumsFlattensWithParentNames(t *testing.T) {
	_, srv := newFakeDiscourse(t)
	sys := testSystem(t, srv, nil)

	forums, err := sys.ListForums(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	for _, f := range forums {
		names[f.Name] = f.ID
		if f.SysIDX != 1 {
			t.Errorf("forum %s has SysIDX %d", f.Name, f.SysIDX)
		}
	}
	if names["Testing"] != "5" || names["Testing / Nested"] != "6" {
		t.Errorf("expected Testing (5) and Testing / Nested (6), got %v", names)
	}
}

func TestListPostsResolvesForumAndAuthor(t *testing.T) {
	f, srv := newFakeDiscourse(t)
	sys := testSystem(t, srv, nil)

	posts, err := sys.ListPosts(context.Background(), "", system.OrderNew)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) == 0 {
		t.Fatal("no posts")
	}
	byID := map[string]post.Post{}
	for _, p := range posts {
		byID[p.ID] = p
		if p.Author.Name == "" {
			t.Errorf("post %s has no author name (username fallback missing)", p.ID)
		}
		if p.Forum.Name == "" {
			t.Errorf("post %s has no forum name", p.ID)
		}
	}
	long := byID["9"]
	if long.ReplyCount != 24 || long.Forum.ID != "5" || long.Forum.Name != "Testing" {
		t.Errorf("unexpected long topic: %+v", long)
	}

	if _, err := sys.ListPosts(context.Background(), "6", system.OrderNew); err != nil {
		t.Fatalf("subcategory list should use the slug path: %v", err)
	}

	f.mu.Lock()
	hits := f.catHits
	f.mu.Unlock()
	if hits != 1 {
		t.Errorf("categories should be fetched once and cached, got %d fetches", hits)
	}
}

func TestLoadPostPagesTheLatestWindowFirst(t *testing.T) {
	_, srv := newFakeDiscourse(t)
	sys := testSystem(t, srv, nil)

	p := post.Post{ID: "9", SysIDX: 1}
	if err := sys.LoadPost(context.Background(), &p); err != nil {
		t.Fatal(err)
	}
	if p.ReplyPage != (post.ReplyPage{Offset: 4, Size: 20, Total: 24}) {
		t.Fatalf("unexpected page: %+v", p.ReplyPage)
	}
	if !p.ReplyPage.HasOlder() || p.ReplyCount != 24 {
		t.Errorf("expected older replies available and count 24: %+v", p)
	}
	if reply.Count(p.Replies) != 20 {
		t.Errorf("expected the 20 newest replies, got %d", reply.Count(p.Replies))
	}
	if p.Body == "" || p.Author.Name != "admin" {
		t.Errorf("first post body/author not applied: %q %+v", p.Body, p.Author)
	}

	threaded := false
	var walk func(rs []reply.Reply)
	walk = func(rs []reply.Reply) {
		for _, r := range rs {
			if r.PostID != "9" {
				t.Errorf("reply %s points at post %q", r.ID, r.PostID)
			}
			if len(r.Replies) > 0 {
				threaded = true
				for _, c := range r.Replies {
					if c.ParentID != r.ID {
						t.Errorf("child %s has parent %q, expected %s", c.ID, c.ParentID, r.ID)
					}
				}
			}
			walk(r.Replies)
		}
	}
	walk(p.Replies)
	if !threaded {
		t.Error("expected reply_to_post_number threading in the long topic")
	}

	p.ReplyPage.Offset -= p.ReplyPage.Size
	if err := sys.LoadPost(context.Background(), &p); err != nil {
		t.Fatal(err)
	}
	if p.ReplyPage.Offset != 0 || p.ReplyPage.HasOlder() {
		t.Errorf("older load should land on offset 0: %+v", p.ReplyPage)
	}
	if reply.Count(p.Replies) != 20 {
		t.Errorf("older window should hold 20 replies, got %d", reply.Count(p.Replies))
	}
}

func TestLoadPostShortTopic(t *testing.T) {
	_, srv := newFakeDiscourse(t)
	sys := testSystem(t, srv, nil)

	p := post.Post{ID: "10", SysIDX: 1}
	if err := sys.LoadPost(context.Background(), &p); err != nil {
		t.Fatal(err)
	}
	if p.ReplyPage.HasOlder() || p.ReplyCount != 2 {
		t.Errorf("short topic should be complete: %+v", p.ReplyPage)
	}
	if len(p.Replies) != 1 || len(p.Replies[0].Replies) != 1 {
		t.Fatalf("expected one reply with one nested answer, got %+v", p.Replies)
	}
}

func TestCreateReplyResolvesParentNumber(t *testing.T) {
	f, srv := newFakeDiscourse(t)
	sys := testSystem(t, srv, map[string]string{"key": "k", "client_id": "c"})

	r := reply.Reply{PostID: "9", ParentID: "4040", Body: "nested"}
	if err := sys.CreateReply(context.Background(), &r); err != nil {
		t.Fatal(err)
	}
	if r.ID != "99" {
		t.Errorf("expected the created post id, got %q", r.ID)
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.created) != 1 {
		t.Fatalf("expected one create call, got %d", len(f.created))
	}
	body := f.created[0]
	if body["topic_id"] != float64(9) || body["reply_to_post_number"] != float64(7) || body["raw"] != "nested" {
		t.Errorf("unexpected create body: %v", body)
	}
}

func TestCreatePostUsesTopicID(t *testing.T) {
	f, srv := newFakeDiscourse(t)
	sys := testSystem(t, srv, map[string]string{"key": "k", "client_id": "c"})

	p := post.Post{Subject: "Hi", Body: "Body"}
	p.Forum.ID = "5"
	if err := sys.CreatePost(context.Background(), &p); err != nil {
		t.Fatal(err)
	}
	if p.ID != "77" {
		t.Errorf("expected topic id 77, got %q", p.ID)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.created[0]["category"] != float64(5) || f.created[0]["title"] != "Hi" {
		t.Errorf("unexpected create body: %v", f.created[0])
	}
}

func TestAnonymousWritesAreRejectedLocally(t *testing.T) {
	_, srv := newFakeDiscourse(t)
	sys := testSystem(t, srv, nil)

	if sys.Capabilities().Has(system.CapWrite) {
		t.Error("anonymous discourse reports write capabilities")
	}
	err := sys.CreatePost(context.Background(), &post.Post{Subject: "x", Body: "y"})
	if !errors.Is(err, system.ErrNoCredentials) {
		t.Errorf("expected ErrNoCredentials, got %v", err)
	}
	if !strings.Contains(sys.Description(), "read-only") {
		t.Errorf("description should say read-only: %q", sys.Description())
	}
}
