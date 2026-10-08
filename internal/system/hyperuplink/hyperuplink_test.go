package hyperuplink

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
)

const (
	topicID = "0192b1c2-7d3e-7f40-8a51-6b7c8d9e0f10"
	forumID = "0192b1c2-7d3e-7f40-8a51-000000000002"
)

type request struct {
	method string
	path   string
	query  string
	auth   string
	body   map[string]any
}

type fixtureServer struct {
	t        *testing.T
	requests []request
}

func (f *fixtureServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	req := request{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, auth: r.Header.Get("Authorization")}
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&req.body); err != nil {
			f.t.Errorf("decoding the %s body failed: %v", r.URL.Path, err)
		}
	}
	f.requests = append(f.requests, req)

	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.URL.Path == "/":
		w.Write([]byte(boardJSON))
	case r.URL.Path == "/topics":
		w.Write([]byte(topicsJSON))
	case r.URL.Path == "/topics/"+topicID && r.URL.Query().Get("page") == "1":
		w.Write([]byte(topicPage1JSON))
	case r.URL.Path == "/topics/"+topicID && r.URL.Query().Get("page") == "2":
		w.Write([]byte(topicPage2JSON))
	case r.URL.Path == "/topics/"+topicID+"/replies":
		w.Write([]byte(`{"id":"0192b1c2-7d3e-7f40-8a51-00000000beef","short_id":"beef"}`))
	case r.URL.Path == "/new":
		w.Write([]byte(`{"id":"0192b1c2-7d3e-7f40-8a51-00000000cafe","slug":"a-new-topic","category_slug":"general","forum_slug":"chat"}`))
	default:
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	}
}

