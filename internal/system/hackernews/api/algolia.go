package api

import (
	"context"
	"net/url"
	"strconv"
	"time"
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

type SearchHit struct {
	ObjectID    string   `json:"objectID"`
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	Author      string   `json:"author"`
	Points      *int     `json:"points"`
	NumComments *int     `json:"num_comments"`
	CreatedAtI  int64    `json:"created_at_i"`
	StoryText   *string  `json:"story_text"`
	Tags        []string `json:"_tags"`
}

func (h SearchHit) Text() string {
	if h.StoryText == nil {
		return ""
	}
	return *h.StoryText
}

func (h SearchHit) Comments() int {
	if h.NumComments == nil {
		return 0
	}
	return *h.NumComments
}

func (h SearchHit) PointsOrZero() int {
	if h.Points == nil {
		return 0
	}
	return *h.Points
}

type searchResult struct {
	Hits []SearchHit `json:"hits"`
}

func (c *Client) Search(ctx context.Context, tag string, since time.Time, limit int) ([]SearchHit, error) {
	return c.search(ctx, "/search", tag, since, limit)
}

func (c *Client) SearchByDate(ctx context.Context, tag string, limit int) ([]SearchHit, error) {
	return c.search(ctx, "/search_by_date", tag, time.Time{}, limit)
}

func (c *Client) search(
	ctx context.Context,
	path string,
	tag string,
	since time.Time,
	limit int,
) ([]SearchHit, error) {
	query := url.Values{}
	query.Set("tags", tag)
	query.Set("hitsPerPage", strconv.Itoa(limit))
	if !since.IsZero() {
		query.Set("numericFilters", "created_at_i>"+strconv.FormatInt(since.Unix(), 10))
	}

	var result searchResult
	if err := c.algolia.Get(ctx, path, query, &result); err != nil {
		return nil, err
	}
	return result.Hits, nil
}
