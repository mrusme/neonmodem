package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mrusme/neonmodem/internal/system/httpx"
)

type reply struct {
	status int
	body   string
}

type request struct {
	Method string
	Path   string
	Query  url.Values
	Body   map[string]any
	Auth   string
}

type fakeServer struct {
	mu       sync.Mutex
	routes   map[string][]reply
	served   map[string]int
	requests []request
	nodeinfo int
}

const routeNodeinfo = "GET " + nodeinfoPath

func newFakeServer(t *testing.T, version Version, routes map[string][]reply) (*fakeServer, string) {
	t.Helper()

	if _, served := routes[routeNodeinfo]; !served && version != 0 {
		routes[routeNodeinfo] = ok(fixture(t, fmt.Sprintf("v%d/nodeinfo.json", version)))
	}
	f := &fakeServer{routes: routes, served: map[string]int{}}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	return f, srv.URL
}

func (f *fakeServer) serve(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &body)
	}

	key := r.Method + " " + r.URL.Path
	f.mu.Lock()
	if key == routeNodeinfo {
		f.nodeinfo++
	} else {
		f.requests = append(f.requests, request{
			Method: r.Method,
			Path:   r.URL.Path,
			Query:  r.URL.Query(),
			Body:   body,
			Auth:   r.Header.Get("Authorization"),
		})
	}
	replies, ok := f.routes[key]
	n := f.served[key]
	f.served[key]++
	f.mu.Unlock()

	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	rp := replies[min(n, len(replies)-1)]
	if strings.HasPrefix(rp.body, "<") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(rp.status)
	fmt.Fprint(w, rp.body)
}

func (f *fakeServer) all() []request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]request(nil), f.requests...)
}

func (f *fakeServer) last(t *testing.T) request {
	t.Helper()

	requests := f.all()
	if len(requests) == 0 {
		t.Fatal("no request was made")
	}
	return requests[len(requests)-1]
}

func (f *fakeServer) nodeinfoRequests() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.nodeinfo
}

func (f *fakeServer) count(route string) int {
	n := 0
	for _, r := range f.all() {
		if r.Method+" "+r.Path == route {
			n++
		}
	}
	return n
}

func ok(body string) []reply {
	return []reply{{http.StatusOK, body}}
}

func fixture(t *testing.T, name string) string {
	t.Helper()

	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

type routes struct {
	login         string
	validate      string
	communities   string
	posts         string
	post          string
	comments      string
	createPost    string
	createComment string
}

type flavor struct {
	name    string
	version Version
	routes  routes
}

var flavors = []flavor{
	{
		name:    "v3",
		version: V3,
		routes: routes{
			login:         routeV3Login,
			validate:      routeV3Validate,
			communities:   routeV3Communities,
			posts:         routeV3Posts,
			post:          routeV3Post,
			comments:      routeV3Comments,
			createPost:    routeV3CreatePost,
			createComment: routeV3CreateComment,
		},
	},
	{
		name:    "v4",
		version: V4,
		routes: routes{
			login:         routeV4Login,
			validate:      routeV4Validate,
			communities:   routeV4Communities,
			posts:         routeV4Posts,
			post:          routeV4Post,
			comments:      routeV4Comments,
			createPost:    routeV4CreatePost,
			createComment: routeV4CreateComment,
		},
	},
}

func (fl flavor) fixtures(t *testing.T) map[string][]reply {
	t.Helper()

	return map[string][]reply{
		fl.routes.validate:      ok(`{"success":true}`),
		fl.routes.communities:   ok(fixture(t, fl.name+"/communities.json")),
		fl.routes.posts:         ok(fixture(t, fl.name+"/posts.json")),
		fl.routes.post:          ok(fixture(t, fl.name+"/post.json")),
		fl.routes.comments:      ok(fixture(t, fl.name+"/comments.json")),
		fl.routes.createPost:    ok(fixture(t, fl.name+"/create_post.json")),
		fl.routes.createComment: ok(fixture(t, fl.name+"/create_comment.json")),
	}
}

func newClient(t *testing.T, cfg Config) *Client {
	t.Helper()

	if cfg.Read == nil {
		cfg.Read = &http.Client{Timeout: 5 * time.Second}
	}
	if cfg.Write == nil {
		cfg.Write = &http.Client{Timeout: 5 * time.Second}
	}
	c, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func newTestClient(t *testing.T, sysURL string, token string) *Client {
	t.Helper()
	return newClient(t, Config{URL: sysURL, Token: token})
}

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		panic(err)
	}
	return t
}

