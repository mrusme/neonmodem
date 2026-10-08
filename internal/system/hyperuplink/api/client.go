package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/mrusme/neonmodem/internal/system/httpx"
)

var ErrNotAnAPI = httpx.ErrNotJSON

type Client struct {
	http *httpx.Client
}

func NewClient(httpClient *http.Client, endpoint string, token string) (*Client, error) {
	hc, err := httpx.NewClient(httpClient, endpoint)
	if err != nil {
		return nil, err
	}
	hc.DecodeError = decodeError
	if token != "" {
		hc.Headers.Set("Authorization", "Bearer "+token)
	}

	return &Client{http: hc}, nil
}

type ErrorBody struct {
	Error  string            `json:"error"`
	Fields map[string]string `json:"fields"`
}

func decodeError(status int, body []byte) string {
	var errbody ErrorBody
	if err := json.Unmarshal(body, &errbody); err != nil || errbody.Error == "" {
		return ""
	}

	msg := errbody.Error
	if len(errbody.Fields) > 0 {
		fields := make([]string, 0, len(errbody.Fields))
		for field := range errbody.Fields {
			fields = append(fields, field)
		}
		sort.Strings(fields)

		parts := make([]string, 0, len(fields))
		for _, field := range fields {
			parts = append(parts, fmt.Sprintf("%s: %s", field, errbody.Fields[field]))
		}
		msg = fmt.Sprintf("%s (%s)", msg, strings.Join(parts, ", "))
	}

	return msg
}
