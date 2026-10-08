package lemmy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
	"go.elara.ws/go-lemmy"
)

const (
	validatePath = "GET /api/v3/user/validate_auth"
	postsPath    = "GET /api/v3/post/list"
	forumsPath   = "GET /api/v3/community/list"
	commentPath  = "POST /api/v3/comment"

	sessionOK   = `{"success":true}`
	notLoggedIn = `{"error":"not_logged_in"}`
	onePost     = `{"posts":[{
		"post":{"id":7,"name":"Linux 7.3 released","creator_id":2,"community_id":3,
			"published":"2026-10-07T08:00:00Z"},
		"creator":{"name":"kernelfan"},
		"community":{"name":"linux"},
		"counts":{"comments":3,"score":12}}]}`
)

type route struct {
	status int
	body   string
}

type fakeLemmy struct {
	mu       sync.Mutex
	routes   map[string]route
	requests []string
	auth     []string
	onPosts  func()
	onCheck  func()
}

func newFakeLemmy(t *testing.T, routes map[string]route) (*fakeLemmy, *httptest.Server) {
	t.Helper()

	f := &fakeLemmy{routes: routes}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path

		f.mu.Lock()
		f.requests = append(f.requests, key)
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		rt, ok := f.routes[key]
		onPosts, onCheck := f.onPosts, f.onCheck
		f.mu.Unlock()

		if key == postsPath && onPosts != nil {
			onPosts()
		}
		if key == validatePath && onCheck != nil {
			onCheck()
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(rt.status)
		fmt.Fprint(w, rt.body)
	}))
	t.Cleanup(srv.Close)

	return f, srv
}

func (f *fakeLemmy) set(key string, rt route) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.routes[key] = rt
}

func (f *fakeLemmy) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

