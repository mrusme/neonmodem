package hyperuplink

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/prompt"
	"github.com/mrusme/neonmodem/internal/system/systemtest"
)

func sessionServer(t *testing.T) string {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/session" || r.Header.Get("Authorization") != "Bearer hup_secret" {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":"invalid token"}`)
			return
		}
		fmt.Fprintf(w, `{"user":{"id":"1","username":%q,"role":"member"}}`, "vera")
	}))
	t.Cleanup(srv.Close)

	return srv.URL
}

func connect(
	t *testing.T,
	sysURL string,
	input string,
	commands systemtest.Commands,
) (system.Settings, string, error) {
	t.Helper()

	sys, err := New(system.Env{})
	if err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	term := prompt.NewTerminal(strings.NewReader(input), &out, commands)
	settings, err := sys.Connect(context.Background(), term, sysURL)

	return settings, out.String(), err
}

func TestConnectStoresTheTokenCommand(t *testing.T) {
	sysURL := sessionServer(t)

	settings, out, err := connect(t, sysURL, "1\nvera\n2\npass show hup\n",
		systemtest.Commands{"pass show hup": "hup_secret"})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	want := map[string]string{"username": "vera", "token_cmd": "pass show hup"}
	if !maps.Equal(settings.Credentials, want) {
		t.Errorf("got %v, want %v", settings.Credentials, want)
	}
	if strings.Contains(out, "hup_secret") {
		t.Errorf("the token was printed:\n%s", out)
	}
}

func TestConnectRejectsAUsernameCommandForAnotherUser(t *testing.T) {
	sysURL := sessionServer(t)

	_, _, err := connect(t, sysURL, "2\npass show hup/user\n2\npass show hup\n",
		systemtest.Commands{"pass show hup/user": "mallory", "pass show hup": "hup_secret"})
	if err == nil ||
		!strings.Contains(err.Error(), "belongs to 'vera'") ||
		!strings.Contains(err.Error(), "printed 'mallory'") {
		t.Errorf("got %v", err)
	}
}

func TestConnectReplacesATypedUsernameForAnotherUser(t *testing.T) {
	sysURL := sessionServer(t)

	settings, out, err := connect(t, sysURL, "1\nmallory\n2\npass show hup\n",
		systemtest.Commands{"pass show hup": "hup_secret"})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if settings.Credentials["username"] != "vera" {
		t.Errorf("got %v", settings.Credentials)
	}
	if !strings.Contains(out, "this token belongs to 'vera', not 'mallory'") {
		t.Errorf("the replacement wasn't shown:\n%s", out)
	}
}

func TestConnectReportsARejectedToken(t *testing.T) {
	sysURL := sessionServer(t)

	_, _, err := connect(t, sysURL, "1\nvera\n2\npass show hup\n",
		systemtest.Commands{"pass show hup": "hup_other"})
	if err == nil || !strings.Contains(err.Error(), "could not authenticate") {
		t.Errorf("got %v", err)
	}
}

func TestConnectEndsWhenTheSiteDoesntAnswer(t *testing.T) {
	sys, err := New(system.Env{ReadTimeout: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	term := prompt.NewTerminal(strings.NewReader("1\nvera\n2\nprint token\n"), &out,
		systemtest.Commands{"print token": "hup_secret"})
	_, err = sys.Connect(context.Background(), term, systemtest.Blocking(t))
	if err == nil || !strings.Contains(err.Error(), "didn't answer within 0.05 seconds") {
		t.Fatalf("got %v, expected the timeout wording\n%s", err, out.String())
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error("the deadline error must stay recognizable")
	}
}
