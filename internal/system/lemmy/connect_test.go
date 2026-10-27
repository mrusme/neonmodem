package lemmy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/prompt"
	"github.com/mrusme/neonmodem/internal/system/systemtest"
)

func pathOf(route string) string {
	_, path, _ := strings.Cut(route, " ")
	return path
}

func TestConnectLogsInThroughTheProxy(t *testing.T) {
	eachFlavor(t, func(t *testing.T, fl flavor) {
		var mu sync.Mutex
		var requests []string
		proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			requests = append(requests, r.Method+" "+r.URL.String())
			mu.Unlock()

			if r.URL.Path == nodeinfoPath {
				fl.serveNodeinfo(w)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if r.Method+" "+r.URL.Path != fl.login || !strings.Contains(string(body), `"password":"from-pass"`) {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"error":"incorrect_login"}`)
				return
			}
			fmt.Fprint(w, `{"jwt":"token","registration_created":false,"verify_email_sent":false}`)
		}))
		t.Cleanup(proxy.Close)

		sys, err := New(system.Env{Proxy: proxy.URL})
		if err != nil {
			t.Fatal(err)
		}
		term := prompt.NewTerminal(strings.NewReader("1\nvera\n2\npass show lemmy\n1\n\n"), io.Discard,
			systemtest.Commands{"pass show lemmy": "from-pass"})

		settings, err := sys.Connect(context.Background(), term, "http://lemmy.example")
		if err != nil {
			t.Fatalf("Connect: %v", err)
		}

		mu.Lock()
		defer mu.Unlock()
		want := []string{"GET http://lemmy.example" + nodeinfoPath, "POST http://lemmy.example" + pathOf(fl.login)}
		if !slices.Equal(requests, want) {
			t.Errorf("got requests %v through the proxy, want %v", requests, want)
		}

		wantCreds := map[string]string{"username": "vera", "token": "token"}
		if !maps.Equal(settings.Credentials, wantCreds) {
			t.Errorf("got %v, want %v", settings.Credentials, wantCreds)
		}
		if settings.Options[OptionListing] != ListingSubscribed {
			t.Errorf("got listing %q", settings.Options[OptionListing])
		}
	})
}

type loginServer struct {
	mu     sync.Mutex
	fl     flavor
	code   string
	failAs *route
	codes  []string
	stays  []bool
	logins int
}

func newLoginServer(t *testing.T, fl flavor, code string) (*loginServer, string) {
	t.Helper()

	s := &loginServer{fl: fl, code: code}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == nodeinfoPath {
			fl.serveNodeinfo(w)
			return
		}

		var body struct {
			Password     string  `json:"password"`
			Code         *string `json:"totp_2fa_token"`
			StayLoggedIn bool    `json:"stay_logged_in"`
		}
		if r.Method+" "+r.URL.Path != fl.login || json.NewDecoder(r.Body).Decode(&body) != nil {
			http.NotFound(w, r)
			return
		}

		sent := ""
		if body.Code != nil {
			sent = *body.Code
		}
		s.mu.Lock()
		s.logins++
		s.codes = append(s.codes, sent)
		s.stays = append(s.stays, body.StayLoggedIn)
		failAs := s.failAs
		s.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch {
		case failAs != nil:
			w.WriteHeader(failAs.status)
			fmt.Fprint(w, failAs.body)
		case body.Password != "secret":
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":"incorrect_login"}`)
		case s.code != "" && body.Code == nil:
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":"missing_totp_token"}`)
		case s.code != "" && sent != s.code:
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":"incorrect_totp_token"}`)
		default:
			fmt.Fprint(w, `{"jwt":"jwt-1","registration_created":false,"verify_email_sent":false}`)
		}
	}))
	t.Cleanup(srv.Close)

	return s, srv.URL
}

func (s *loginServer) failWith(rt route) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failAs = &rt
}

func (s *loginServer) sent() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.codes)
}

func (s *loginServer) stayedLoggedIn() []bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.stays)
}

func (s *loginServer) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.logins
}

const account = "1\nvera\n2\nprint password\n"

func connectWith(t *testing.T, sysURL string, input string, commands systemtest.Commands) (system.Settings, string, error) {
	t.Helper()

	sys, err := New(system.Env{})
	if err != nil {
		t.Fatal(err)
	}
	if commands == nil {
		commands = systemtest.Commands{}
	}
	commands["print password"] = "secret"

	var out strings.Builder
	term := prompt.NewTerminal(strings.NewReader(input), &out, commands)
	settings, err := sys.Connect(context.Background(), term, sysURL)
	return settings, out.String(), err
}

