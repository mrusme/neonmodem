package api

// https://docs.discourse.org

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mrusme/neonmodem/internal/system/httpx"
)

type Credentials struct {
	ClientID string
	Key      string
}

type Client struct {
	http *httpx.Client
}

func NewClient(httpClient *http.Client, endpoint string, creds Credentials) (*Client, error) {
	hc, err := httpx.NewClient(httpClient, endpoint)
	if err != nil {
		return nil, err
	}
	hc.DecodeError = decodeError

	if creds.Key != "" {
		hc.Headers.Set("User-Api-Client-Id", creds.ClientID)
		hc.Headers.Set("User-Api-Key", creds.Key)
	}

	return &Client{http: hc}, nil
}

type ErrorBody struct {
	Errors    []string `json:"errors"`
	ErrorType string   `json:"error_type"`
}

func decodeError(status int, body []byte) string {
	var errbody ErrorBody
	if err := json.Unmarshal(body, &errbody); err != nil {
		return ""
	}
	return strings.Join(errbody.Errors, "\n")
}
