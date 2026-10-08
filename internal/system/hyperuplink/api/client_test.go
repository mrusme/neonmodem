package api

import (
	"context"
	"errors"
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

// A real API rejecting the token has to keep reporting its own message.
func TestWhoamiOnJSONError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"err_apikey_invalid"}`))
	})

	_, err := client.Whoami(context.Background())
	if err == nil {
		t.Fatal("expected an error for a rejected token, got none")
	}
	if errors.Is(err, ErrNotAnAPI) {
		t.Fatalf("a JSON error was misreported as a bad URL: %v", err)
	}
	if !strings.Contains(err.Error(), "err_apikey_invalid") {
		t.Fatalf("expected the API error message, got: %v", err)
	}
}

func TestWhoamiOnValidResponse(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer hup_test" {
			t.Errorf("unexpected Authorization header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte(`{"user":{"id":"1","username":"vera","role":"user"}}`))
	})

	session, err := client.Whoami(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.User.Username != "vera" {
		t.Fatalf("expected username vera, got: %s", session.User.Username)
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

func TestFieldErrorsAreListed(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"error":"err_validation","fields":{"name":"too short","text":"required"}}`))
	})

	_, err := client.CreatePost(context.Background(), &NewPostModel{})
	if err == nil {
		t.Fatal("expected a validation error, got none")
	}
	if !strings.Contains(err.Error(), "name: too short, text: required") {
		t.Fatalf("expected the field errors in order, got: %v", err)
	}
}
