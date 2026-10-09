package hyperuplink

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/hyperuplink/api"
	"github.com/mrusme/neonmodem/internal/system/prompt"
	"github.com/mrusme/neonmodem/internal/system/systemtest"
)

type board struct {
	guests     bool
	otp        bool
	limited    bool
	apiVersion string

	signIns  int
	keyNames []string
}

func newBoard() *board {
	return &board{guests: true, apiVersion: "v1"}
}

func (b *board) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/v1" && r.Method == http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"name":"Example Board","api":%q,"version":"v0.2.0","guests":%t,`+
			`"base_url":"https://board.example","links":{"session":"/api/v1/session"}}`, b.apiVersion, b.guests)
	case r.URL.Path == "/api/v1/session" && r.Method == http.MethodPost:
		b.signIn(w, r)
	case r.URL.Path == "/api/v1/session" && r.Method == http.MethodGet:
		if r.Header.Get("Authorization") != "Bearer hup_secret" {
			problem(w, http.StatusUnauthorized, "err_apikey_invalid")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"authenticated":true,"role":"user","user":{"id":"1","username":"vera","role":"user"},`+
			`"key":{"id":"k1","name":"browser"},"permissions":{"admin":false,"default":"read_write","categories":{}}}`)
	default:
		problem(w, http.StatusNotFound, "err_not_found")
	}
}

func (b *board) signIn(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
		OTPCode  string `json:"otp_code"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		problem(w, http.StatusBadRequest, "err_form_invalid")
		return
	}
	b.signIns++

	switch {
	case b.limited:
		w.Header().Set("Retry-After", "60")
		problem(w, http.StatusTooManyRequests, "err_rate_limited")
		return
	case !strings.EqualFold(in.Username, "vera") || in.Password != "secret":
		problem(w, http.StatusUnauthorized, "username_password_wrong")
		return
	case b.otp && in.OTPCode == "":
		problem(w, http.StatusUnauthorized, "err_otp_required")
		return
	case b.otp && in.OTPCode != "123456":
		problem(w, http.StatusUnauthorized, "err_otp_code_wrong")
		return
	}

	b.keyNames = append(b.keyNames, in.Name)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, `{"token":"hup_new","key":{"id":"k2","name":%q},"user":{"id":"1","username":"vera","role":"user"}}`, in.Name)
}

