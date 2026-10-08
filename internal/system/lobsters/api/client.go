package api

import (
	"net/http"

	"github.com/mrusme/neonmodem/internal/system/httpx"
)

type Client struct {
	http *httpx.Client
}

func NewClient(httpClient *http.Client, endpoint string) (*Client, error) {
	hc, err := httpx.NewClient(httpClient, endpoint)
	if err != nil {
		return nil, err
	}

	return &Client{http: hc}, nil
}
