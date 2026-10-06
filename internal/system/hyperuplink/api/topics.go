package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

const TopicsBaseURL = "/topics"

type TopicModel struct {
	ID          string   `json:"id"`
	ShortID     string   `json:"short_id"`
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	ForumID     string   `json:"forum_id"`
	AuthorID    string   `json:"author_id"`
	Kind        string   `json:"kind"`
	Pinned      bool     `json:"pinned"`
	Text        string   `json:"text"`
	HTML        string   `json:"html"`
	PollOptions []string `json:"poll_options"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	LockedAt  string `json:"locked_at"`
	DeletedAt string `json:"deleted_at"`

	Views int64 `json:"views"`

	CategoryName string `json:"category_name"`
	CategorySlug string `json:"category_slug"`
	ForumName    string `json:"forum_name"`
	ForumSlug    string `json:"forum_slug"`

	AuthorUsername string `json:"author_username"`

	Replies     int    `json:"replies"`
	LastReplyAt string `json:"last_reply_at"`
}

type ReplyModel struct {
	ID       string `json:"id"`
	ShortID  string `json:"short_id"`
	TopicID  string `json:"topic_id"`
	ReplyID  string `json:"reply_id"`
	AuthorID string `json:"author_id"`

	Text string `json:"text"`
	HTML string `json:"html"`

	CreatedAt string `json:"created_at"`
	DeletedAt string `json:"deleted_at"`

	AuthorUsername string `json:"author_username"`
}

type TopicsResponse struct {
	Forum  *ForumModel  `json:"forum,omitempty"`
	Topics []TopicModel `json:"topics"`
	Total  int64        `json:"total"`
	Pages  int          `json:"pages"`
}

type TopicResponse struct {
	Topic   TopicModel   `json:"topic"`
	Replies []ReplyModel `json:"replies"`
	Total   int64        `json:"total"`
	Pages   int          `json:"pages"`
}

type CreateReplyModel struct {
	Text    string `json:"text"`
	ReplyID string `json:"reply_id,omitempty"`
}

type CreatedReply struct {
	ID      string `json:"id"`
	ShortID string `json:"short_id"`
}

type TopicsService interface {
	List(ctx context.Context, forumID string, page int) (*TopicsResponse, error)
	Show(ctx context.Context, id string, page int) (*TopicResponse, error)
	CreateReply(ctx context.Context, topicID string, w *CreateReplyModel) (*CreatedReply, error)
}

type TopicsServiceHandler struct {
	client *Client
}

func (a *TopicsServiceHandler) List(
	ctx context.Context,
	forumID string,
	page int,
) (*TopicsResponse, error) {
	query := url.Values{}
	query.Set("page", strconv.Itoa(page))
	if forumID != "" {
		query.Set("forum_id", forumID)
	}

	response := new(TopicsResponse)
	if err := a.client.http.Get(ctx, TopicsBaseURL, query, response); err != nil {
		return nil, err
	}

	return response, nil
}

func (a *TopicsServiceHandler) Show(
	ctx context.Context,
	id string,
	page int,
) (*TopicResponse, error) {
	query := url.Values{}
	query.Set("page", strconv.Itoa(page))

	response := new(TopicResponse)
	if err := a.client.http.Get(ctx, TopicsBaseURL+"/"+id, query, response); err != nil {
		return nil, err
	}

	return response, nil
}

func (a *TopicsServiceHandler) CreateReply(
	ctx context.Context,
	topicID string,
	w *CreateReplyModel,
) (*CreatedReply, error) {
	response := new(CreatedReply)
	path := fmt.Sprintf("%s/%s/replies", TopicsBaseURL, topicID)
	if err := a.client.http.Post(ctx, path, w, response); err != nil {
		return nil, err
	}

	return response, nil
}
