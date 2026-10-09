package api

import "context"

const forumsPath = "/forums"

type Ref struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type Access struct {
	Read     bool `json:"read"`
	Write    bool `json:"write"`
	Moderate bool `json:"moderate"`
}

type Forum struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	Position       int    `json:"position"`
	Description    string `json:"description"`
	Category       Ref    `json:"category"`
	Topics         int    `json:"topics"`
	Replies        int    `json:"replies"`
	LastActivityAt string `json:"last_activity_at"`
	Permissions    Access `json:"permissions"`
	URL            string `json:"url"`
}

func (c *Client) Forums(ctx context.Context) ([]Forum, error) {
	var response struct {
		Forums []Forum `json:"forums"`
	}
	if err := c.http.Get(ctx, forumsPath, nil, &response); err != nil {
		return nil, err
	}

	return response.Forums, nil
}
