package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestURLKeepsSubpath(t *testing.T) {
	c, err := NewClient(http.DefaultClient, "https://host.example/forum/")
	if err != nil {
		t.Fatal(err)
	}

	if got := c.URL("/latest.json", nil); got != "https://host.example/forum/latest.json" {
		t.Errorf("absolute path lost the sub-path: %s", got)
	}

	q := url.Values{}
	q.Set("page", "2")
	if got := c.URL("t/1.json", q); got != "https://host.example/forum/t/1.json?page=2" {
		t.Errorf("relative path or query wrong: %s", got)
	}
}

func TestNewClientRejectsRelativeURL(t *testing.T) {
	if _, err := NewClient(http.DefaultClient, "host.example/forum"); err == nil {
		t.Fatal("expected an error for a URL without scheme")
	}
}

func serve(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c, err := NewClient(NewHTTPClient(Options{}), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestDoRejectsHTMLOnSuccess(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html>sign in</html>"))
	})

	var out map[string]any
	err := c.Get(context.Background(), "/x", nil, &out)
	if !errors.Is(err, ErrNotJSON) {
		t.Fatalf("expected ErrNotJSON, got %v", err)
	}
	if strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("decoder error leaked: %v", err)
	}
}

func TestDoReportsStatusAndDecodedMessage(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"errors":["not permitted"]}`))
	})
	c.DecodeError = func(status int, body []byte) string {
		if strings.Contains(string(body), "not permitted") {
			return "not permitted"
		}
		return ""
	}

	err := c.Get(context.Background(), "/x", nil, &map[string]any{})
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("expected *Error, got %T %v", err, err)
	}
	if e.StatusCode != http.StatusForbidden || e.Message != "not permitted" {
		t.Fatalf("unexpected error: %+v", e)
	}
	if !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "not permitted") {
		t.Fatalf("error text should name status and message: %v", err)
	}
	if StatusOf(err) != http.StatusForbidden {
		t.Fatal("StatusOf did not unwrap the status")
	}
}

func TestDoNonJSONErrorIsMarked(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("<html>404</html>"))
	})

	err := c.Get(context.Background(), "/x", nil, &map[string]any{})
	if !errors.Is(err, ErrNotJSON) || StatusOf(err) != http.StatusNotFound {
		t.Fatalf("expected a 404 marked as not JSON, got %v", err)
	}
}

func TestDoNoContentLeavesOutput(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	out := map[string]any{"keep": true}
	if err := c.Get(context.Background(), "/x", nil, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := out["keep"]; !ok {
		t.Fatal("output was modified on 204")
	}
}

func TestDoMalformedJSON(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"a":`))
	})

	err := c.Get(context.Background(), "/x", nil, &map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "malformed response") {
		t.Fatalf("expected a malformed response error, got %v", err)
	}
}

func TestHeadersAndUserAgent(t *testing.T) {
	var gotUA, gotAuth, gotAccept string
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	})
	c.Headers.Set("Authorization", "Bearer x")

	if err := c.Get(context.Background(), "/x", nil, &map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if gotUA != DefaultUserAgent {
		t.Errorf("user agent %q", gotUA)
	}
	if gotAuth != "Bearer x" {
		t.Errorf("authorization %q", gotAuth)
	}
	if gotAccept != "application/json" {
		t.Errorf("accept %q", gotAccept)
	}
}

func TestPostEncodesJSONBody(t *testing.T) {
	var gotBody, gotType string
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 1024)
		n, _ := r.Body.Read(b)
		gotBody = strings.TrimSpace(string(b[:n]))
		gotType = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":7}`))
	})

	var out struct{ ID int }
	if err := c.Post(context.Background(), "/posts", map[string]string{"raw": "hi"}, &out); err != nil {
		t.Fatal(err)
	}
	if gotBody != `{"raw":"hi"}` || gotType != "application/json" {
		t.Errorf("body %q type %q", gotBody, gotType)
	}
	if out.ID != 7 {
		t.Errorf("response not decoded: %+v", out)
	}
}

func TestRetriesOnlyWhenEnabled(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	retrying, _ := NewClient(NewHTTPClient(Options{Retries: 3}), srv.URL)
	if err := retrying.Get(context.Background(), "/x", nil, &map[string]any{}); err != nil {
		t.Fatalf("expected the retrying client to succeed, got %v", err)
	}
	if attempts.Load() != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts.Load())
	}

	attempts.Store(0)
	plain, _ := NewClient(NewHTTPClient(Options{}), srv.URL)
	err := plain.Get(context.Background(), "/x", nil, &map[string]any{})
	if StatusOf(err) != http.StatusInternalServerError {
		t.Fatalf("expected the plain client to fail with 500, got %v", err)
	}
	if attempts.Load() != 1 {
		t.Fatalf("plain client retried: %d attempts", attempts.Load())
	}
}
