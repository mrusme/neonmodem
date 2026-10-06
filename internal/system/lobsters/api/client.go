package api

import (
	"net/http"

	"github.com/mrusme/neonmodem/internal/system/httpx"
)

type Client struct {
	http *httpx.Client

	Stories StoriesService
	Tags    TagsService
}

func NewClient(httpClient *http.Client, endpoint string) (*Client, error) {
	hc, err := httpx.NewClient(httpClient, endpoint)
	if err != nil {
		return nil, err
	}

	c := &Client{http: hc}
	c.Stories = &StoryServiceHandler{client: c}
	c.Tags = &TagServiceHandler{client: c}

	return c, nil
}
