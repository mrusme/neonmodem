package api

import "context"

type Discovery struct {
	Name    string            `json:"name"`
	API     string            `json:"api"`
	Version string            `json:"version"`
	Guests  bool              `json:"guests"`
	BaseURL string            `json:"base_url"`
	Links   map[string]string `json:"links"`
}

func (c *Client) Discover(ctx context.Context) (*Discovery, error) {
	response := new(Discovery)
	if err := c.http.Get(ctx, "", nil, response); err != nil {
		return nil, err
	}

	return response, nil
}
