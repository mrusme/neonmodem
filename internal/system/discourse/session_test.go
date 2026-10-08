package discourse

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/httpx"
	"github.com/mrusme/neonmodem/internal/system/systemtest"
)

var account = map[string]string{
	system.CredentialUsername: "vera",
	system.CredentialKey:      "key",
	system.CredentialClientID: "client",
}

type forbidding struct {
	mu       sync.Mutex
	paths    []string
	validKey bool
}

func (f *forbidding) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.paths = append(f.paths, r.URL.Path)
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if r.URL.Path == "/session/current.json" && f.validKey {
		w.Write([]byte(`{"current_user":{"id":1,"username":"vera"}}`))
		return
	}
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(`{"errors":["You are not permitted to view the requested resource."],"error_type":"invalid_access"}`))
}

func (f *forbidding) requested(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, p := range f.paths {
		if p == path {
			n++
		}
	}
	return n
}

func forbiddingServer(t *testing.T, validKey bool) (*forbidding, *httptest.Server) {
	t.Helper()
	f := &forbidding{validKey: validKey}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, srv
}

func TestARevokedKeyNamesTheConnectCommandAndEndsTheSession(t *testing.T) {
	f, srv := forbiddingServer(t, false)
	sys := testSystem(t, srv, account)

	_, err := sys.ListPosts(context.Background(), "", system.OrderNew)
	if !errors.Is(err, system.ErrNeedsConnect) {
		t.Fatalf("got %v, expected an error that needs a new connect", err)
	}
	if !strings.Contains(err.Error(), "neonmodem connect --type discourse --url "+srv.URL) {
		t.Errorf("the error doesn't name the command: %v", err)
	}
	if f.requested("/session/current.json") != 1 {
		t.Errorf("the key check ran %d times", f.requested("/session/current.json"))
	}

	if _, again := sys.ListForums(context.Background()); !errors.Is(again, system.ErrNeedsConnect) {
		t.Errorf("the ended session should answer every later call, got %v", again)
	}
	if n := f.requested("/categories.json"); n != 1 {
		t.Errorf("an ended session must not request anything, categories were requested %d times", n)
	}
}

func TestARestrictedResourceKeepsItsError(t *testing.T) {
	f, srv := forbiddingServer(t, true)
	sys := testSystem(t, srv, account)

	_, err := sys.ListPosts(context.Background(), "", system.OrderNew)
	if errors.Is(err, system.ErrNeedsConnect) || httpx.StatusOf(err) != http.StatusForbidden {
		t.Fatalf("got %v, expected the original 403", err)
	}
	if f.requested("/session/current.json") != 1 {
		t.Errorf("the key check ran %d times", f.requested("/session/current.json"))
	}
}

func TestAnAnonymousConnectionNeverChecksTheKey(t *testing.T) {
	f, srv := forbiddingServer(t, false)
	sys := testSystem(t, srv, nil)

	_, err := sys.ListPosts(context.Background(), "", system.OrderNew)
	if errors.Is(err, system.ErrNeedsConnect) || httpx.StatusOf(err) != http.StatusForbidden {
		t.Fatalf("got %v, expected the original 403", err)
	}
	if f.requested("/session/current.json") != 0 {
		t.Error("an anonymous connection has no key to check")
	}
}

func TestConnectVerifiesTheKey(t *testing.T) {
	_, accepting := forbiddingServer(t, true)
	_, rejecting := forbiddingServer(t, false)
	sys := testSystem(t, accepting, nil)

	if err := sys.verifyKey(context.Background(), accepting.URL, "client", "key"); err != nil {
		t.Errorf("an accepted key gives no error, got %v", err)
	}
	err := sys.verifyKey(context.Background(), rejecting.URL, "client", "key")
	if err == nil || !strings.Contains(err.Error(), "rejected the user API key") {
		t.Errorf("a rejected key names the rejection, got %v", err)
	}

	sys.readTimeout = 50 * time.Millisecond
	err = sys.verifyKey(context.Background(), systemtest.Blocking(t), "client", "key")
	if err == nil || !strings.Contains(err.Error(), "didn't answer within 0.05 seconds") {
		t.Errorf("a silent site gives the timeout wording, got %v", err)
	}
}
