package api

import (
	"context"
	"net/http"
)

const StoriesBaseURL = "/s"

type UserModel struct {
	Username        string `json:"username"`
	CreatedAt       string `json:"created_at"`
	IsAdmin         bool   `json:"is_admin"`
	About           string `json:"about"`
	IsModerator     bool   `json:"is_moderator"`
	Karma           int    `json:"karma"`
	AvatarURL       string `json:"avatar_url"`
	InvitedByUser   string `json:"invited_by_user"`
	GithubUsername  string `json:"github_username"`
	TwitterUsername string `json:"twitter_username"`
}

type StoryModel struct {
	ShortID          string    `json:"short_id"`
	ShortIDURL       string    `json:"short_id_url"`
	CreatedAt        string    `json:"created_at"`
	Title            string    `json:"title"`
	URL              string    `json:"url"`
	Score            int       `json:"score"`
	Flags            int       `json:"flags"`
	CommentCount     int       `json:"comment_count"`
	Description      string    `json:"description"`
	DescriptionPlain string    `json:"description_plain"`
	CommentsURL      string    `json:"comments_url"`
	CategoryID       int       `json:"category_id"`
	SubmitterUser    UserModel `json:"submitter_user"`
	Tags             []string  `json:"tags"`
	Comments         []struct {
		ShortID        string    `json:"short_id"`
		ShortIDURL     string    `json:"short_id_url"`
		CreatedAt      string    `json:"created_at"`
		UpdatedAt      string    `json:"updated_at"`
		IsDeleted      bool      `json:"is_deleted"`
		IsModerated    bool      `json:"is_moderated"`
		Score          int       `json:"score"`
		Flags          int       `json:"flags"`
		ParentComment  string    `json:"parent_comment"`
		Comment        string    `json:"comment"`
		CommentPlain   string    `json:"comment_plain"`
		CommentsURL    string    `json:"comments_url"`
		URL            string    `json:"url"`
		IndentLevel    int       `json:"indent_level"`
		CommentingUser UserModel `json:"commenting_user"`
	} `json:"comments"`
}

type StoriesService interface {
	Show(
		ctx context.Context,
		id string,
	) (*StoryModel, error)
	List(
		ctx context.Context,
		tag string,
	) (*[]StoryModel, error)
}

type StoryServiceHandler struct {
	client *Client
}

// Show
func (a *StoryServiceHandler) Show(
	ctx context.Context,
	id string,
) (*StoryModel, error) {
	uri := StoriesBaseURL + "/" + id + ".json"

	req, err := a.client.NewRequest(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	response := new(StoryModel)
	if err = a.client.Do(ctx, req, response); err != nil {
		return nil, err
	}

	return response, nil
}

// List
func (a *StoryServiceHandler) List(
	ctx context.Context,
	tag string,
) (*[]StoryModel, error) {
	var uri string
	if tag == "" {
		uri = "/newest.json"
	} else {
		uri = "/t/" + tag + ".json"
	}

	req, err := a.client.NewRequest(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	response := new([]StoryModel)
	if err = a.client.Do(ctx, req, response); err != nil {
		return nil, err
	}

	return response, nil
}
