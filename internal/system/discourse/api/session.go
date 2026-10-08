package api

import (
	"context"
)

const SessionBaseURL = "/session/current.json"

type CurrentUserModel struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

type CurrentSessionResponse struct {
	CurrentUser CurrentUserModel `json:"current_user"`
}

func (c *Client) CurrentSession(ctx context.Context) (*CurrentSessionResponse, error) {
	response := new(CurrentSessionResponse)
	if err := c.http.Get(ctx, SessionBaseURL, nil, response); err != nil {
		return nil, err
	}

	return response, nil
}