func testSystem(t *testing.T) (system.System, *fixtureServer) {
	t.Helper()

	f := &fixtureServer{t: t}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)

	sys, err := New(system.Env{
		Index: 3,
		Settings: system.Settings{
			URL:         srv.URL,
			Credentials: map[string]string{system.CredentialToken: "hup_test"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return sys, f
}

const boardJSON = `{
  "categories_forums": [
    {"category": {"id": "c1", "name": "General", "slug": "general", "position": 1},
     "forums": [
       {"id": "` + forumID + `", "name": "Chat", "slug": "chat", "position": 1, "category_id": "c1",
        "description": "Talk about anything", "category_name": "General", "category_slug": "general",
        "topics": 2, "replies": 5},
       {"id": "f2", "name": "Help", "slug": "help", "position": 2, "category_id": "c1",
        "description": "Ask for help", "category_name": "General", "category_slug": "general",
        "topics": 0, "replies": 0}
     ]},
    {"category": {"id": "c2", "name": "Meta", "slug": "meta", "position": 2},
     "forums": [
       {"id": "f3", "name": "Site", "slug": "site", "position": 1, "category_id": "c2",
        "description": "About this board", "category_name": "Meta", "category_slug": "meta",
        "topics": 1, "replies": 0}
     ]}
  ],
  "recent_topics": []
}`

const topicsJSON = `{
  "forum": {"id": "` + forumID + `", "name": "Chat", "slug": "chat", "category_name": "General", "category_slug": "general"},
  "topics": [
    {"id": "` + topicID + `", "short_id": "abc", "name": "A pinned and locked topic", "slug": "a-pinned-and-locked-topic",
     "forum_id": "` + forumID + `", "author_id": "u1", "kind": "regular", "pinned": true,
     "text": "The opening text.", "html": "<p>The opening text.</p>",
     "created_at": "2026-10-01T10:00:00Z", "updated_at": "2026-10-01T10:00:00Z", "locked_at": "2026-10-02T10:00:00Z",
     "views": 12, "category_name": "General", "category_slug": "general", "forum_name": "Chat", "forum_slug": "chat",
     "author_username": "alice", "replies": 3, "last_reply_at": "2026-10-02T09:00:00Z"},
    {"id": "0192b1c2-7d3e-7f40-8a51-000000000099", "short_id": "def", "name": "An open topic", "slug": "an-open-topic",
     "forum_id": "` + forumID + `", "author_id": "u2", "kind": "regular", "pinned": false,
     "text": "Second.", "html": "<p>Second.</p>",
     "created_at": "2026-10-03T10:00:00Z", "updated_at": "2026-10-03T10:00:00Z",
     "views": 1, "category_name": "General", "category_slug": "general", "forum_name": "Chat", "forum_slug": "chat",
     "author_username": "bob", "replies": 0}
  ],
  "total": 2,
  "pages": 1
}`

const topicPage1JSON = `{
  "topic": {"id": "` + topicID + `", "short_id": "abc", "name": "A pinned and locked topic", "slug": "a-pinned-and-locked-topic",
    "forum_id": "` + forumID + `", "author_id": "u1", "kind": "regular", "pinned": true,
    "text": "The full opening text.", "html": "<p>The full opening text.</p>",
    "created_at": "2026-10-01T10:00:00Z", "locked_at": "2026-10-02T10:00:00Z",
    "category_name": "General", "category_slug": "general", "forum_name": "Chat", "forum_slug": "chat",
    "author_username": "alice", "replies": 3},
  "replies": [
    {"id": "r1", "short_id": "r1", "topic_id": "` + topicID + `", "reply_id": "", "author_id": "u2",
     "text": "First reply.", "created_at": "2026-10-01T11:00:00Z", "author_username": "bob"},
    {"id": "r2", "short_id": "r2", "topic_id": "` + topicID + `", "reply_id": "r1", "author_id": "u1",
     "text": "Answer to the first.", "created_at": "2026-10-01T12:00:00Z", "author_username": "alice"}
  ],
  "total": 3,
  "pages": 2
}`

const topicPage2JSON = `{
  "topic": {"id": "` + topicID + `", "name": "A pinned and locked topic", "text": "The full opening text.",
    "pinned": true, "locked_at": "2026-10-02T10:00:00Z"},
  "replies": [
    {"id": "r3", "short_id": "r3", "topic_id": "` + topicID + `", "reply_id": "", "author_id": "u3",
     "text": "", "created_at": "2026-10-02T09:00:00Z", "deleted_at": "2026-10-02T09:30:00Z", "author_username": "carol"}
  ],
  "total": 3,
  "pages": 2
}`

func TestForumsAreNamedAfterTheirCategory(t *testing.T) {
	sys, f := testSystem(t)

	forums, err := sys.ListForums(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	var names []string
	for _, forum := range forums {
		names = append(names, forum.Name)
		if forum.SysIDX != 3 {
			t.Errorf("%s belongs to system %d, expected 3", forum.Name, forum.SysIDX)
		}
	}
	if got := strings.Join(names, ", "); got != "General/Chat, General/Help, Meta/Site" {
		t.Errorf("forum names are %q", got)
	}
	if forums[0].ID != forumID || forums[0].Info != "Talk about anything" {
		t.Errorf("the first forum is %+v", forums[0])
	}
	if f.requests[0].auth != "Bearer hup_test" {
		t.Errorf("the token is sent as %q", f.requests[0].auth)
	}
}

func TestTopicsBecomePostsWithTheirIDsIntact(t *testing.T) {
	sys, f := testSystem(t)

	posts, err := sys.ListPosts(context.Background(), forumID, system.OrderActive)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 2 {
		t.Fatalf("got %d posts, expected 2", len(posts))
	}
	if f.requests[0].path != "/topics" || !strings.Contains(f.requests[0].query, "forum_id="+forumID) {
		t.Errorf("the list request was %s?%s", f.requests[0].path, f.requests[0].query)
	}

	p := posts[0]
	if p.ID != topicID || p.Subject != "A pinned and locked topic" || p.Body != "The opening text." {
		t.Errorf("unexpected post %+v", p)
	}
	if !p.Pinned || !p.Closed || p.ReplyCount != 3 || p.Kind != post.KindText {
		t.Errorf("flags are wrong: %+v", p)
	}
	if p.Author.Name != "alice" || p.Author.ID != "u1" {
		t.Errorf("author is %+v", p.Author)
	}
	if p.Forum.ID != forumID || p.Forum.Name != "General/Chat" || p.Forum.SysIDX != 3 || p.SysIDX != 3 {
		t.Errorf("forum is %+v", p.Forum)
	}
	if !strings.HasSuffix(p.URL, "/_general/chat/a-pinned-and-locked-topic") {
		t.Errorf("URL is %q", p.URL)
	}
	if p.CreatedAt.IsZero() {
		t.Error("the creation time was not parsed")
	}
	if posts[1].Closed || posts[1].Pinned {
		t.Error("an open topic is neither closed nor pinned")
	}
}

func TestLoadPostCollectsEveryReplyPageIntoATree(t *testing.T) {
	sys, f := testSystem(t)

	p := &post.Post{ID: topicID, Subject: "A pinned and locked topic", SysIDX: 3}
	if err := sys.LoadPost(context.Background(), p); err != nil {
		t.Fatal(err)
	}

	if len(f.requests) != 2 {
		t.Fatalf("expected two page requests, got %d", len(f.requests))
	}
	if f.requests[0].path != "/topics/"+topicID || f.requests[0].query != "page=1" || f.requests[1].query != "page=2" {
		t.Errorf("the pages were requested as %+v", f.requests)
	}

	if p.Body != "The full opening text." || !p.Pinned || !p.Closed || p.ReplyCount != 3 {
		t.Errorf("the post was not refreshed: %+v", p)
	}
	if len(p.Replies) != 2 {
		t.Fatalf("expected 2 top-level replies, got %d", len(p.Replies))
	}

	first := p.Replies[0]
	if first.ID != "r1" || first.PostID != topicID || first.Author.Name != "bob" || first.SysIDX != 3 {
		t.Errorf("the first reply is %+v", first)
	}
	if len(first.Replies) != 1 || first.Replies[0].ID != "r2" || first.Replies[0].ParentID != "r1" {
		t.Errorf("the answer is not nested under the first reply: %+v", first.Replies)
	}
	if deleted := p.Replies[1]; deleted.ID != "r3" || !deleted.Deleted {
		t.Errorf("the deleted reply from page two is %+v", deleted)
	}
}

func TestCreatePostAndReplyPostTheirBodiesAndKeepTheIDs(t *testing.T) {
	sys, f := testSystem(t)

	p := &post.Post{Subject: "A new topic", Body: "Some text", Forum: forum.Forum{ID: forumID}}
	if err := sys.CreatePost(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if p.ID != "0192b1c2-7d3e-7f40-8a51-00000000cafe" {
		t.Errorf("the created post has id %q", p.ID)
	}
	created := f.requests[0]
	if created.method != http.MethodPost || created.path != "/new" || created.auth != "Bearer hup_test" {
		t.Errorf("the post was created with %+v", created)
	}
	if created.body["name"] != "A new topic" || created.body["text"] != "Some text" ||
		created.body["forum_id"] != forumID || created.body["kind"] != "regular" {
		t.Errorf("the post body was %+v", created.body)
	}

	r := &reply.Reply{PostID: topicID, ParentID: "r1", Body: "An answer"}
	if err := sys.CreateReply(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if r.ID != "0192b1c2-7d3e-7f40-8a51-00000000beef" {
		t.Errorf("the created reply has id %q", r.ID)
	}
	replied := f.requests[1]
	if replied.method != http.MethodPost || replied.path != "/topics/"+topicID+"/replies" {
		t.Errorf("the reply was created with %+v", replied)
	}
	if replied.body["text"] != "An answer" || replied.body["reply_id"] != "r1" {
		t.Errorf("the reply body was %+v", replied.body)
	}
}

func TestServerErrorsNameTheField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"error":"validation failed","fields":{"name":"is too short"}}`))
	}))
	t.Cleanup(srv.Close)

	sys, err := New(system.Env{Settings: system.Settings{URL: srv.URL}})
	if err != nil {
		t.Fatal(err)
	}

	err = sys.CreatePost(context.Background(), &post.Post{Subject: "x"})
	if err == nil || !strings.Contains(err.Error(), "validation failed (name: is too short)") {
		t.Errorf("the error is %v", err)
	}
}
