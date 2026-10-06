package api

import (
	"context"
	"net/url"
)

const StoriesBaseURL = "/s"

type CommentModel struct {
	ShortID        string `json:"short_id"`
	ShortIDURL     string `json:"short_id_url"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
	LastEditedAt   string `json:"last_edited_at"`
	IsDeleted      bool   `json:"is_deleted"`
	IsModerated    bool   `json:"is_moderated"`
	Score          int    `json:"score"`
	Flags          int    `json:"flags"`
	ParentComment  string `json:"parent_comment"`
	Comment        string `json:"comment"`
	CommentPlain   string `json:"comment_plain"`
	URL            string `json:"url"`
	Depth          int    `json:"depth"`
	CommentingUser string `json:"commenting_user"`
}

type StoryModel struct {
	ShortID          string         `json:"short_id"`
	ShortIDURL       string         `json:"short_id_url"`
	CreatedAt        string         `json:"created_at"`
	Title            string         `json:"title"`
	URL              string         `json:"url"`
	Score            int            `json:"score"`
	Flags            int            `json:"flags"`
	CommentCount     int            `json:"comment_count"`
	Description      string         `json:"description"`
	DescriptionPlain string         `json:"description_plain"`
	CommentsURL      string         `json:"comments_url"`
	SubmitterUser    string         `json:"submitter_user"`
	UserIsAuthor     bool           `json:"user_is_author"`
	Tags             []string       `json:"tags"`
	Comments         []CommentModel `json:"comments"`
}

type StoriesService interface {
	Show(ctx context.Context, id string) (*StoryModel, error)
	List(ctx context.Context, tag string) ([]StoryModel, error)
}

type StoryServiceHandler struct {
	client *Client
}

func (a *StoryServiceHandler) Show(
	ctx context.Context,
	id string,
) (*StoryModel, error) {
	response := new(StoryModel)
	if err := a.client.http.Get(ctx, StoriesBaseURL+"/"+url.PathEscape(id)+".json", nil, response); err != nil {
		return nil, err
	}

	return response, nil
}

func (a *StoryServiceHandler) List(
	ctx context.Context,
	tag string,
) ([]StoryModel, error) {
	path := "/newest.json"
	if tag != "" {
		path = "/t/" + url.PathEscape(tag) + ".json"
	}

	var response []StoryModel
	if err := a.client.http.Get(ctx, path, nil, &response); err != nil {
		return nil, err
	}

	return response, nil
}
