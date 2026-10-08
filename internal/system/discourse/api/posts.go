package api

import (
	"context"
)

const PostsBaseURL = "/posts"

type CreatePostModel struct {
	Title             string `json:"title,omitempty"`
	Raw               string `json:"raw"`
	TopicID           int    `json:"topic_id,omitempty"`
	ReplyToPostNumber int    `json:"reply_to_post_number,omitempty"`
	Category          int    `json:"category,omitempty"`
	TargetRecipients  string `json:"target_recipients,omitempty"`
	Archetype         string `json:"archetype,omitempty"`
	CreatedAt         string `json:"created_at,omitempty"`
	EmbedURL          string `json:"embed_url,omitempty"`
	ExternalID        string `json:"external_id,omitempty"`
}

type PostModel struct {
	ID                int     `json:"id"`
	Name              string  `json:"name"`
	Username          string  `json:"username"`
	AvatarTemplate    string  `json:"avatar_template"`
	CreatedAt         string  `json:"created_at"`
	Cooked            string  `json:"cooked"`
	PostNumber        int     `json:"post_number"`
	PostType          int     `json:"post_type"`
	UpdatedAt         string  `json:"updated_at"`
	ReplyCount        int     `json:"reply_count"`
	ReplyToPostNumber int     `json:"reply_to_post_number"`
	IncomingLinkCount int     `json:"incoming_link_count"`
	Reads             int     `json:"reads"`
	ReadersCount      int     `json:"readers_count"`
	Score             float32 `json:"score"`
	Yours             bool    `json:"yours"`
	TopicID           int     `json:"topic_id"`
	TopicSlug         string  `json:"topic_slug"`
	TopicTitle        string  `json:"topic_title"`
	CategoryID        int     `json:"category_id"`
	DisplayUsername   string  `json:"display_username"`
	Version           int     `json:"version"`
	CanEdit           bool    `json:"can_edit"`
	CanDelete         bool    `json:"can_delete"`
	UserTitle         string  `json:"user_title"`
	Raw               string  `json:"raw"`
	Moderator         bool    `json:"moderator"`
	Admin             bool    `json:"admin"`
	Staff             bool    `json:"staff"`
	UserID            int     `json:"user_id"`
	Hidden            bool    `json:"hidden"`
	TrustLevel        int     `json:"trust_level"`
	DeletedAt         string  `json:"deleted_at"`
	UserDeleted       bool    `json:"user_deleted"`
	EditReason        string  `json:"edit_reason"`
	Wiki              bool    `json:"wiki"`
}

func (c *Client) CreatePost(ctx context.Context, w *CreatePostModel) (PostModel, error) {
	response := new(PostModel)
	if err := c.http.Post(ctx, PostsBaseURL+".json", w, response); err != nil {
		return PostModel{}, err
	}

	return *response, nil
}

func (c *Client) Post(ctx context.Context, id string) (PostModel, error) {
	response := new(PostModel)
	if err := c.http.Get(ctx, PostsBaseURL+"/"+id+".json", nil, response); err != nil {
		return PostModel{}, err
	}

	return *response, nil
}
