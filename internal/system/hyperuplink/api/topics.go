package api

import (
	"context"
	"net/url"
	"strconv"
)

const (
	SortActive  = "active"
	SortNew     = "new"
	SortReplies = "replies"

	topicsPath  = "/topics"
	repliesPath = "/replies"
)

type Author struct {
	ID                string `json:"id"`
	Username          string `json:"username"`
	Role              string `json:"role"`
	ProfilePictureURL string `json:"profile_picture_url"`
	JoinedAt          string `json:"joined_at"`
}

type Attachment struct {
	ID        string `json:"id"`
	Filename  string `json:"filename"`
	MimeType  string `json:"mime_type"`
	CreatedAt string `json:"created_at"`
	URL       string `json:"url"`
}

type PollOption struct {
	Index   int    `json:"index"`
	Text    string `json:"text"`
	Votes   int    `json:"votes"`
	Percent int    `json:"percent"`
}

type Poll struct {
	Options []PollOption `json:"options"`
	Total   int          `json:"total"`
	Ended   bool         `json:"ended"`
	EndsAt  string       `json:"ends_at"`
}

type Topic struct {
	ID             string       `json:"id"`
	ShortID        string       `json:"short_id"`
	Slug           string       `json:"slug"`
	Name           string       `json:"name"`
	Kind           string       `json:"kind"`
	Pinned         bool         `json:"pinned"`
	LockedAt       string       `json:"locked_at"`
	Text           string       `json:"text"`
	HTML           string       `json:"html"`
	CreatedAt      string       `json:"created_at"`
	LastActivityAt string       `json:"last_activity_at"`
	Replies        int          `json:"replies"`
	Views          int64        `json:"views"`
	Author         Author       `json:"author"`
	Forum          Ref          `json:"forum"`
	Category       Ref          `json:"category"`
	Attachments    []Attachment `json:"attachments"`
	Poll           *Poll        `json:"poll"`
	URL            string       `json:"url"`
}

type Reply struct {
	ID          string       `json:"id"`
	ShortID     string       `json:"short_id"`
	TopicID     string       `json:"topic_id"`
	ParentID    string       `json:"parent_id"`
	Text        string       `json:"text"`
	HTML        string       `json:"html"`
	CreatedAt   string       `json:"created_at"`
	Author      Author       `json:"author"`
	Attachments []Attachment `json:"attachments"`
	URL         string       `json:"url"`
}

type Pagination struct {
	Page    int   `json:"page"`
	PerPage int   `json:"per_page"`
	Total   int64 `json:"total"`
	Pages   int   `json:"pages"`
}

type TopicsResponse struct {
	Forum      *Forum     `json:"forum"`
	Topics     []Topic    `json:"topics"`
	Pagination Pagination `json:"pagination"`
}

type RepliesResponse struct {
	Replies    []Reply    `json:"replies"`
	Pagination Pagination `json:"pagination"`
}

type NewTopic struct {
	ForumID string `json:"forum_id"`
	Name    string `json:"name"`
	Text    string `json:"text"`
	Kind    string `json:"kind,omitempty"`
}

type NewReply struct {
	Text     string `json:"text"`
	ParentID string `json:"parent_id,omitempty"`
}

func (c *Client) Topics(ctx context.Context, forumID string, sort string, page int) (*TopicsResponse, error) {
	query := url.Values{}
	query.Set("sort", sort)
	query.Set("page", strconv.Itoa(page))

	path := topicsPath
	if forumID != "" {
		path = forumsPath + "/" + forumID + topicsPath
	}

	response := new(TopicsResponse)
	if err := c.http.Get(ctx, path, query, response); err != nil {
		return nil, err
	}

	return response, nil
}

func (c *Client) Topic(ctx context.Context, id string) (*Topic, error) {
	var response struct {
		Topic Topic `json:"topic"`
	}
	if err := c.http.Get(ctx, topicsPath+"/"+id, nil, &response); err != nil {
		return nil, err
	}

	return &response.Topic, nil
}

func (c *Client) Replies(ctx context.Context, topicID string, page int, perPage int) (*RepliesResponse, error) {
	query := url.Values{}
	query.Set("page", strconv.Itoa(page))
	query.Set("per_page", strconv.Itoa(perPage))

	response := new(RepliesResponse)
	if err := c.http.Get(ctx, topicsPath+"/"+topicID+repliesPath, query, response); err != nil {
		return nil, err
	}

	return response, nil
}

func (c *Client) CreateTopic(ctx context.Context, in *NewTopic) (*Topic, error) {
	var response struct {
		Topic Topic `json:"topic"`
	}
	if err := c.http.Post(ctx, topicsPath, in, &response); err != nil {
		return nil, err
	}

	return &response.Topic, nil
}

func (c *Client) CreateReply(ctx context.Context, topicID string, in *NewReply) (*Reply, error) {
	var response struct {
		Reply Reply `json:"reply"`
	}
	if err := c.http.Post(ctx, topicsPath+"/"+topicID+repliesPath, in, &response); err != nil {
		return nil, err
	}

	return &response.Reply, nil
}