func TestConnectStoresTheTokenInsteadOfThePassword(t *testing.T) {
	eachFlavor(t, func(t *testing.T, fl flavor) {
		server, sysURL := newLoginServer(t, fl, "")

		settings, out, err := connectWith(t, sysURL, account+"1\n\n", nil)
		if err != nil {
			t.Fatalf("Connect: %v\n%s", err, out)
		}

		want := map[string]string{"username": "vera", "token": "jwt-1"}
		if !maps.Equal(settings.Credentials, want) {
			t.Errorf("got %v, want %v", settings.Credentials, want)
		}
		if !slices.Equal(server.sent(), []string{""}) {
			t.Errorf("sent codes %q", server.sent())
		}
		if !slices.Equal(server.stayedLoggedIn(), []bool{fl.name == "v4"}) {
			t.Errorf("sent stay_logged_in %v", server.stayedLoggedIn())
		}
		if !strings.Contains(out, "Logged in. Your password won't be stored") ||
			strings.Contains(out, "two-factor") {
			t.Errorf("unexpected output:\n%s", out)
		}
	})
}

func TestConnectAsksForTheCodeOfATwoFactorAccount(t *testing.T) {
	eachFlavor(t, func(t *testing.T, fl flavor) {
		server, sysURL := newLoginServer(t, fl, "123456")

		settings, out, err := connectWith(t, sysURL, account+"123 456\n1\n\n", nil)
		if err != nil {
			t.Fatalf("Connect: %v\n%s", err, out)
		}

		if settings.Credential(system.CredentialToken) != "jwt-1" {
			t.Errorf("got credentials %v", settings.Credentials)
		}
		if !slices.Equal(server.sent(), []string{"", "123456"}) {
			t.Errorf("sent codes %q", server.sent())
		}
		if !strings.Contains(out, "Your account uses two-factor authentication.") {
			t.Errorf("missing the 2FA notice:\n%s", out)
		}
	})
}

func TestConnectAsksAgainAfterARejectedCode(t *testing.T) {
	eachFlavor(t, func(t *testing.T, fl flavor) {
		server, sysURL := newLoginServer(t, fl, "123456")

		_, out, err := connectWith(t, sysURL, account+"111111\n123456\n1\n\n", nil)
		if err != nil {
			t.Fatalf("Connect: %v\n%s", err, out)
		}
		if !slices.Equal(server.sent(), []string{"", "111111", "123456"}) {
			t.Errorf("sent codes %q", server.sent())
		}
		if !strings.Contains(out, "The code was rejected.") {
			t.Errorf("missing the rejection:\n%s", out)
		}
	})
}

func TestConnectStopsAfterThreeRejectedCodes(t *testing.T) {
	eachFlavor(t, func(t *testing.T, fl flavor) {
		server, sysURL := newLoginServer(t, fl, "123456")

		_, _, err := connectWith(t, sysURL, account+"111111\n222222\n333333\n", nil)
		if err == nil || !strings.Contains(err.Error(), "three codes were rejected") {
			t.Fatalf("got %v", err)
		}
		if len(server.sent()) != 4 {
			t.Errorf("sent codes %q", server.sent())
		}
	})
}

func TestConnectChecksTheCodeBeforeSendingIt(t *testing.T) {
	eachFlavor(t, func(t *testing.T, fl flavor) {
		server, sysURL := newLoginServer(t, fl, "123456")

		_, out, err := connectWith(t, sysURL, account+"12345\nabcdef\n1234567\n123456\n1\n\n", nil)
		if err != nil {
			t.Fatalf("Connect: %v\n%s", err, out)
		}
		if n := strings.Count(out, "A 2FA code has six digits."); n != 3 {
			t.Errorf("got %d hints:\n%s", n, out)
		}
		if !slices.Equal(server.sent(), []string{"", "123456"}) {
			t.Errorf("sent codes %q", server.sent())
		}
	})
}

func TestConnectStopsWhenLoginsAreLimited(t *testing.T) {
	eachFlavor(t, func(t *testing.T, fl flavor) {
		server, sysURL := newLoginServer(t, fl, "")
		server.failWith(fl.rateLimited)

		_, _, err := connectWith(t, sysURL, account, nil)
		if err == nil || !strings.Contains(err.Error(), "too many logins from this IP address") {
			t.Fatalf("got %v", err)
		}
		if n := server.count(); n != 1 {
			t.Errorf("logged in %d times", n)
		}
	})
}

