package lemmy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/lemmy/api"
)

type route struct {
	status int
	body   string
}

type fakeLemmy struct {
	mu           sync.Mutex
	fl           flavor
	routes       map[string]route
	requests     []string
	auth         []string
	nodeinfoAuth []string
	onPosts      func()
	onCheck      func()
	onQuery      func(url.Values)
}

func newFakeLemmy(t *testing.T, fl flavor, routes map[string]route) (*fakeLemmy, *httptest.Server) {
	t.Helper()

	f := &fakeLemmy{fl: fl, routes: routes}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path

		f.mu.Lock()
		if r.URL.Path == nodeinfoPath {
			f.nodeinfoAuth = append(f.nodeinfoAuth, r.Header.Get("Authorization"))
			f.mu.Unlock()
			if fl.nodeinfo == "" {
				http.NotFound(w, r)
				return
			}
			fl.serveNodeinfo(w)
			return
		}
		f.requests = append(f.requests, key)
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		if f.onQuery != nil {
			f.onQuery(r.URL.Query())
		}
		rt, ok := f.routes[key]
		onPosts, onCheck := f.onPosts, f.onCheck
		f.mu.Unlock()

		if key == fl.posts && onPosts != nil {
			onPosts()
		}
		if key == fl.validate && onCheck != nil {
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

func (f *fakeLemmy) nodeinfoCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.nodeinfoAuth)
}

func plainSystem(t *testing.T, sysURL string, credentials map[string]string) *System {
	t.Helper()

	sys := &System{
		settings: system.Settings{URL: sysURL, Credentials: credentials},
		logger:   slog.New(slog.DiscardHandler),
	}
	client, err := api.NewClient(api.Config{
		URL:   sysURL,
		Token: sys.token(),
		Read:  &http.Client{Timeout: 5 * time.Second},
		Write: &http.Client{Timeout: 5 * time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}
	sys.client = client
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

func eachFlavor(t *testing.T, test func(t *testing.T, fl flavor)) {
	t.Helper()

	for _, fl := range flavors(t) {
		t.Run(fl.name, func(t *testing.T) {
			test(t, fl)
		})
	}
}

func TestNewSendsTheTokenWithEveryRequest(t *testing.T) {
	eachFlavor(t, func(t *testing.T, fl flavor) {
		f, srv := newFakeLemmy(t, fl, map[string]route{
			fl.validate: {200, fl.sessionOK},
			fl.posts:    {200, fl.onePost(t)},
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
		if len(f.nodeinfoAuth) != 1 || f.nodeinfoAuth[0] != "" {
			t.Errorf("the nodeinfo was read %d times with Authorization %q", len(f.nodeinfoAuth), f.nodeinfoAuth)
		}
	})
}

func TestTheCheckRunsBesideTheRequest(t *testing.T) {
	eachFlavor(t, func(t *testing.T, fl flavor) {
		f, srv := newFakeLemmy(t, fl, map[string]route{
			fl.validate: {200, fl.sessionOK},
			fl.posts:    {200, fl.onePost(t)},
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
	})
}

func TestAnEndedSessionDropsTheListAndStopsRequests(t *testing.T) {
	eachFlavor(t, func(t *testing.T, fl flavor) {
		f, srv := newFakeLemmy(t, fl, map[string]route{
			fl.validate: {400, fl.notLoggedIn},
			fl.posts:    {200, fl.onePost(t)},
			fl.forums:   {200, fl.noForums()},
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
	})
}

func TestAFailedCheckDoesntEndTheSession(t *testing.T) {
	eachFlavor(t, func(t *testing.T, fl flavor) {
		f, srv := newFakeLemmy(t, fl, map[string]route{
			fl.validate: {503, `<html>maintenance</html>`},
			fl.posts:    {200, fl.onePost(t)},
		})

		sys := plainSystem(t, srv.URL, token("vera"))
		_, err := sys.ListPosts(context.Background(), "", system.OrderNew)
		if err == nil || errors.Is(err, system.ErrNeedsConnect) {
			t.Fatalf("got %v, want a plain error", err)
		}
		if !strings.Contains(err.Error(), "checking the session") {
			t.Errorf("got %v", err)
		}

		f.set(fl.validate, route{200, fl.sessionOK})
		posts, err := sys.ListPosts(context.Background(), "", system.OrderNew)
		if err != nil || len(posts) != 1 {
			t.Errorf("the next list call didn't check again: %d posts, %v", len(posts), err)
		}
	})
}

func TestAFailedDetectionIsReportedAsSuch(t *testing.T) {
	eachFlavor(t, func(t *testing.T, fl flavor) {
		fl.nodeinfo = ""
		f, srv := newFakeLemmy(t, fl, map[string]route{
			fl.validate: {200, fl.sessionOK},
			fl.posts:    {200, fl.onePost(t)},
		})

		sys := plainSystem(t, srv.URL, token("vera"))
		_, err := sys.ListPosts(context.Background(), "", system.OrderNew)
		if err == nil || !strings.HasPrefix(err.Error(), "reading the software version: ") ||
			errors.Is(err, system.ErrNeedsConnect) {
			t.Fatalf("got %v", err)
		}
		if f.count() != 0 {
			t.Errorf("made requests %v", f.requests)
		}
	})
}

func TestAnAuthErrorOnAWriteEndsTheSession(t *testing.T) {
	eachFlavor(t, func(t *testing.T, fl flavor) {
		f, srv := newFakeLemmy(t, fl, map[string]route{
			fl.createReply: {401, `{"error":"incorrect_login"}`},
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
			if strings.Contains(r, "/login") {
				t.Errorf("a login was attempted: %v", f.requests)
			}
		}
	})
}

func TestAUsernameWithoutATokenMakesNoRequest(t *testing.T) {
	eachFlavor(t, func(t *testing.T, fl flavor) {
		f, srv := newFakeLemmy(t, fl, map[string]route{})

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

		if f.count() != 0 || f.nodeinfoCount() != 0 {
			t.Errorf("made requests %v and read the nodeinfo %d times", f.requests, f.nodeinfoCount())
		}
		if !sys.Capabilities().Has(system.CapWrite) || sys.Description() != "Lemmy" {
			t.Error("a connection with a username should look like an account")
		}
	})
}

func TestAnAnonymousSystemDoesntCheck(t *testing.T) {
	eachFlavor(t, func(t *testing.T, fl flavor) {
		f, srv := newFakeLemmy(t, fl, map[string]route{fl.posts: {200, fl.onePost(t)}})

		sys, err := New(system.Env{Settings: system.Settings{URL: srv.URL}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := sys.ListPosts(context.Background(), "", system.OrderNew); err != nil {
			t.Fatal(err)
		}

		f.mu.Lock()
		defer f.mu.Unlock()
		if len(f.requests) != 1 || f.requests[0] != fl.posts || f.auth[0] != "" {
			t.Errorf("got requests %v with Authorization %v", f.requests, f.auth)
		}
	})
}

func TestConcurrentListCallsShareTheEnd(t *testing.T) {
	eachFlavor(t, func(t *testing.T, fl flavor) {
		f, srv := newFakeLemmy(t, fl, map[string]route{
			fl.validate: {400, fl.notLoggedIn},
			fl.posts:    {200, fl.onePost(t)},
			fl.forums:   {200, fl.noForums()},
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
		if n := f.nodeinfoCount(); n != 1 {
			t.Errorf("read the nodeinfo %d times", n)
		}
	})
}