func samePost(t *testing.T, got Post, want Post) {
	t.Helper()

	if !got.Published.Equal(want.Published) {
		t.Errorf("post %d published %v, want %v", want.ID, got.Published, want.Published)
	}
	got.Published, want.Published = time.Time{}, time.Time{}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func sameComments(t *testing.T, got []Comment, want []Comment) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("got %d comments, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if !got[i].Published.Equal(want[i].Published) {
			t.Errorf("comment %d published %v, want %v", want[i].ID, got[i].Published, want[i].Published)
		}
		g, w := got[i], want[i]
		g.Published, w.Published = time.Time{}, time.Time{}
		if g != w {
			t.Errorf("comment %d: got %+v, want %+v", i, g, w)
		}
	}
}

func second[T any](_ T, err error) error {
	return err
}

func TestNewClientNeedsAnAbsoluteURL(t *testing.T) {
	for _, sysURL := range []string{"lemmy.example", "/lemmy", ""} {
		if _, err := NewClient(Config{URL: sysURL}); err == nil {
			t.Errorf("%q was accepted", sysURL)
		}
	}
	if _, err := NewClient(Config{URL: "https://lemmy.example/"}); err != nil {
		t.Errorf("an absolute URL was rejected: %v", err)
	}
}

func TestEveryRequestSendsTheToken(t *testing.T) {
	for _, fl := range flavors {
		f, sysURL := newFakeServer(t, fl.version, fl.fixtures(t))
		c := newTestClient(t, sysURL, "session-token")
		ctx := context.Background()

		for i, err := range []error{
			c.ValidateAuth(ctx),
			second(c.Communities(ctx, ListingAll, 50, 1)),
			second(c.Posts(ctx, PostQuery{Listing: ListingAll, Limit: 50})),
			second(c.Post(ctx, 11)),
			second(c.Comments(ctx, 11, 8)),
			second(c.CreatePost(ctx, NewPost{CommunityID: 3, Name: "Name", Body: "Body"})),
			second(c.CreateComment(ctx, NewComment{PostID: 11, Content: "Content"})),
		} {
			if err != nil {
				t.Fatalf("%s call %d: %v", fl.name, i, err)
			}
		}

		requests := f.all()
		if len(requests) != 7 {
			t.Fatalf("%s made %d requests", fl.name, len(requests))
		}
		for _, r := range requests {
			if r.Auth != "Bearer session-token" {
				t.Errorf("%s %s %s sent Authorization %q", fl.name, r.Method, r.Path, r.Auth)
			}
		}
	}
}

func TestReadsWithoutATokenSendNoAuthorization(t *testing.T) {
	for _, fl := range flavors {
		f, sysURL := newFakeServer(t, fl.version, fl.fixtures(t))
		c := newTestClient(t, sysURL, "")
		ctx := context.Background()

		for i, err := range []error{
			second(c.Communities(ctx, ListingAll, 50, 1)),
			second(c.Posts(ctx, PostQuery{Listing: ListingAll, Limit: 50})),
			second(c.Post(ctx, 11)),
			second(c.Comments(ctx, 11, 8)),
		} {
			if err != nil {
				t.Fatalf("%s call %d: %v", fl.name, i, err)
			}
		}

		for _, r := range f.all() {
			if r.Auth != "" {
				t.Errorf("%s %s %s sent Authorization %q", fl.name, r.Method, r.Path, r.Auth)
			}
		}
	}
}

func TestCreatesAreNotRetriedAndListsAre(t *testing.T) {
	for _, fl := range flavors {
		unavailable := reply{http.StatusServiceUnavailable, `{"error":"unknown"}`}
		f, sysURL := newFakeServer(t, fl.version, map[string][]reply{
			fl.routes.posts:      {unavailable, {http.StatusOK, fixture(t, fl.name+"/posts.json")}},
			fl.routes.createPost: {unavailable, {http.StatusOK, fixture(t, fl.name+"/create_post.json")}},
		})
		c := newClient(t, Config{
			URL:   sysURL,
			Token: "session-token",
			Read:  httpx.NewHTTPClient(httpx.Options{Retries: 3}),
			Write: httpx.NewHTTPClient(httpx.Options{}),
		})

		if _, err := c.CreatePost(context.Background(), NewPost{CommunityID: 3, Name: "Name", Body: "Body"}); err == nil {
			t.Fatalf("%s: the create succeeded on its second attempt", fl.name)
		}
		if _, err := c.Posts(context.Background(), PostQuery{Listing: ListingAll, Limit: 50}); err != nil {
			t.Fatalf("%s: the list wasn't retried: %v", fl.name, err)
		}
		if f.count(fl.routes.createPost) != 1 || f.count(fl.routes.posts) != 2 {
			t.Errorf("%s made %d creates and %d lists, want one and two",
				fl.name, f.count(fl.routes.createPost), f.count(fl.routes.posts))
		}
	}
}
