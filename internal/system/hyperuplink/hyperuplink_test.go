package hyperuplink

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	topicID    = "0192b1c2-7d3e-7f40-8a51-6b7c8d9e0f10"
	forumID    = "0192b1c2-7d3e-7f40-8a51-000000000002"
	newTopicID = "0192b1c2-7d3e-7f40-8a51-00000000cafe"
	newReplyID = "0192b1c2-7d3e-7f40-8a51-00000000beef"
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

	status int
	code   string
	html   bool
}

func problem(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"type":"about:blank","title":%q,"status":%d,"code":%q}`,
		http.StatusText(status), status, code)
}

func (f *fixtureServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	req := request{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, auth: r.Header.Get("Authorization")}
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&req.body); err != nil {
			f.t.Errorf("decoding the %s body failed: %v", r.URL.Path, err)
		}
	}
	f.requests = append(f.requests, req)

	switch {
	case f.html:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("<html><body>Cannot GET " + r.URL.Path + "</body></html>"))
		return
	case f.status != 0:
		problem(w, f.status, f.code)
		return
	}

	page := r.URL.Query().Get("page")
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.URL.Path == "/api/v1/forums":
		w.Write([]byte(forumsJSON))
	case r.URL.Path == "/api/v1/topics" && r.Method == http.MethodGet:
		w.Write([]byte(topicsJSON))
	case r.URL.Path == "/api/v1/forums/"+forumID+"/topics":
		w.Write([]byte(topicsJSON))
	case r.URL.Path == "/api/v1/topics/"+topicID:
		w.Write([]byte(topicJSON))
	case r.URL.Path == "/api/v1/topics/"+topicID+"/replies" && r.Method == http.MethodGet && page == "1":
		w.Write([]byte(repliesPage1JSON))
	case r.URL.Path == "/api/v1/topics/"+topicID+"/replies" && r.Method == http.MethodGet && page == "2":
		w.Write([]byte(repliesPage2JSON))
	case r.URL.Path == "/api/v1/topics" && r.Method == http.MethodPost:
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"topic":{"id":"` + newTopicID + `","slug":"a-new-topic","name":"A new topic",` +
			`"url":"https://board.example/_general/chat/a-new-topic","attachments":[]}}`))
	case r.URL.Path == "/api/v1/topics/"+topicID+"/replies" && r.Method == http.MethodPost:
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"reply":{"id":"` + newReplyID + `","short_id":"beef","topic_id":"` + topicID + `","parent_id":"r1"}}`))
	default:
		problem(w, http.StatusNotFound, "err_not_found")
	}
}

func testSystem(t *testing.T, f *fixtureServer, token string) (system.System, string) {
	t.Helper()

	f.t = t
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)

	settings := system.Settings{URL: srv.URL}
	if token != "" {
		settings.Credentials = map[string]string{system.CredentialToken: token}
	}
	sys, err := New(system.Env{Index: 3, Settings: settings})
	if err != nil {
		t.Fatal(err)
	}
	return sys, srv.URL
}

const forumsJSON = `{"forums": [
  {"id": "` + forumID + `", "name": "Chat", "slug": "chat", "position": 1, "description": "Talk about anything",
   "category": {"id": "c1", "name": "General", "slug": "general"}, "topics": 2, "replies": 5,
   "last_activity_at": "2026-10-02T09:00:00Z", "permissions": {"read": true, "write": true, "moderate": false},
   "url": "https://board.example/_general/chat"},
  {"id": "f2", "name": "Help", "slug": "help", "position": 2, "description": "Ask for help",
   "category": {"id": "c1", "name": "General", "slug": "general"}, "topics": 0, "replies": 0,
   "last_activity_at": null, "permissions": {"read": true, "write": false, "moderate": false},
   "url": "https://board.example/_general/help"},
  {"id": "f3", "name": "Site", "slug": "site", "position": 1, "description": "About this board",
   "category": {"id": "c2", "name": "Meta", "slug": "meta"}, "topics": 1, "replies": 0,
   "last_activity_at": null, "permissions": {"read": true, "write": true, "moderate": false},
   "url": "https://board.example/_meta/site"}
]}`

const attachmentJSON = `{"id": "a1", "filename": "layout.png", "mime_type": "image/png",
  "created_at": "2026-10-01T10:00:00Z", "url": "/api/v1/attachments/a1"}`

const topicsJSON = `{
  "forum": {"id": "` + forumID + `", "name": "Chat", "slug": "chat", "category": {"id": "c1", "name": "General", "slug": "general"}},
  "topics": [
    {"id": "` + topicID + `", "short_id": "abc", "slug": "a-pinned-and-locked-topic", "name": "A pinned and locked topic",
     "kind": "regular", "pinned": true, "locked_at": "2026-10-02T10:00:00Z",
     "text": "The opening text.", "html": "<p>The opening text.</p>",
     "created_at": "2026-10-01T10:00:00Z", "last_activity_at": "2026-10-02T09:00:00Z", "replies": 3, "views": 12,
     "author": {"id": "u1", "username": "alice", "role": "user", "joined_at": "2026-01-01T00:00:00Z"},
     "forum": {"id": "` + forumID + `", "name": "Chat", "slug": "chat"}, "category": {"name": "General", "slug": "general"},
     "attachments": [` + attachmentJSON + `],
     "url": "https://board.example/_general/chat/a-pinned-and-locked-topic"},
    {"id": "0192b1c2-7d3e-7f40-8a51-000000000099", "short_id": "def", "slug": "an-open-topic", "name": "An open topic",
     "kind": "regular", "pinned": false, "locked_at": null,
     "text": "Second.", "html": "<p>Second.</p>",
     "created_at": "2026-10-03T10:00:00Z", "last_activity_at": "2026-10-03T10:00:00Z", "replies": 0, "views": 1,
     "author": {"id": "u2", "username": "bob", "role": "user", "joined_at": "2026-01-01T00:00:00Z"},
     "forum": {"id": "` + forumID + `", "name": "Chat", "slug": "chat"}, "category": {"name": "General", "slug": "general"},
     "attachments": [],
     "url": "https://board.example/_general/chat/an-open-topic"}
  ],
  "pagination": {"page": 1, "per_page": 10, "total": 2, "pages": 1}
}`

const topicJSON = `{"topic":
  {"id": "` + topicID + `", "short_id": "abc", "slug": "a-pinned-and-locked-topic", "name": "A pinned and locked topic",
   "kind": "poll", "pinned": true, "locked_at": "2026-10-02T10:00:00Z",
   "text": "The full opening text.\n", "html": "<p>The full opening text.</p>",
   "created_at": "2026-10-01T10:00:00Z", "last_activity_at": "2026-10-02T09:00:00Z", "replies": 3, "views": 12,
   "author": {"id": "u1", "username": "alice", "role": "user", "joined_at": "2026-01-01T00:00:00Z"},
   "forum": {"id": "` + forumID + `", "name": "Chat", "slug": "chat"}, "category": {"name": "General", "slug": "general"},
   "attachments": [` + attachmentJSON + `],
   "poll": {"options": [{"index": 0, "text": "QWERTY", "votes": 7, "percent": 58},
                        {"index": 1, "text": "Colemak", "votes": 4, "percent": 33},
                        {"index": 2, "text": "Other", "votes": 1, "percent": 8}],
            "total": 12, "ended": true, "ends_at": "2026-10-05T18:00:00Z",
            "viewer": {"can_vote": false, "has_voted": true, "selection": 0}},
   "viewer": {"unread": false},
   "url": "https://board.example/_general/chat/a-pinned-and-locked-topic"}
}`

const repliesPage1JSON = `{
  "replies": [
    {"id": "r1", "short_id": "r1", "topic_id": "` + topicID + `", "parent_id": null,
     "text": "First reply.", "html": "<p>First reply.</p>", "created_at": "2026-10-01T11:00:00Z",
     "author": {"id": "u2", "username": "bob", "role": "user", "joined_at": "2026-01-01T00:00:00Z"},
     "attachments": [], "url": "https://board.example/_general/chat/a-pinned-and-locked-topic#post-r1"},
    {"id": "r2", "short_id": "r2", "topic_id": "` + topicID + `", "parent_id": "r1",
     "text": "Answer to the first.", "html": "<p>Answer to the first.</p>", "created_at": "2026-10-01T12:00:00Z",
     "author": {"id": "u1", "username": "alice", "role": "user", "joined_at": "2026-01-01T00:00:00Z"},
     "attachments": [{"id": "a2", "filename": "notes.txt", "mime_type": "text/plain",
                      "created_at": "2026-10-01T12:00:00Z", "url": "/api/v1/attachments/a2"}],
     "url": "https://board.example/_general/chat/a-pinned-and-locked-topic#post-r2"}
  ],
  "pagination": {"page": 1, "per_page": 100, "total": 3, "pages": 2}
}`

const repliesPage2JSON = `{
  "replies": [
    {"id": "r3", "short_id": "r3", "topic_id": "` + topicID + `", "parent_id": "gone",
     "text": "Orphaned answer.", "html": "<p>Orphaned answer.</p>", "created_at": "2026-10-02T09:00:00Z",
     "author": {"id": "u3", "username": "carol", "role": "user", "joined_at": "2026-01-01T00:00:00Z"},
     "attachments": [], "url": "https://board.example/_general/chat/a-pinned-and-locked-topic#post-r3"}
  ],
  "pagination": {"page": 2, "per_page": 100, "total": 3, "pages": 2}
}`

func TestForumsAreNamedAfterTheirCategory(t *testing.T) {
	f := &fixtureServer{}
	sys, _ := testSystem(t, f, "hup_test")

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
	if f.requests[0].path != "/api/v1/forums" || f.requests[0].auth != "Bearer hup_test" {
		t.Errorf("the forums were requested as %+v", f.requests[0])
	}
}

func TestTopicsBecomePostsWithTheirIDsIntact(t *testing.T) {
	f := &fixtureServer{}
	sys, origin := testSystem(t, f, "hup_test")

	posts, err := sys.ListPosts(context.Background(), forumID, system.OrderActive)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 2 {
		t.Fatalf("got %d posts, expected 2", len(posts))
	}
	if f.requests[0].path != "/api/v1/forums/"+forumID+"/topics" ||
		!strings.Contains(f.requests[0].query, "sort=active") || !strings.Contains(f.requests[0].query, "page=1") {
		t.Errorf("the list request was %s?%s", f.requests[0].path, f.requests[0].query)
	}

	p := posts[0]
	if p.ID != topicID || p.Subject != "A pinned and locked topic" {
		t.Errorf("unexpected post %+v", p)
	}
	wantBody := "The opening text.\n\nAttachments:\n- ![layout.png (image/png)](" +
		origin + "/api/v1/attachments/a1)"
	if p.Body != wantBody {
		t.Errorf("the body is %q", p.Body)
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
	if p.URL != "https://board.example/_general/chat/a-pinned-and-locked-topic" {
		t.Errorf("URL is %q", p.URL)
	}
	if p.CreatedAt.IsZero() {
		t.Error("the creation time was not parsed")
	}
	if posts[1].Closed || posts[1].Pinned || posts[1].Body != "Second." {
		t.Errorf("an open topic without attachments is %+v", posts[1])
	}
}

func TestEveryOrderMapsToTheServersSort(t *testing.T) {
	f := &fixtureServer{}
	sys, _ := testSystem(t, f, "hup_test")

	for order, want := range map[system.Order]string{
		system.OrderNew:      "sort=new",
		system.OrderActive:   "sort=active",
		system.OrderComments: "sort=replies",
	} {
		f.requests = nil
		if _, err := sys.ListPosts(context.Background(), "", order); err != nil {
			t.Fatalf("%s: %v", order, err)
		}
		if f.requests[0].path != "/api/v1/topics" || !strings.Contains(f.requests[0].query, want) {
			t.Errorf("%s was requested as %s?%s", order, f.requests[0].path, f.requests[0].query)
		}
	}

	f.requests = nil
	if _, err := sys.ListPosts(context.Background(), "", system.OrderHot); !errors.Is(err, system.ErrOrderUnavailable) {
		t.Errorf("Hot gave %v", err)
	}
	if len(f.requests) != 0 {
		t.Errorf("an unavailable order made %d requests", len(f.requests))
	}
	ordering := sys.Orders("")
	if ordering.Default != system.OrderActive || len(ordering.Supported) != 3 {
		t.Errorf("the orders are %+v", ordering)
	}
}

func TestLoadPostCollectsEveryReplyPageIntoATree(t *testing.T) {
	f := &fixtureServer{}
	sys, origin := testSystem(t, f, "hup_test")

	p := &post.Post{ID: topicID, Subject: "A pinned and locked topic", SysIDX: 3}
	if err := sys.LoadPost(context.Background(), p); err != nil {
		t.Fatal(err)
	}

	if len(f.requests) != 3 {
		t.Fatalf("expected the topic and two reply pages, got %d requests", len(f.requests))
	}
	if f.requests[0].path != "/api/v1/topics/"+topicID ||
		f.requests[1].path != "/api/v1/topics/"+topicID+"/replies" ||
		f.requests[1].query != "page=1&per_page=100" || f.requests[2].query != "page=2&per_page=100" {
		t.Errorf("the pages were requested as %+v", f.requests)
	}

	wantBody := "The full opening text.\n\n" +
		"Poll, 12 votes, ended:\n- QWERTY: 7 votes (58%)\n- Colemak: 4 votes (33%)\n- Other: 1 vote (8%)\n\n" +
		"Attachments:\n- ![layout.png (image/png)](" + origin + "/api/v1/attachments/a1)"
	if p.Body != wantBody {
		t.Errorf("the body is %q", p.Body)
	}
	if !p.Pinned || !p.Closed || p.ReplyCount != 3 {
		t.Errorf("the post was not refreshed: %+v", p)
	}
	if len(p.Replies) != 2 {
		t.Fatalf("expected 2 top-level replies, got %d", len(p.Replies))
	}

	first := p.Replies[0]
	if first.ID != "r1" || first.PostID != topicID || first.Author.Name != "bob" || first.SysIDX != 3 ||
		first.Body != "First reply." {
		t.Errorf("the first reply is %+v", first)
	}
	if len(first.Replies) != 1 || first.Replies[0].ID != "r2" || first.Replies[0].ParentID != "r1" {
		t.Errorf("the answer is not nested under the first reply: %+v", first.Replies)
	}
	wantAnswer := "Answer to the first.\n\nAttachments:\n- notes.txt (text/plain): " +
		origin + "/api/v1/attachments/a2"
	if first.Replies[0].Body != wantAnswer {
		t.Errorf("the answer's body is %q", first.Replies[0].Body)
	}
	if orphan := p.Replies[1]; orphan.ID != "r3" || orphan.ParentID != "" || orphan.Author.Name != "carol" {
		t.Errorf("the reply whose parent is gone is %+v", orphan)
	}
}

func TestCreatePostAndReplyPostTheirBodiesAndKeepTheIDs(t *testing.T) {
	f := &fixtureServer{}
	sys, _ := testSystem(t, f, "hup_test")

	p := &post.Post{Subject: "A new topic", Body: "Some text", Forum: forum.Forum{ID: forumID}}
	if err := sys.CreatePost(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if p.ID != newTopicID || p.URL != "https://board.example/_general/chat/a-new-topic" {
		t.Errorf("the created post has id %q and URL %q", p.ID, p.URL)
	}
	created := f.requests[0]
	if created.method != http.MethodPost || created.path != "/api/v1/topics" || created.auth != "Bearer hup_test" {
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
	if r.ID != newReplyID {
		t.Errorf("the created reply has id %q", r.ID)
	}
	replied := f.requests[1]
	if replied.method != http.MethodPost || replied.path != "/api/v1/topics/"+topicID+"/replies" {
		t.Errorf("the reply was created with %+v", replied)
	}
	if replied.body["text"] != "An answer" || replied.body["parent_id"] != "r1" {
		t.Errorf("the reply body was %+v", replied.body)
	}

	top := &reply.Reply{PostID: topicID, Body: "A top-level answer"}
	if err := sys.CreateReply(context.Background(), top); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.requests[2].body["parent_id"]; ok {
		t.Errorf("a top-level reply sent a parent: %+v", f.requests[2].body)
	}
}

func TestGuestsReadWithoutAKeyAndCantPost(t *testing.T) {
	f := &fixtureServer{}
	sys, _ := testSystem(t, f, "")

	if sys.Description() != "Hyperuplink (read-only)" || sys.Capabilities() != system.CapRead {
		t.Errorf("a guest connection is described as %q with capabilities %08b",
			sys.Description(), sys.Capabilities())
	}

	if _, err := sys.ListForums(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.requests[0].auth != "" {
		t.Errorf("a guest sent %q", f.requests[0].auth)
	}

	err := sys.CreatePost(context.Background(), &post.Post{Subject: "x", Forum: forum.Forum{ID: forumID}})
	if !errors.Is(err, system.ErrNoCredentials) {
		t.Errorf("a guest post gave %v", err)
	}
	if err := sys.CreateReply(context.Background(), &reply.Reply{PostID: topicID}); !errors.Is(err, system.ErrNoCredentials) {
		t.Errorf("a guest reply gave %v", err)
	}
	if len(f.requests) != 1 {
		t.Errorf("a guest's writes reached the server: %+v", f.requests)
	}
}

func TestARejectedKeyEndsTheSession(t *testing.T) {
	f := &fixtureServer{status: http.StatusUnauthorized, code: "err_apikey_invalid"}
	sys, origin := testSystem(t, f, "hup_test")

	_, err := sys.ListForums(context.Background())
	if !errors.Is(err, system.ErrNeedsConnect) {
		t.Fatalf("got %v", err)
	}
	want := "the API key for 127.0.0.1 was rejected; connect again with " +
		"`neonmodem connect --type hyperuplink --url " + origin + "`"
	if err.Error() != want {
		t.Errorf("the error reads %q", err)
	}

	if _, again := sys.ListPosts(context.Background(), "", system.OrderActive); again == nil ||
		again.Error() != err.Error() {
		t.Errorf("the second call gave %v", again)
	}
	if err := sys.LoadPost(context.Background(), &post.Post{ID: topicID}); !errors.Is(err, system.ErrNeedsConnect) {
		t.Errorf("LoadPost gave %v", err)
	}
	if err := sys.CreatePost(context.Background(), &post.Post{}); !errors.Is(err, system.ErrNeedsConnect) {
		t.Errorf("CreatePost gave %v", err)
	}
	if len(f.requests) != 1 {
		t.Errorf("the ended session kept calling the server: %d requests", len(f.requests))
	}
}

func TestAClosedBoardEndsAGuestSession(t *testing.T) {
	f := &fixtureServer{status: http.StatusUnauthorized, code: "err_authentication_required"}
	sys, _ := testSystem(t, f, "")

	_, err := sys.ListPosts(context.Background(), "", system.OrderActive)
	if !errors.Is(err, system.ErrNeedsConnect) {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(err.Error(), "127.0.0.1 no longer serves guests; connect with an account using `neonmodem connect") {
		t.Errorf("the error reads %q", err)
	}
}

