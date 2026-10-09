package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mrusme/neonmodem/internal/system/httpx"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client, err := NewClient(httpx.NewHTTPClient(httpx.Options{}), srv.URL, "hup_test")
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func problem(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	fmt.Fprint(w, body)
}

// A web interface answering on the address the user gave us returns HTML, which
// used to reach the JSON decoder and surface as "invalid character '<'".
func TestWhoamiOnHTMLResponse(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<!DOCTYPE html><html><body>Sign in</body></html>"))
	})

	_, err := client.Whoami(context.Background())
	if err == nil {
		t.Fatal("expected an error for an HTML response, got none")
	}
	if !errors.Is(err, ErrNotAnAPI) {
		t.Fatalf("expected ErrNotAnAPI, got: %v", err)
	}
	if strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("JSON decoder error leaked to the user: %v", err)
	}
}

// A server that isn't Hyperuplink at all typically returns an HTML 404.
func TestWhoamiOnHTMLErrorResponse(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("<html><body>404 not found</body></html>"))
	})

	_, err := client.Whoami(context.Background())
	if err == nil {
		t.Fatal("expected an error for an HTML 404, got none")
	}
	if !errors.Is(err, ErrNotAnAPI) {
		t.Fatalf("expected ErrNotAnAPI, got: %v", err)
	}

	var reqErr *httpx.Error
	if !errors.As(err, &reqErr) || reqErr.StatusCode != http.StatusNotFound {
		t.Fatalf("expected an httpx.Error carrying status 404, got: %v", err)
	}
}

// A real API rejecting the key has to keep reporting its own code.
func TestWhoamiOnProblem(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="hyperuplink", error="invalid_token"`)
		problem(w, http.StatusUnauthorized,
			`{"type":"about:blank","title":"Unauthorized","status":401,"instance":"/api/v1/session","code":"err_apikey_invalid"}`)
	})

	_, err := client.Whoami(context.Background())
	if err == nil {
		t.Fatal("expected an error for a rejected key, got none")
	}
	if errors.Is(err, ErrNotAnAPI) {
		t.Fatalf("a problem was misreported as a bad URL: %v", err)
	}
	if !strings.Contains(err.Error(), "returned status 401: err_apikey_invalid") {
		t.Fatalf("expected the problem code, got: %v", err)
	}

	p, ok := ProblemOf(fmt.Errorf("wrapped: %w", err))
	if !ok || p.Code != CodeAPIKeyInvalid || p.Status != http.StatusUnauthorized {
		t.Fatalf("ProblemOf gave %+v, %v", p, ok)
	}
}

func TestWhoamiOnValidResponse(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/session" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer hup_test" {
			t.Errorf("unexpected Authorization header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte(`{"authenticated":true,"role":"user","user":{"id":"1","username":"vera","role":"user"},` +
			`"key":{"id":"k1","name":"browser","last_used_at":null,"created_at":"2026-10-01T10:00:00Z"},` +
			`"permissions":{"admin":false,"default":"read_write","categories":{"c1":"read"}}}`))
	})

	session, err := client.Whoami(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !session.Authenticated || session.User == nil || session.User.Username != "vera" {
		t.Fatalf("unexpected session: %+v", session)
	}
	if session.Key == nil || session.Key.Name != "browser" || session.Permissions.Categories["c1"] != "read" {
		t.Fatalf("unexpected key or permissions: %+v", session)
	}
}

// JSON that doesn't decode is a broken server, not a wrong address.
func TestWhoamiOnMalformedJSON(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"user":`))
	})

	_, err := client.Whoami(context.Background())
	if err == nil {
		t.Fatal("expected an error for malformed JSON, got none")
	}
	if !strings.Contains(err.Error(), "malformed response") {
		t.Fatalf("expected a malformed response error, got: %v", err)
	}
}

func TestProblemsListTheirFields(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		problem(w, http.StatusUnprocessableEntity,
			`{"type":"about:blank","title":"Unprocessable Entity","status":422,"code":"validation",`+
				`"errors":{"name":["required","max"],"forum_id":["uuid"]}}`)
	})

	_, err := client.CreateTopic(context.Background(), &NewTopic{})
	if err == nil {
		t.Fatal("expected a validation error, got none")
	}
	if !strings.Contains(err.Error(), "validation (forum_id: uuid; name: required, max)") {
		t.Fatalf("expected the fields in order, got: %v", err)
	}
}

func TestProblemsShowTheirDetail(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		problem(w, http.StatusServiceUnavailable,
			`{"type":"about:blank","title":"Service Unavailable","status":503,"code":"err_unavailable","detail":"the database is down"}`)
	})

	_, err := client.Forums(context.Background())
	if err == nil || !strings.Contains(err.Error(), "err_unavailable: the database is down") {
		t.Fatalf("expected the detail, got: %v", err)
	}
}

// A JSON error without a code, as the old API sent, is shown as it came.
func TestOtherJSONErrorsKeepTheirBody(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	})

	_, err := client.Forums(context.Background())
	if err == nil || !strings.Contains(err.Error(), `{"error":"not found"}`) {
		t.Fatalf("expected the body, got: %v", err)
	}
	if _, ok := ProblemOf(err); ok {
		t.Fatal("a body without a code isn't a problem")
	}
	if _, ok := ProblemOf(errors.New("plain")); ok {
		t.Fatal("a plain error isn't a problem")
	}
}

func TestRevokeDeletesTheSession(t *testing.T) {
	var method, path string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	})

	if err := client.Revoke(context.Background()); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodDelete || path != "/api/v1/session" {
		t.Fatalf("the request was %s %s", method, path)
	}
}
