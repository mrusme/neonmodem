package discourse

import (
	"context"
	"net/http"
)

const CategoriesBaseURL = "/categories"

type LatestCategoriesResponse struct {
	CategoryList struct {
		CanCreateCategory bool `json:"can_create_category"`
		CanCreateTopic    bool `json:"can_create_topic"`

		Categories []CategoryModel `json:"categories"`
	} `json:"category_list"`
}

type CategoryModel struct {
	ID                           int             `json:"id"`
	Name                         string          `json:"name"`
	Color                        string          `json:"color"`
	TextColor                    string          `json:"text_color"`
	Slug                         string          `json:"slug"`
	TopicCount                   int             `json:"topic_count"`
	PostCount                    int             `json:"post_count"`
	Position                     int             `json:"position"`
	Description                  string          `json:"description"`
	DescriptionText              string          `json:"description_text"`
	DescriptionExcerpt           string          `json:"description_excerpt"`
	TopicUrl                     string          `json:"topic_url"`
	ReadRestricted               bool            `json:"read_restricted"`
	Permission                   int             `json:"permission"`
	NotificationLevel            int             `json:"notification_level"`
	CanEdit                      bool            `json:"can_edit"`
	TopicTemplate                string          `json:"topic_template"`
	HasChildren                  bool            `json:"has_children"`
	SortOrder                    string          `json:"sort_order"`
	SortAscending                string          `json:"sort_ascending"`
	ShowSubcategoryList          bool            `json:"show_subcategory_list"`
	NumFeaturedTopics            int             `json:"num_featured_topics"`
	DefaultView                  string          `json:"default_view"`
	SubcategoryListStyle         string          `json:"subcategory_list_style"`
	DefaultTopPeriod             string          `json:"default_top_period"`
	DefaultListFilter            string          `json:"default_list_filter"`
	MinimumRequiredTags          int             `json:"minimum_required_tags"`
	NavigateToFirstPostAfterRead bool            `json:"navigate_to_first_post_after_read"`
	TopicsDay                    int             `json:"topics_day"`
	TopicsWeek                   int             `json:"topics_week"`
	TopicsMonth                  int             `json:"topics_month"`
	TopicsYear                   int             `json:"topics_year"`
	TopicsAllTime                int             `json:"topics_all_time"`
	IsUncategorized              bool            `json:"is_uncategorized"`
	SubcategoryIDs               []string        `json:"subcategory_ids"`
	SubcategoryList              []CategoryModel `json:"subcategory_list"`
	UploadedLogo                 string          `json:"uploaded_logo"`
	UploadedLogoDark             string          `json:"uploaded_logo_dark"`
	UploadedBackground           string          `json:"uploaded_background"`
}

type CategoriesService interface {
	List(
		ctx context.Context,
	) (*LatestCategoriesResponse, error)
}

type CategoryServiceHandler struct {
	client *Client
}

// List
func (a *CategoryServiceHandler) List(
	ctx context.Context,
) (*LatestCategoriesResponse, error) {
	uri := CategoriesBaseURL + ".json"

	req, err := a.client.NewRequest(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	response := new(LatestCategoriesResponse)
	if err = a.client.Do(ctx, req, response); err != nil {
		return nil, err
	}

	return response, nil
}