func TestServerProblemsNameTheField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"type":"about:blank","title":"Unprocessable Entity","status":422,"code":"validation",` +
			`"errors":{"name":["required","max"],"text":["required"]}}`))
	}))
	t.Cleanup(srv.Close)

	sys, err := New(system.Env{Settings: system.Settings{
		URL:         srv.URL,
		Credentials: map[string]string{system.CredentialToken: "hup_test"},
	}})
	if err != nil {
		t.Fatal(err)
	}

	err = sys.CreatePost(context.Background(), &post.Post{Subject: "x"})
	if err == nil || !strings.Contains(err.Error(), "validation (name: required, max; text: required)") {
		t.Errorf("the error is %v", err)
	}
	if errors.Is(err, system.ErrNeedsConnect) {
		t.Error("a validation problem must not end the session")
	}
}

func TestAnOlderBoardIsNamed(t *testing.T) {
	f := &fixtureServer{html: true}
	sys, _ := testSystem(t, f, "hup_test")

	_, err := sys.ListForums(context.Background())
	if err == nil || !strings.Contains(err.Error(), "the board may run a Hyperuplink older than its API v1") {
		t.Errorf("got %v", err)
	}
	if errors.Is(err, system.ErrNeedsConnect) {
		t.Error("an HTML answer must not end the session")
	}
}