func plainSystem(t *testing.T, sysURL string, credentials map[string]string) *System {
	t.Helper()

	client, err := lemmy.NewWithClient(sysURL, &http.Client{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	sys := &System{
		settings: system.Settings{URL: sysURL, Credentials: credentials},
		logger:   slog.New(slog.DiscardHandler),
		client:   client,
	}
	client.Token = sys.token()
	return sys
}

func token(user string) map[string]string {
	creds := map[string]string{system.CredentialToken: "session-token"}
	if user != "" {
		creds[system.CredentialUsername] = user
	}
	return creds
}

func needsConnect(t *testing.T, err error, want ...string) {
	t.Helper()

	if !errors.Is(err, system.ErrNeedsConnect) {
		t.Fatalf("got %v, want an error that needs neonmodem connect", err)
	}
	for _, w := range want {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("%q doesn't contain %q", err, w)
		}
	}
}

func TestNewSendsTheTokenWithEveryRequest(t *testing.T) {
	f, srv := newFakeLemmy(t, map[string]route{
		validatePath: {200, sessionOK},
		postsPath:    {200, onePost},
	})

	sys, err := New(system.Env{Settings: system.Settings{URL: srv.URL, Credentials: token("vera")}})
	if err != nil {
		t.Fatal(err)
	}
	posts, err := sys.ListPosts(context.Background(), "", system.OrderNew)
	if err != nil || len(posts) != 1 {
		t.Fatalf("got %d posts and %v", len(posts), err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.requests) != 2 {
		t.Fatalf("got requests %v", f.requests)
	}
	for i, auth := range f.auth {
		if auth != "Bearer session-token" {
			t.Errorf("%s sent Authorization %q", f.requests[i], auth)
		}
	}
}

func TestTheCheckRunsBesideTheRequest(t *testing.T) {
	f, srv := newFakeLemmy(t, map[string]route{
		validatePath: {200, sessionOK},
		postsPath:    {200, onePost},
	})

	listed := make(chan struct{})
	var once sync.Once
	var released atomic.Bool
	f.onPosts = func() { once.Do(func() { close(listed) }) }
	f.onCheck = func() {
		select {
		case <-listed:
			released.Store(true)
		case <-time.After(2 * time.Second):
		}
	}

	sys := plainSystem(t, srv.URL, token("vera"))
	if _, err := sys.ListPosts(context.Background(), "", system.OrderNew); err != nil {
		t.Fatal(err)
	}
	if !released.Load() {
		t.Error("the post list wasn't requested while the session check was running")
	}
}

func TestAnEndedSessionDropsTheListAndStopsRequests(t *testing.T) {
	f, srv := newFakeLemmy(t, map[string]route{
		validatePath: {400, notLoggedIn},
		postsPath:    {200, onePost},
		forumsPath:   {200, `{"communities":[]}`},
	})

	sys := plainSystem(t, srv.URL, token("vera"))
	posts, err := sys.ListPosts(context.Background(), "", system.OrderNew)
	if posts != nil {
		t.Errorf("the anonymous list was kept: %+v", posts)
	}
	needsConnect(t, err, "the session of vera has ended",
		"`neonmodem connect --type lemmy --url "+srv.URL+"`")

	before := f.count()
	_, err = sys.ListForums(context.Background())
	needsConnect(t, err)
	needsConnect(t, sys.LoadPost(context.Background(), &post.Post{ID: "7"}))
	needsConnect(t, sys.CreateReply(context.Background(), &reply.Reply{PostID: "7", Body: "x"}))
	if after := f.count(); after != before {
		t.Errorf("an ended session made %d more requests", after-before)
	}
}

func TestAFailedCheckDoesntEndTheSession(t *testing.T) {
	f, srv := newFakeLemmy(t, map[string]route{
		validatePath: {503, `<html>maintenance</html>`},
		postsPath:    {200, onePost},
	})

	sys := plainSystem(t, srv.URL, token("vera"))
	_, err := sys.ListPosts(context.Background(), "", system.OrderNew)
	if err == nil || errors.Is(err, system.ErrNeedsConnect) {
		t.Fatalf("got %v, want a plain error", err)
	}
	if !strings.Contains(err.Error(), "checking the session") {
		t.Errorf("got %v", err)
	}

	f.set(validatePath, route{200, sessionOK})
	posts, err := sys.ListPosts(context.Background(), "", system.OrderNew)
	if err != nil || len(posts) != 1 {
		t.Errorf("the next list call didn't check again: %d posts, %v", len(posts), err)
	}
}

func TestAnAuthErrorOnAWriteEndsTheSession(t *testing.T) {
	f, srv := newFakeLemmy(t, map[string]route{
		commentPath: {401, `{"error":"incorrect_login"}`},
	})

	sys := plainSystem(t, srv.URL, token(""))
	err := sys.CreateReply(context.Background(), &reply.Reply{PostID: "7", Body: "x"})
	needsConnect(t, err, "the session has ended; log in again with")

	before := f.count()
	_, err = sys.ListPosts(context.Background(), "", system.OrderNew)
	needsConnect(t, err)
	if f.count() != before {
		t.Error("the list call made a request after the session ended")
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.requests {
		if strings.Contains(r, "/user/login") {
			t.Errorf("a login was attempted: %v", f.requests)
		}
	}
}

func TestAUsernameWithoutATokenMakesNoRequest(t *testing.T) {
	f, srv := newFakeLemmy(t, map[string]route{})

	sys, err := New(system.Env{Settings: system.Settings{
		URL:         srv.URL,
		Credentials: map[string]string{system.CredentialUsername: "vera"},
	}})
	if err != nil {
		t.Fatal(err)
	}

	_, err = sys.ListPosts(context.Background(), "", system.OrderNew)
	needsConnect(t, err, "there's no session token for vera; log in with",
		"`neonmodem connect --type lemmy --url "+srv.URL+"`")
	_, err = sys.ListForums(context.Background())
	needsConnect(t, err)
	needsConnect(t, sys.LoadPost(context.Background(), &post.Post{ID: "7"}))
	needsConnect(t, sys.CreatePost(context.Background(), &post.Post{Forum: forum.Forum{ID: "3"}}))

	if f.count() != 0 {
		t.Errorf("made requests %v", f.requests)
	}
	if !sys.Capabilities().Has(system.CapWrite) || sys.Description() != "Lemmy" {
		t.Error("a connection with a username should look like an account")
	}
}

func TestAnAnonymousSystemDoesntCheck(t *testing.T) {
	f, srv := newFakeLemmy(t, map[string]route{postsPath: {200, onePost}})

	sys, err := New(system.Env{Settings: system.Settings{URL: srv.URL}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sys.ListPosts(context.Background(), "", system.OrderNew); err != nil {
		t.Fatal(err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.requests) != 1 || f.requests[0] != postsPath || f.auth[0] != "" {
		t.Errorf("got requests %v with Authorization %v", f.requests, f.auth)
	}
}

func TestConcurrentListCallsShareTheEnd(t *testing.T) {
	_, srv := newFakeLemmy(t, map[string]route{
		validatePath: {400, notLoggedIn},
		postsPath:    {200, onePost},
		forumsPath:   {200, `{"communities":[]}`},
	})

	sys := plainSystem(t, srv.URL, token("vera"))
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Go(func() {
			if i%2 == 0 {
				_, errs[i] = sys.ListPosts(context.Background(), "", system.OrderNew)
			} else {
				_, errs[i] = sys.ListForums(context.Background())
			}
		})
	}
	wg.Wait()

	for _, err := range errs {
		if !errors.Is(err, errs[0]) {
			t.Fatalf("calls ended with different errors: %v and %v", errs[0], err)
		}
	}
	needsConnect(t, errs[0])
}
