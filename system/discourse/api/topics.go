package api

import (
	"context"
	"net/http"
)

const TopicsBaseURL = "/t"

type LatestTopicsResponse struct {
	Users []struct {
		ID             int    `json:"id"`
		Username       string `json:"username"`
		Name           string `json:"name"`
		AvatarTemplate string `json:"avatar_template"`
	}

	TopicList struct {
		CanCreateTopic bool   `json:"can_create_topic"`
		Draft          string `json:"draft"`
		DraftKey       string `json:"draft_key"`
		DraftSequence  int    `json:"draft_sequence"`
		PerPage        int    `json:"per_page"`

		Topics []TopicModel `json:"topics"`
	} `json:"topic_list"`
}

type SingleTopicResponse struct {
	PostStream struct {
		Posts []PostModel `json:"posts"`
	} `json:"post_stream"`

	TopicModel
}

type TopicModel struct {
	ID                 int    `json:"id"`
	Title              string `json:"title"`
	FancyTitle         string `json:"fancy_title"`
	Slug               string `json:"slug"`
	PostsCount         int    `json:"posts_count"`
	ReplyCount         int    `json:"reply_count"`
	HighestPostNumber  int    `json:"highest_post_number"`
	ImageURL           string `json:"image_url"`
	CreatedAt          string `json:"created_at"`
	LastPostedAt       string `json:"last_posted_at"`
	Bumped             bool   `json:"bumped"`
	BumpedAt           string `json:"bumped_at"`
	Archetype          string `json:"archetype"`
	Unseen             bool   `json:"unseen"`
	LastReadPostNumber int    `json:"last_read_post_number"`
	UnreadPosts        int    `json:"unread_posts"`
	Pinned             bool   `json:"pinned"`
	Unpinned           string `json:"unpinned"`
	Visible            bool   `json:"visible"`
	Closed             bool   `json:"closed"`
	Archived           bool   `json:"archived"`
	NotificationLevel  int    `json:"notification_level"`
	Bookmarked         bool   `json:"bookmarked"`
	Liked              bool   `json:"liked"`
	Views              int    `json:"views"`
	LikeCount          int    `json:"like_count"`
	HasSummary         bool   `json:"has_summary"`
	LastPosterUsername string `json:"last_poster_username"`
	CategoryID         int    `json:"category_id"`
	OpLikeCount        int    `json:"op_like_count"`
	PinnedGlobally     bool   `json:"pinned_globally"`
	FeaturedLink       string `json:"featured_link"`
	Posters            []struct {
		Extras         string `json:"extras"`
		Description    string `json:"description"`
		UserID         int    `json:"user_id"`
		PrimaryGroupID string `json:"primary_group_id"`
	} `json:"posters"`
}

type TopicsService interface {
	Show(
		ctx context.Context,
		id string,
	) (*SingleTopicResponse, error)
	ListLatest(
		ctx context.Context,
	) (*LatestTopicsResponse, error)
}

type TopicServiceHandler struct {
	client *Client
}

// Show
func (a *TopicServiceHandler) Show(
	ctx context.Context,
	id string,
) (*SingleTopicResponse, error) {
	uri := TopicsBaseURL + "/" + id + ".json"

	req, err := a.client.NewRequest(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	response := new(SingleTopicResponse)
	if err = a.client.Do(ctx, req, response); err != nil {
		return nil, err
	}

	return response, nil
}

// List
func (a *TopicServiceHandler) ListLatest(
	ctx context.Context,
) (*LatestTopicsResponse, error) {
	uri := "/latest.json"

	req, err := a.client.NewRequest(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	response := new(LatestTopicsResponse)
	if err = a.client.Do(ctx, req, response); err != nil {
		return nil, err
	}

	return response, nil
}
