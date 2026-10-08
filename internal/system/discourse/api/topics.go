package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

const TopicsBaseURL = "/t"

type UserModel struct {
	ID             int    `json:"id"`
	Username       string `json:"username"`
	Name           string `json:"name"`
	AvatarTemplate string `json:"avatar_template"`
}

type TopicListResponse struct {
	Users []UserModel `json:"users"`

	TopicList struct {
		CanCreateTopic bool   `json:"can_create_topic"`
		PerPage        int    `json:"per_page"`
		MoreTopicsURL  string `json:"more_topics_url"`

		Topics []TopicModel `json:"topics"`
	} `json:"topic_list"`
}

type SingleTopicResponse struct {
	PostStream struct {
		Posts  []PostModel `json:"posts"`
		Stream []int       `json:"stream"`
	} `json:"post_stream"`

	ChunkSize int `json:"chunk_size"`

	TopicModel
}

type PosterModel struct {
	Extras         string `json:"extras"`
	Description    string `json:"description"`
	UserID         int    `json:"user_id"`
	PrimaryGroupID int    `json:"primary_group_id"`
}

type TopicModel struct {
	ID                 int           `json:"id"`
	Title              string        `json:"title"`
	FancyTitle         string        `json:"fancy_title"`
	Slug               string        `json:"slug"`
	PostsCount         int           `json:"posts_count"`
	ReplyCount         int           `json:"reply_count"`
	HighestPostNumber  int           `json:"highest_post_number"`
	ImageURL           string        `json:"image_url"`
	CreatedAt          string        `json:"created_at"`
	LastPostedAt       string        `json:"last_posted_at"`
	Bumped             bool          `json:"bumped"`
	BumpedAt           string        `json:"bumped_at"`
	Archetype          string        `json:"archetype"`
	Unseen             bool          `json:"unseen"`
	Pinned             bool          `json:"pinned"`
	Visible            bool          `json:"visible"`
	Closed             bool          `json:"closed"`
	Archived           bool          `json:"archived"`
	Views              int           `json:"views"`
	LikeCount          int           `json:"like_count"`
	LastPosterUsername string        `json:"last_poster_username"`
	CategoryID         int           `json:"category_id"`
	PinnedGlobally     bool          `json:"pinned_globally"`
	FeaturedLink       string        `json:"featured_link"`
	Posters            []PosterModel `json:"posters"`
}

func (c *Client) Topic(ctx context.Context, id string) (*SingleTopicResponse, error) {
	response := new(SingleTopicResponse)
	if err := c.http.Get(ctx, TopicsBaseURL+"/"+id+".json", nil, response); err != nil {
		return nil, err
	}

	return response, nil
}

func (c *Client) TopicPosts(ctx context.Context, id string, postIDs []int) (*SingleTopicResponse, error) {
	query := url.Values{}
	for _, postID := range postIDs {
		query.Add("post_ids[]", strconv.Itoa(postID))
	}

	response := new(SingleTopicResponse)
	if err := c.http.Get(ctx, TopicsBaseURL+"/"+id+"/posts.json", query, response); err != nil {
		return nil, err
	}

	return response, nil
}

func (c *Client) Topics(
	ctx context.Context,
	list string,
	categorySlugPath string,
	categoryID int,
	query url.Values,
) (*TopicListResponse, error) {
	path := "/" + list + ".json"
	if categoryID > 0 {
		path = fmt.Sprintf("/c/%s/%d/l/%s.json", categorySlugPath, categoryID, list)
	}

	response := new(TopicListResponse)
	if err := c.http.Get(ctx, path, query, response); err != nil {
		return nil, err
	}

	return response, nil
}
