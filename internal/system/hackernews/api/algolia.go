package api

import (
	"context"
	"strconv"
)

type AlgoliaItem struct {
	ID         int           `json:"id"`
	CreatedAt  string        `json:"created_at"`
	CreatedAtI int64         `json:"created_at_i"`
	Type       string        `json:"type"`
	Author     *string       `json:"author"`
	Title      *string       `json:"title"`
	URL        *string       `json:"url"`
	Text       *string       `json:"text"`
	Points     *int          `json:"points"`
	ParentID   *int          `json:"parent_id"`
	StoryID    *int          `json:"story_id"`
	Children   []AlgoliaItem `json:"children"`
}

func (i *AlgoliaItem) AuthorName() string {
	if i.Author == nil {
		return ""
	}
	return *i.Author
}

func (i *AlgoliaItem) Body() string {
	if i.Text == nil {
		return ""
	}
	return *i.Text
}

func (i *AlgoliaItem) IsDeleted() bool {
	return i.Author == nil || (i.Type == "comment" && i.Text == nil)
}

func (c *Client) Tree(ctx context.Context, id int) (*AlgoliaItem, error) {
	item := new(AlgoliaItem)
	if err := c.algolia.Get(ctx, "/items/"+strconv.Itoa(id), nil, item); err != nil {
		return nil, err
	}

	return item, nil
}
