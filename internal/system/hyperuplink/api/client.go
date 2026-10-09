package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/mrusme/neonmodem/internal/system/httpx"
)

const (
	Prefix  = "/api/v1"
	Version = "v1"

	CodeAPIKeyInvalid          = "err_apikey_invalid"
	CodeAuthenticationRequired = "err_authentication_required"
	CodeOTPRequired            = "err_otp_required"
	CodeOTPCodeWrong           = "err_otp_code_wrong"
	CodeUsernamePasswordWrong  = "username_password_wrong"
	CodeRateLimited            = "err_rate_limited"
)

var ErrNotAnAPI = httpx.ErrNotJSON

type Client struct {
	http *httpx.Client
}

func NewClient(httpClient *http.Client, origin string, token string) (*Client, error) {
	hc, err := httpx.NewClient(httpClient, strings.TrimRight(origin, "/")+Prefix)
	if err != nil {
		return nil, err
	}
	hc.DecodeError = decodeError
	if token != "" {
		hc.Headers.Set("Authorization", "Bearer "+token)
	}

	return &Client{http: hc}, nil
}

type Problem struct {
	Type     string              `json:"type"`
	Title    string              `json:"title"`
	Status   int                 `json:"status"`
	Detail   string              `json:"detail"`
	Instance string              `json:"instance"`
	Code     string              `json:"code"`
	Errors   map[string][]string `json:"errors"`
}

func (p Problem) String() string {
	var b strings.Builder
	b.WriteString(p.Code)
	if p.Detail != "" {
		b.WriteString(": " + p.Detail)
	}

	if len(p.Errors) > 0 {
		fields := make([]string, 0, len(p.Errors))
		for field := range p.Errors {
			fields = append(fields, field)
		}
		sort.Strings(fields)

		parts := make([]string, 0, len(fields))
		for _, field := range fields {
			parts = append(parts, fmt.Sprintf("%s: %s", field, strings.Join(p.Errors[field], ", ")))
		}
		fmt.Fprintf(&b, " (%s)", strings.Join(parts, "; "))
	}

	return b.String()
}

func parseProblem(body []byte) (Problem, bool) {
	var p Problem
	if err := json.Unmarshal(body, &p); err != nil || p.Code == "" {
		return Problem{}, false
	}
	return p, true
}

func decodeError(_ int, body []byte) string {
	p, ok := parseProblem(body)
	if !ok {
		return ""
	}
	return p.String()
}

func ProblemOf(err error) (Problem, bool) {
	e, ok := errors.AsType[*httpx.Error](err)
	if !ok {
		return Problem{}, false
	}
	return parseProblem(e.Body)
}
