package api

import (
	"context"
)

const NewPostBaseURL = "/new"

type NewPostModel struct {
	Name    string `json:"name"`
	Text    string `json:"text"`
	ForumID string `json:"forum_id"`
	Kind    string `json:"kind,omitempty"`
}

type CreatedPost struct {
	ID           string `json:"id"`
	Slug         string `json:"slug"`
	CategorySlug string `json:"category_slug"`
	ForumSlug    string `json:"forum_slug"`
}

type PostsService interface {
	Create(ctx context.Context, w *NewPostModel) (*CreatedPost, error)
}

type PostsServiceHandler struct {
	client *Client
}

func (a *PostsServiceHandler) Create(
	ctx context.Context,
	w *NewPostModel,
) (*CreatedPost, error) {
	response := new(CreatedPost)
	if err := a.client.http.Post(ctx, NewPostBaseURL, w, response); err != nil {
		return nil, err
	}

	return response, nil
}
