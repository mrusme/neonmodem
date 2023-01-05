package api

import (
	"context"
	"net/http"
)

const TagsBaseURL = "/tags"

type TagModel struct {
	ID               int     `json:"id"`
	Tag              string  `json:"tag"`
	Description      string  `json:"description"`
	Privileged       bool    `json:"privileged"`
	IsMedia          bool    `json:"is_media"`
	Active           bool    `json:"active"`
	HotnessMod       float32 `json:"hotness_mod"`
	PermitByNewUsers bool    `json:"permit_by_new_users"`
	CategoryID       int     `json:"category_id"`
}

type TagsService interface {
	List(
		ctx context.Context,
	) (*[]TagModel, error)
}

type TagServiceHandler struct {
	client *Client
}

// List
func (a *TagServiceHandler) List(
	ctx context.Context,
) (*[]TagModel, error) {
	uri := TagsBaseURL + ".json"

	req, err := a.client.NewRequest(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	response := new([]TagModel)
	if err = a.client.Do(ctx, req, response); err != nil {
		return nil, err
	}

	return response, nil
}
