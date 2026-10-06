package api

import (
	"net/http"

	"github.com/mrusme/neonmodem/internal/system/httpx"
)

const (
	FirebaseBaseURL = "https://hacker-news.firebaseio.com/v0"
	AlgoliaBaseURL  = "https://hn.algolia.com/api/v1"
	SiteURL         = "https://news.ycombinator.com"

	DefaultWorkers = 8
)

type Client struct {
	firebase *httpx.Client
	algolia  *httpx.Client
	workers  int
}

func NewClient(httpClient *http.Client) (*Client, error) {
	return NewClientWithBases(httpClient, FirebaseBaseURL, AlgoliaBaseURL)
}

func NewClientWithBases(httpClient *http.Client, firebaseBase string, algoliaBase string) (*Client, error) {
	firebase, err := httpx.NewClient(httpClient, firebaseBase)
	if err != nil {
		return nil, err
	}
	algolia, err := httpx.NewClient(httpClient, algoliaBase)
	if err != nil {
		return nil, err
	}

	return &Client{
		firebase: firebase,
		algolia:  algolia,
		workers:  DefaultWorkers,
	}, nil
}