func serve(t *testing.T, handler http.Handler) string {
	t.Helper()

	srv := httptest.NewServer(handler)
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

const (
	passwordInput = "1\nvera\n1\n2\nprint password\n"
	keyInput      = "1\nvera\n2\n2\npass show hup\n"
)

var (
	passwordCommand = systemtest.Commands{"print password": "secret"}
	keyCommand      = systemtest.Commands{"pass show hup": "hup_secret"}
)

func TestConnectSignsInWithThePassword(t *testing.T) {
	b := newBoard()
	sysURL := serve(t, b)

	settings, out, err := connect(t, sysURL, passwordInput+"1\n", passwordCommand)
	if err != nil {
		t.Fatalf("Connect: %v\n%s", err, out)
	}

	want := map[string]string{"username": "vera", "token": "hup_new"}
	if settings.URL != sysURL || !maps.Equal(settings.Credentials, want) {
		t.Errorf("got %q %v, want %v", settings.URL, settings.Credentials, want)
	}
	for _, line := range []string{
		`Connecting to "Example Board" (Hyperuplink v0.2.0).`,
		"3) No account, read-only access",
		"How do you want to sign in?",
		"Signed in. Your password won't be stored, only the API key the board issued.",
		"How do you want to store your API key?",
	} {
		if !strings.Contains(out, line) {
			t.Errorf("the output lacks %q:\n%s", line, out)
		}
	}
	if strings.Contains(out, "hup_new") || strings.Contains(out, "secret") {
		t.Errorf("a secret was printed:\n%s", out)
	}
	if len(b.keyNames) != 1 || !strings.HasPrefix(b.keyNames[0], "neonmodem") || len(b.keyNames[0]) > keyNameMax {
		t.Errorf("the key was named %q", b.keyNames)
	}
}

func TestConnectStoresTheServersSpellingOfATypedUsername(t *testing.T) {
	sysURL := serve(t, newBoard())

	settings, _, err := connect(t, sysURL, "1\nVERA\n1\n2\nprint password\n1\n", passwordCommand)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if settings.Credentials["username"] != "vera" {
		t.Errorf("got %v", settings.Credentials)
	}
}

func TestConnectKeepsAUsernameCommandOnThePasswordPath(t *testing.T) {
	sysURL := serve(t, newBoard())

	settings, _, err := connect(t, sysURL, "2\nprint user\n1\n2\nprint password\n2\nprint key\n",
		systemtest.Commands{"print user": "VERA", "print password": "secret", "print key": "hup_new"})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	want := map[string]string{"username_cmd": "print user", "token_cmd": "print key"}
	if !maps.Equal(settings.Credentials, want) {
		t.Errorf("got %v, want %v", settings.Credentials, want)
	}
}

func TestConnectAsksForTheCodeWhenTheBoardWantsOne(t *testing.T) {
	b := newBoard()
	b.otp = true
	sysURL := serve(t, b)

	settings, out, err := connect(t, sysURL, passwordInput+"12 34\n000000\n12 34 56\n1\n", passwordCommand)
	if err != nil {
		t.Fatalf("Connect: %v\n%s", err, out)
	}
	if settings.Credentials["token"] != "hup_new" {
		t.Errorf("got %v", settings.Credentials)
	}
	for _, line := range []string{
		"Your account uses two-factor authentication.",
		"A 2FA code has six digits.",
		"The code was rejected.",
	} {
		if !strings.Contains(out, line) {
			t.Errorf("the output lacks %q:\n%s", line, out)
		}
	}
	if b.signIns != 3 {
		t.Errorf("the board saw %d sign-ins, expected 3", b.signIns)
	}
}

func TestConnectGivesUpAfterThreeRejectedCodes(t *testing.T) {
	b := newBoard()
	b.otp = true
	sysURL := serve(t, b)

	_, out, err := connect(t, sysURL, passwordInput+"000000\n000001\n000002\n", passwordCommand)
	if err == nil || !strings.Contains(err.Error(), "three codes were rejected") {
		t.Fatalf("got %v\n%s", err, out)
	}
	if n := strings.Count(out, "The code was rejected."); n != 2 {
		t.Errorf("the rejection notice was shown %d times, expected 2", n)
	}
}

func TestConnectReportsWrongCredentials(t *testing.T) {
	sysURL := serve(t, newBoard())

	_, _, err := connect(t, sysURL, passwordInput, systemtest.Commands{"print password": "nope"})
	if err == nil || !strings.Contains(err.Error(), "could not sign in to "+sysURL+": the username or password is wrong") {
		t.Fatalf("got %v", err)
	}
}

func TestConnectReportsTheSignInLimit(t *testing.T) {
	b := newBoard()
	b.limited = true
	sysURL := serve(t, b)

	start := time.Now()
	_, _, err := connect(t, sysURL, passwordInput, passwordCommand)
	if err == nil || !strings.Contains(err.Error(), "too many sign-ins from this address") {
		t.Fatalf("got %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Error("the limited sign-in was retried instead of reported")
	}
}

func TestConnectOffersNoAccountOnlyWhenGuestsAreServed(t *testing.T) {
	open := serve(t, newBoard())
	settings, out, err := connect(t, open, "3\n", nil)
	if err != nil {
		t.Fatalf("Connect: %v\n%s", err, out)
	}
	if settings.URL != open || len(settings.Credentials) != 0 {
		t.Errorf("a guest connection stored %+v", settings)
	}
	if !strings.Contains(out, "Connecting without an account; posting and replying will not be available.") {
		t.Errorf("the read-only notice is missing:\n%s", out)
	}

	b := newBoard()
	b.guests = false
	closed := serve(t, b)
	settings, out, err = connect(t, closed, keyInput, keyCommand)
	if err != nil {
		t.Fatalf("Connect: %v\n%s", err, out)
	}
	if strings.Contains(out, "No account") {
		t.Errorf("a closed board offered a guest connection:\n%s", out)
	}
	if !strings.Contains(out, `Connecting to "Example Board" (Hyperuplink v0.2.0). The board doesn't serve guests, so an account is needed.`) {
		t.Errorf("the closed board wasn't announced:\n%s", out)
	}
	if settings.Credentials["token_cmd"] != "pass show hup" {
		t.Errorf("got %v", settings.Credentials)
	}
}

func TestConnectStoresTheKeyCommand(t *testing.T) {
	sysURL := serve(t, newBoard())

	settings, out, err := connect(t, sysURL, keyInput, keyCommand)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	want := map[string]string{"username": "vera", "token_cmd": "pass show hup"}
	if !maps.Equal(settings.Credentials, want) {
		t.Errorf("got %v, want %v", settings.Credentials, want)
	}
	if strings.Contains(out, "hup_secret") {
		t.Errorf("the key was printed:\n%s", out)
	}
}

func TestConnectRejectsAUsernameCommandForAnotherUser(t *testing.T) {
	sysURL := serve(t, newBoard())

	_, _, err := connect(t, sysURL, "2\npass show hup/user\n2\n2\npass show hup\n",
		systemtest.Commands{"pass show hup/user": "mallory", "pass show hup": "hup_secret"})
	if err == nil ||
		!strings.Contains(err.Error(), "belongs to 'vera'") ||
		!strings.Contains(err.Error(), "printed 'mallory'") {
		t.Errorf("got %v", err)
	}
}

func TestConnectReplacesATypedUsernameForAnotherUser(t *testing.T) {
	sysURL := serve(t, newBoard())

	settings, out, err := connect(t, sysURL, "1\nmallory\n2\n2\npass show hup\n", keyCommand)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if settings.Credentials["username"] != "vera" {
		t.Errorf("got %v", settings.Credentials)
	}
	if !strings.Contains(out, "this API key belongs to 'vera', not 'mallory'") {
		t.Errorf("the replacement wasn't shown:\n%s", out)
	}
}

func TestConnectReportsARejectedKey(t *testing.T) {
	sysURL := serve(t, newBoard())

	_, _, err := connect(t, sysURL, keyInput, systemtest.Commands{"pass show hup": "hup_other"})
	if err == nil || !strings.Contains(err.Error(), "could not authenticate") ||
		!strings.Contains(err.Error(), "err_apikey_invalid") {
		t.Errorf("got %v", err)
	}
}

func TestConnectStripsATypedSuffix(t *testing.T) {
	sysURL := serve(t, newBoard())

	settings, out, err := connect(t, sysURL+"/api/v1/", "3\n", nil)
	if err != nil {
		t.Fatalf("Connect: %v\n%s", err, out)
	}
	if settings.URL != sysURL {
		t.Errorf("the stored URL is %q", settings.URL)
	}
	if !strings.Contains(out, "Using "+sysURL+"; neonmodem adds /api/v1 itself.") {
		t.Errorf("the notice is missing:\n%s", out)
	}
}

func TestConnectNamesTheAPIAddressForAWebPage(t *testing.T) {
	sysURL := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("<!DOCTYPE html><html><body>Example Board</body></html>"))
	}))

	_, _, err := connect(t, sysURL, "", nil)
	if err == nil || !strings.Contains(err.Error(), sysURL+" doesn't look quite right") ||
		!strings.Contains(err.Error(), "the address a reverse proxy forwards /api/ to") {
		t.Errorf("got %v", err)
	}
	if !errors.Is(err, api.ErrNotAnAPI) {
		t.Error("the error must stay recognizable")
	}
}

func TestConnectRejectsOtherAPIs(t *testing.T) {
	other := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"hello":"world"}`))
	}))
	if _, _, err := connect(t, other, "", nil); err == nil ||
		!strings.Contains(err.Error(), other+" is not a Hyperuplink API") {
		t.Errorf("got %v", err)
	}

	b := newBoard()
	b.apiVersion = "v2"
	newer := serve(t, b)
	if _, _, err := connect(t, newer, "", nil); err == nil ||
		!strings.Contains(err.Error(), newer+" serves API version v2; this version of neonmodem speaks v1") {
		t.Errorf("got %v", err)
	}
}

func TestConnectEndsWhenTheSiteDoesntAnswer(t *testing.T) {
	sys, err := New(system.Env{ReadTimeout: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	term := prompt.NewTerminal(strings.NewReader(""), &out, systemtest.Commands{})
	_, err = sys.Connect(context.Background(), term, systemtest.Blocking(t))
	if err == nil || !strings.Contains(err.Error(), "didn't answer within 0.05 seconds") {
		t.Fatalf("got %v, expected the timeout wording\n%s", err, out.String())
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error("the deadline error must stay recognizable")
	}
}
