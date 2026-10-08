package api

import (
	"context"
	"net/url"

	"github.com/guregu/null"
)

const CategoriesBaseURL = "/categories"

type LatestCategoriesResponse struct {
	CategoryList struct {
		CanCreateCategory bool `json:"can_create_category"`
		CanCreateTopic    bool `json:"can_create_topic"`

		Categories []CategoryModel `json:"categories"`
	} `json:"category_list"`
}

type CategoryModel struct {
	ID                 int             `json:"id"`
	Name               string          `json:"name"`
	Color              string          `json:"color"`
	TextColor          string          `json:"text_color"`
	Slug               string          `json:"slug"`
	TopicCount         int             `json:"topic_count"`
	PostCount          int             `json:"post_count"`
	Position           int             `json:"position"`
	Description        null.String     `json:"description"`
	DescriptionText    null.String     `json:"description_text"`
	DescriptionExcerpt null.String     `json:"description_excerpt"`
	TopicUrl           null.String     `json:"topic_url"`
	ReadRestricted     bool            `json:"read_restricted"`
	Permission         null.Int        `json:"permission"`
	NotificationLevel  int             `json:"notification_level"`
	CanEdit            bool            `json:"can_edit"`
	HasChildren        null.Bool       `json:"has_children"`
	ParentCategoryID   null.Int        `json:"parent_category_id"`
	IsUncategorized    bool            `json:"is_uncategorized"`
	SubcategoryIDs     []int           `json:"subcategory_ids"`
	SubcategoryList    []CategoryModel `json:"subcategory_list"`
}

func (c *Client) Categories(ctx context.Context) (*LatestCategoriesResponse, error) {
	query := url.Values{}
	query.Set("include_subcategories", "true")

	response := new(LatestCategoriesResponse)
	if err := c.http.Get(ctx, CategoriesBaseURL+".json", query, response); err != nil {
		return nil, err
	}

	return response, nil
}
