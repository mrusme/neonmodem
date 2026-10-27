package api

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

var sentinels = []error{ErrSessionInvalid, ErrCodeRequired, ErrCodeIncorrect, ErrRateLimited}

func matches(err error) []error {
	var found []error
	for _, s := range sentinels {
		if errors.Is(err, s) {
			found = append(found, s)
		}
	}
	return found
}

func TestV3ErrorsBecomeErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		text   string
		match  error
	}{
		{"wrong password", http.StatusUnauthorized, `{"error":"incorrect_login"}`,
			"401 Unauthorized: incorrect_login", ErrSessionInvalid},
		{"broken token", http.StatusBadRequest, `{"error":"not_logged_in"}`,
			"400 Bad Request: not_logged_in", ErrSessionInvalid},
		{"no code", http.StatusBadRequest, `{"error":"missing_totp_token"}`,
			"400 Bad Request: missing_totp_token", ErrCodeRequired},
		{"wrong code", http.StatusBadRequest, `{"error":"incorrect_totp_token"}`,
			"400 Bad Request: incorrect_totp_token", ErrCodeIncorrect},
		{"rate limit", http.StatusBadRequest, `{"error":"rate_limit_error"}`,
			"400 Bad Request: rate_limit_error", ErrRateLimited},
		{"missing post", http.StatusBadRequest, `{"error":"couldnt_find_post"}`,
			"400 Bad Request: couldnt_find_post", nil},
		{"unknown route", http.StatusNotFound, ``,
			"404 Not Found", nil},
	} {
		_, sysURL := newFakeServer(t, V3, map[string][]reply{routeV3Login: {{tc.status, tc.body}}})
		c := newTestClient(t, sysURL, "")

		_, err := c.Login(context.Background(), Login{Username: "vera", Password: "secret"})
		le, ok := errors.AsType[*Error](err)
		if !ok {
			t.Fatalf("%s: got %v, want an *Error", tc.name, err)
		}
		if le.Status != tc.status || err.Error() != tc.text {
			t.Errorf("%s: got status %d and %q, want %d and %q", tc.name, le.Status, err, tc.status, tc.text)
		}

		found := matches(err)
		switch {
		case tc.match == nil && len(found) > 0:
			t.Errorf("%s matches %v", tc.name, found)
		case tc.match != nil && (len(found) != 1 || !errors.Is(found[0], tc.match)):
			t.Errorf("%s matches %v, want %v", tc.name, found, tc.match)
		}
	}
}

func TestV4ErrorsBecomeErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		text   string
		match  error
	}{
		{"wrong password", http.StatusUnauthorized, `{"error":"incorrect_login","cause":"IncorrectLogin"}`,
			"401 Unauthorized: incorrect_login", ErrSessionInvalid},
		{"broken token", http.StatusBadRequest, `{"error":"not_logged_in","cause":"NotLoggedIn"}`,
			"400 Bad Request: not_logged_in", ErrSessionInvalid},
		{"no code", http.StatusBadRequest, `{"error":"missing_totp_token","cause":"MissingTotpToken"}`,
			"400 Bad Request: missing_totp_token", ErrCodeRequired},
		{"wrong code", http.StatusBadRequest, `{"error":"incorrect_totp_token","cause":"IncorrectTotpToken"}`,
			"400 Bad Request: incorrect_totp_token", ErrCodeIncorrect},
		{"rate limit", http.StatusTooManyRequests, `{"error":"too_many_requests"}`,
			"429 Too Many Requests: too_many_requests", ErrRateLimited},
		{"missing post", http.StatusNotFound, `{"error":"not_found","cause":"Record not found"}`,
			"404 Not Found: not_found", nil},
		{"bad query", http.StatusBadRequest, `{"error":"unknown","message":"Query deserialize error: invalid digit found in string"}`,
			"400 Bad Request: unknown: Query deserialize error: invalid digit found in string", nil},
		{"unknown route", http.StatusNotFound, ``,
			"404 Not Found", nil},
		{"proxy page", http.StatusServiceUnavailable, `<html>maintenance</html>`,
			"503 Service Unavailable", nil},
	} {
		_, sysURL := newFakeServer(t, V4, map[string][]reply{routeV4Login: {{tc.status, tc.body}}})
		c := newTestClient(t, sysURL, "")

		_, err := c.Login(context.Background(), Login{Username: "vera", Password: "secret"})
		le, ok := errors.AsType[*Error](err)
		if !ok {
			t.Fatalf("%s: got %v, want an *Error", tc.name, err)
		}
		if le.Status != tc.status || err.Error() != tc.text {
			t.Errorf("%s: got status %d and %q, want %d and %q", tc.name, le.Status, err, tc.status, tc.text)
		}

		found := matches(err)
		switch {
		case tc.match == nil && len(found) > 0:
			t.Errorf("%s matches %v", tc.name, found)
		case tc.match != nil && (len(found) != 1 || !errors.Is(found[0], tc.match)):
			t.Errorf("%s matches %v, want %v", tc.name, found, tc.match)
		}
	}
}

func TestV3AnHTMLPageIsNoLemmyError(t *testing.T) {
	_, sysURL := newFakeServer(t, V3, map[string][]reply{
		routeV3Validate: {{http.StatusServiceUnavailable, `<html>maintenance</html>`}},
	})
	c := newTestClient(t, sysURL, "session-token")

	err := c.ValidateAuth(context.Background())
	if err == nil {
		t.Fatal("the page was accepted")
	}
	if _, ok := errors.AsType[*Error](err); ok {
		t.Errorf("got the Lemmy error %v", err)
	}
	if found := matches(err); len(found) > 0 {
		t.Errorf("%v matches %v", err, found)
	}
}

func TestErrorTextAndStatusSentinels(t *testing.T) {
	err := &Error{Status: http.StatusBadRequest, Code: "unknown", Message: "Query deserialize error: invalid digit found in string"}
	if got := err.Error(); got != "400 Bad Request: unknown: Query deserialize error: invalid digit found in string" {
		t.Errorf("got %q", got)
	}

	limited := &Error{Status: http.StatusTooManyRequests, Code: "too_many_requests"}
	if found := matches(limited); len(found) != 1 || !errors.Is(found[0], ErrRateLimited) {
		t.Errorf("a 429 matches %v", found)
	}

	unauthorized := &Error{Status: http.StatusUnauthorized}
	if found := matches(unauthorized); len(found) != 1 || !errors.Is(found[0], ErrSessionInvalid) {
		t.Errorf("a 401 without a code matches %v", found)
	}
}
