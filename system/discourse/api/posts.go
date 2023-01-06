package api

import (
	"context"
	"net/http"
)

const PostsBaseURL = "/posts"

type CreatePostModel struct {
	Title             string `json:"title,omitempty"`
	Raw               string `json:"raw"`
	TopicID           int    `json:"topic_id,omitempty"`
	ReplyToPostNumber int    `json:"reply_to_post_number,omitempty"`
	Category          int    `json:"category,omitempty"`
	TargetRecipients  string `json:"targe_recipients,omitempty"`
	Archetype         string `json:"archetype,omitempty"`
	CreatedAt         string `json:"created_at,omitempty"`
	EmbedURL          string `json:"embed_url,omitempty"`
	ExternalID        string `json:"external_id,omitempty"`
}

type PostModel struct {
	ID                int     `json:"id"`
	Name              string  `json:"name"`
	Username          string  `json:"username"`
	AvatarTemplate    string  `json:"avater_template"`
	CreatedAt         string  `json:"created_at"`
	Cooked            string  `json:"cooked"`
	PostNumber        int     `json:"post_number"`
	PostType          int     `json:"post_type"`
	UpdatedAt         string  `json:"updated_at"`
	ReplyCount        int     `json:"reply_count"`
	IncomingLinkCount int     `json:"incoming_link_count"`
	Reads             int     `json:"reads"`
	ReadersCount      int     `json:"readers_count"`
	Score             float32 `json:"score"`
	Yours             bool    `json:"yours"`
	TopicID           int     `json:"topic_id"`
	TopicSlug         string  `json:"topic_slug"`
	TopicTitle        string  `json:"topic_title"`
	TopicHTMLTitle    string  `json:"topic_html_title"`
	CategoryID        int     `json:"category_id"`
	DisplayUsername   string  `json:"display_username"`
	PrimaryGroupName  string  `json:"primary_group_name"`
	FlairName         string  `json:"flair_name"`
	FlairURL          string  `json:"flair_url"`
	FlairBGColor      string  `json:"flair_bg_color"`
	FlairColor        string  `json:"flair_color"`
	Version           int     `json:"version"`
	CanEdit           bool    `json:"can_edit"`
	CanDelete         bool    `json:"can_delete"`
	CanRecover        bool    `json:"can_recover"`
	CanWiki           bool    `json:"can_wiki"`
	UserTitle         string  `json:"user_title"`
	Raw               string  `json:"raw"`
	ActionsSummary    []struct {
		ID     int  `json:"id"`
		CanAct bool `json:"can_act"`
	} `json:"actions_summary"`
	Moderator                   bool   `json:"moderator"`
	Admin                       bool   `json:"admin"`
	Staff                       bool   `json:"staff"`
	UserID                      int    `json:"user_id"`
	Hidden                      bool   `json:"hidden"`
	TrustLevel                  int    `json:"trust_level"`
	DeletedAt                   string `json:"deleted_at"`
	UserDeleted                 bool   `json:"user_deleted"`
	EditReason                  string `json:"edit_reason"`
	CanViewEditHistory          bool   `json:"can_view_edit_history"`
	Wiki                        bool   `json:"wiki"`
	ReviewableID                string `json:"reviewable_id"`
	ReviewableScoreCount        int    `json:"reviewable_score_count"`
	ReviewableScorePendingCount int    `json:"reviewable_score_pending_count"`
}

type ListPostsResponse struct {
	LatestPosts []PostModel `json:"latest_posts,omitempty"`
}

type PostsService interface {
	Create(
		ctx context.Context,
		w *CreatePostModel,
	) (PostModel, error)
	Show(
		ctx context.Context,
		id string,
	) (PostModel, error)
	List(
		ctx context.Context,
	) (*ListPostsResponse, error)
}

type PostServiceHandler struct {
	client *Client
}

// Create
func (a *PostServiceHandler) Create(
	ctx context.Context,
	w *CreatePostModel,
) (PostModel, error) {
	uri := PostsBaseURL + ".json"

	req, err := a.client.NewRequest(ctx, http.MethodPost, uri, w)
	if err != nil {
		return PostModel{}, err
	}

	response := new(PostModel)
	if err = a.client.Do(ctx, req, response); err != nil {
		return PostModel{}, err
	}

	return *response, nil
}

// Show
func (a *PostServiceHandler) Show(
	ctx context.Context,
	id string,
) (PostModel, error) {
	uri := PostsBaseURL + "/" + id + ".json"

	req, err := a.client.NewRequest(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return PostModel{}, err
	}

	response := new(Response)
	if err = a.client.Do(ctx, req, response); err != nil {
		return PostModel{}, err
	}

	return response.Post, nil
}

// List
func (a *PostServiceHandler) List(
	ctx context.Context,
) (*ListPostsResponse, error) {
	uri := PostsBaseURL + ".json"

	req, err := a.client.NewRequest(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	response := new(ListPostsResponse)
	if err = a.client.Do(ctx, req, response); err != nil {
		return nil, err
	}

	return response, nil
}
