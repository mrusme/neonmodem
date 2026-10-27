package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var (
	ErrSessionInvalid = errors.New("the session isn't valid")
	ErrCodeRequired   = errors.New("a 2FA code is required")
	ErrCodeIncorrect  = errors.New("the 2FA code is incorrect")
	ErrRateLimited    = errors.New("too many requests")
	ErrNoToken        = errors.New("the server didn't send a session token")
)

type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d %s", e.Status, http.StatusText(e.Status))
	if e.Code != "" {
		b.WriteString(": " + e.Code)
	}
	if e.Message != "" {
		b.WriteString(": " + e.Message)
	}
	return b.String()
}

func (e *Error) Is(target error) bool {
	switch target {
	case ErrSessionInvalid:
		return e.Status == http.StatusUnauthorized ||
			e.Code == "not_logged_in" ||
			e.Code == "incorrect_login"
	case ErrCodeRequired:
		return e.Code == "missing_totp_token"
	case ErrCodeIncorrect:
		return e.Code == "incorrect_totp_token"
	case ErrRateLimited:
		return e.Status == http.StatusTooManyRequests || e.Code == "rate_limit_error"
	}
	return false
}
