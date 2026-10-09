package api

import "context"

const sessionPath = "/session"

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type Key struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	LastUsedAt string `json:"last_used_at"`
	CreatedAt  string `json:"created_at"`
}

type Permissions struct {
	Admin      bool              `json:"admin"`
	Default    string            `json:"default"`
	Categories map[string]string `json:"categories"`
}

type Session struct {
	Authenticated bool        `json:"authenticated"`
	Role          string      `json:"role"`
	User          *User       `json:"user"`
	Key           *Key        `json:"key"`
	Permissions   Permissions `json:"permissions"`
}

func (c *Client) Whoami(ctx context.Context) (*Session, error) {
	response := new(Session)
	if err := c.http.Get(ctx, sessionPath, nil, response); err != nil {
		return nil, err
	}

	return response, nil
}

type SignInInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
	OTPCode  string `json:"otp_code,omitempty"`
	Name     string `json:"name,omitempty"`
}

type SignInResponse struct {
	Token string `json:"token"`
	Key   Key    `json:"key"`
	User  User   `json:"user"`
}

func (c *Client) SignIn(ctx context.Context, in *SignInInput) (*SignInResponse, error) {
	response := new(SignInResponse)
	if err := c.http.Post(ctx, sessionPath, in, response); err != nil {
		return nil, err
	}

	return response, nil
}

func (c *Client) Revoke(ctx context.Context) error {
	return c.http.Delete(ctx, sessionPath)
}
