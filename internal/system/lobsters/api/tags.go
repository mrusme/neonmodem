package api

import (
	"context"
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

func (c *Client) Tags(ctx context.Context) ([]TagModel, error) {
	var response []TagModel
	if err := c.http.Get(ctx, TagsBaseURL+".json", nil, &response); err != nil {
		return nil, err
	}

	return response, nil
}
