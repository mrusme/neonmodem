package api

import (
	"context"
)

const SessionBaseURL = "/session"

type UserModel struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type SessionResponse struct {
	User UserModel `json:"user"`
}

type SessionService interface {
	Whoami(ctx context.Context) (*SessionResponse, error)
}

type SessionServiceHandler struct {
	client *Client
}

func (a *SessionServiceHandler) Whoami(
	ctx context.Context,
) (*SessionResponse, error) {
	response := new(SessionResponse)
	if err := a.client.http.Get(ctx, SessionBaseURL, nil, response); err != nil {
		return nil, err
	}

	return response, nil
}