func TestConnectStopsOnAWrongPassword(t *testing.T) {
	eachFlavor(t, func(t *testing.T, fl flavor) {
		_, sysURL := newLoginServer(t, fl, "")

		sys, err := New(system.Env{})
		if err != nil {
			t.Fatal(err)
		}
		term := prompt.NewTerminal(strings.NewReader("1\nvera\n2\nprint password\n"), io.Discard,
			systemtest.Commands{"print password": "wrong"})
		_, err = sys.Connect(context.Background(), term, sysURL)
		if err == nil || !strings.Contains(err.Error(), "401 Unauthorized: incorrect_login") {
			t.Fatalf("got %v", err)
		}
	})
}

func TestConnectReadsTheTokenFromACommand(t *testing.T) {
	eachFlavor(t, func(t *testing.T, fl flavor) {
		_, sysURL := newLoginServer(t, fl, "")

		settings, out, err := connectWith(t, sysURL, account+"2\npass show lemmy/token\n\n",
			systemtest.Commands{"pass show lemmy/token": "jwt-1"})
		if err != nil {
			t.Fatalf("Connect: %v\n%s", err, out)
		}

		want := map[string]string{"username": "vera", "token_cmd": "pass show lemmy/token"}
		if !maps.Equal(settings.Credentials, want) {
			t.Errorf("got %v, want %v", settings.Credentials, want)
		}
		if !strings.Contains(out, "Save this session token in your password manager now.") ||
			strings.Count(out, "jwt-1") != 1 {
			t.Errorf("the token wasn't printed once:\n%s", out)
		}
	})
}

func TestConnectWithoutAnAccount(t *testing.T) {
	eachFlavor(t, func(t *testing.T, fl flavor) {
		server, sysURL := newLoginServer(t, fl, "")

		sys, err := New(system.Env{})
		if err != nil {
			t.Fatal(err)
		}

		var out strings.Builder
		term := prompt.NewTerminal(strings.NewReader("3\n\n"), &out, systemtest.Commands{})

		settings, err := sys.Connect(context.Background(), term, sysURL)
		if err != nil {
			t.Fatalf("Connect: %v", err)
		}
		if len(settings.Credentials) != 0 || server.count() != 0 {
			t.Errorf("got credentials %v after %d logins", settings.Credentials, server.count())
		}
		for _, notice := range []string{
			"Connecting without an account",
			"The subscribed feed needs an account",
		} {
			if !strings.Contains(out.String(), notice) {
				t.Errorf("missing notice %q:\n%s", notice, out.String())
			}
		}
	})
}

func TestConnectChecksTheSiteBeforeAsking(t *testing.T) {
	for _, tc := range []struct {
		name string
		info string
		want string
	}{
		{"other software", `{"version":"2.1","software":{"name":"piefed","version":"1.2.3"}}`,
			"the site runs piefed 1.2.3, not Lemmy"},
		{"no number", `{"version":"2.1","software":{"name":"lemmy","version":"nightly"}}`,
			`can't tell which API Lemmy "nightly" serves`},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != nodeinfoPath {
				t.Errorf("%s: requested %s", tc.name, r.URL.Path)
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, tc.info)
		}))
		t.Cleanup(srv.Close)

		var out strings.Builder
		sys, err := New(system.Env{})
		if err != nil {
			t.Fatal(err)
		}
		_, err = sys.Connect(context.Background(), prompt.NewTerminal(strings.NewReader(account), &out,
			systemtest.Commands{"print password": "secret"}), srv.URL)
		if err == nil || err.Error() != "could not connect to "+srv.URL+": "+tc.want {
			t.Errorf("%s: got %v", tc.name, err)
		}
		if out.Len() != 0 {
			t.Errorf("%s: asked before the check:\n%s", tc.name, out.String())
		}
	}
}

func TestConnectEndsWhenTheSiteDoesntAnswer(t *testing.T) {
	sys, err := New(system.Env{ReadTimeout: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	term := prompt.NewTerminal(strings.NewReader(account+"1\n\n"), &out,
		systemtest.Commands{"print password": "secret"})
	_, err = sys.Connect(context.Background(), term, systemtest.Blocking(t))
	if err == nil || !strings.Contains(err.Error(), "didn't answer within 0.05 seconds") {
		t.Fatalf("got %v, expected the timeout wording\n%s", err, out.String())
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error("the deadline error must stay recognizable")
	}
	if out.Len() != 0 {
		t.Errorf("asked before the check:\n%s", out.String())
	}
}
