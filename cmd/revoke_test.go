package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrusme/neonmodem/internal/config"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/prompt"
	"github.com/mrusme/neonmodem/internal/system/systemtest"
)

type hupBoard struct {
	revokeStatus int
	revokeCode   string
	revoked      []string
}

func hupProblem(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type": "about:blank", "title": http.StatusText(status), "status": status, "code": code,
	})
}

func (b *hupBoard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/v1" && r.Method == http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"name":"Example Board","api":"v1","version":"v0.2.0","guests":true,`+
			`"base_url":"https://board.example","links":{}}`)
	case r.URL.Path == "/api/v1/session" && r.Method == http.MethodGet:
		if r.Header.Get("Authorization") != "Bearer hup_new" {
			hupProblem(w, http.StatusUnauthorized, "err_apikey_invalid")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"authenticated":true,"role":"user","user":{"id":"1","username":"vera","role":"user"},`+
			`"permissions":{"admin":false,"default":"read_write","categories":{}}}`)
	case r.URL.Path == "/api/v1/session" && r.Method == http.MethodDelete:
		b.revoked = append(b.revoked, r.Header.Get("Authorization"))
		if b.revokeStatus != 0 {
			hupProblem(w, b.revokeStatus, b.revokeCode)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		hupProblem(w, http.StatusNotFound, "err_not_found")
	}
}

func hyperuplinkAt(sysURL string, credentials map[string]string) config.SystemConfig {
	return config.SystemConfig{
		Type:     "hyperuplink",
		Settings: system.Settings{URL: sysURL, Credentials: credentials},
	}
}

func runHyperuplinkConnect(
	t *testing.T,
	a *app,
	input string,
	commands systemtest.Commands,
	sysURL string,
) (string, error) {
	t.Helper()

	var out bytes.Buffer
	term := prompt.NewTerminal(strings.NewReader(input), &out, commands)
	err := a.connect(context.Background(), term, &out, "hyperuplink", sysURL)
	return out.String(), err
}

const replaceWithKey = "2\n1\nvera\n2\n2\nprint key\n"

var newKeyCommand = systemtest.Commands{"print key": "hup_new"}

func TestReplacingAHyperuplinkConnectionRevokesTheOldKey(t *testing.T) {
	b := &hupBoard{}
	srv := httptest.NewServer(b)
	t.Cleanup(srv.Close)
	a, path := connectApp(t, hyperuplinkAt(srv.URL, map[string]string{"username": "vera", "token": "hup_old"}))

	out, err := runHyperuplinkConnect(t, a, replaceWithKey, newKeyCommand, srv.URL)
	if err != nil {
		t.Fatalf("connect: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Successfully replaced the connection!") ||
		!strings.Contains(out, "Revoked the previous API key on 127.0.0.1.") ||
		strings.Contains(out, "stay valid") {
		t.Errorf("unexpected output:\n%s", out)
	}
	if len(b.revoked) != 1 || b.revoked[0] != "Bearer hup_old" {
		t.Errorf("the board saw the revocations %q", b.revoked)
	}

	saved, err := config.LoadFrom([]string{path}, filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"username": "vera", "token_cmd": "print key"}
	if len(saved.Systems) != 1 || saved.Systems[0].Settings.Credential("token_cmd") != want["token_cmd"] ||
		saved.Systems[0].Settings.Credential("username") != want["username"] ||
		saved.Systems[0].Settings.Credential("token") != "" {
		t.Errorf("the saved entry is %+v", saved.Systems)
	}
}

func TestReplacingReportsAnAlreadyInvalidKey(t *testing.T) {
	b := &hupBoard{revokeStatus: http.StatusUnauthorized, revokeCode: "err_apikey_invalid"}
	srv := httptest.NewServer(b)
	t.Cleanup(srv.Close)
	a, _ := connectApp(t, hyperuplinkAt(srv.URL, map[string]string{"username": "vera", "token": "hup_old"}))

	out, err := runHyperuplinkConnect(t, a, replaceWithKey, newKeyCommand, srv.URL)
	if err != nil {
		t.Fatalf("connect: %v\n%s", err, out)
	}
	if !strings.Contains(out, "The previous API key was already invalid.") || strings.Contains(out, "stay valid") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestReplacingReportsAFailedRevocation(t *testing.T) {
	b := &hupBoard{revokeStatus: http.StatusInternalServerError, revokeCode: "err_internal"}
	srv := httptest.NewServer(b)
	t.Cleanup(srv.Close)
	a, _ := connectApp(t, hyperuplinkAt(srv.URL, map[string]string{"username": "vera", "token": "hup_old"}))

	out, err := runHyperuplinkConnect(t, a, replaceWithKey, newKeyCommand, srv.URL)
	if err != nil {
		t.Fatalf("connect: %v\n%s", err, out)
	}
	if !strings.Contains(out, "The previous API key couldn't be revoked on 127.0.0.1: DELETE "+srv.URL+
		"/api/v1/session returned status 500: err_internal. It stays valid until you revoke it there.") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestReplacingLeavesAKeyFromACommandAlone(t *testing.T) {
	b := &hupBoard{}
	srv := httptest.NewServer(b)
	t.Cleanup(srv.Close)
	a, _ := connectApp(t, hyperuplinkAt(srv.URL, map[string]string{"username": "vera", "token_cmd": "pass show old"}))

	out, err := runHyperuplinkConnect(t, a, replaceWithKey, newKeyCommand, srv.URL)
	if err != nil {
		t.Fatalf("connect: %v\n%s", err, out)
	}
	if !strings.Contains(out, "The previous API key comes from a command and wasn't revoked. "+
		"It stays valid until you revoke it on 127.0.0.1.") {
		t.Errorf("unexpected output:\n%s", out)
	}
	if len(b.revoked) != 0 {
		t.Errorf("the board saw the revocations %q", b.revoked)
	}
}

func TestReplacingAGuestConnectionPrintsNoReminder(t *testing.T) {
	b := &hupBoard{}
	srv := httptest.NewServer(b)
	t.Cleanup(srv.Close)
	a, _ := connectApp(t, hyperuplinkAt(srv.URL, nil))

	out, err := runHyperuplinkConnect(t, a, "2\n3\n", nil, srv.URL)
	if err != nil {
		t.Fatalf("connect: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Successfully replaced the connection!") || strings.Contains(out, "previous") {
		t.Errorf("unexpected output:\n%s", out)
	}
	if len(b.revoked) != 0 {
		t.Errorf("the board saw the revocations %q", b.revoked)
	}
}

func TestARevocationFailureReachesTheTerminalWithoutEscapes(t *testing.T) {
	b := &hupBoard{revokeStatus: http.StatusInternalServerError, revokeCode: "err\x1b[2Jinternal"}
	srv := httptest.NewServer(b)
	t.Cleanup(srv.Close)
	a, _ := connectApp(t, hyperuplinkAt(srv.URL, map[string]string{"username": "vera", "token": "hup_old"}))

	out, err := runHyperuplinkConnect(t, a, replaceWithKey, newKeyCommand, srv.URL)
	if err != nil {
		t.Fatalf("connect: %v\n%s", err, out)
	}
	if strings.ContainsRune(out, 0x1b) || !strings.Contains(out, "returned status 500: errinternal. It stays valid") {
		t.Errorf("unexpected output:\n%q", out)
	}
}
