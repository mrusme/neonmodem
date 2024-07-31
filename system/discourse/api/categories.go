package api

import (
	"context"
	"net/http"

	"github.com/guregu/null"
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
	Description                  null.String     `json:"description",omitempty`
	DescriptionText              null.String     `json:"description_text",omitempty`
	DescriptionExcerpt           null.String     `json:"description_excerpt",omitempty`
	TopicUrl                     null.String     `json:"topic_url",omitempty`
	ReadRestricted               bool            `json:"read_restricted"`
	Permission                   null.Int        `json:"permission",omitempty`
	NotificationLevel            int             `json:"notification_level"`
	CanEdit                      bool            `json:"can_edit"`
	TopicTemplate                null.String     `json:"topic_template",omitempty`
	HasChildren                  null.Bool       `json:"has_children",omitempty`
	SortOrder                    null.String     `json:"sort_order",omitempty`
	SortAscending                null.Bool       `json:"sort_ascending",omitempty`
	ShowSubcategoryList          bool            `json:"show_subcategory_list"`
	NumFeaturedTopics            int             `json:"num_featured_topics"`
	DefaultView                  null.String     `json:"default_view",omitempty`
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
	SubcategoryIDs               []int           `json:"subcategory_ids"`
	SubcategoryList              []CategoryModel `json:"subcategory_list"`
	UploadedLogo                 null.String     `json:"uploaded_logo",omitempty`
	UploadedLogoDark             null.String     `json:"uploaded_logo_dark",omitempty`
	UploadedBackground           null.String     `json:"uploaded_background",omitempty`
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

	q := req.URL.Query()
	q.Add("include_subcategories", "true")
	req.URL.RawQuery = q.Encode()

	response := new(LatestCategoriesResponse)
	if err = a.client.Do(ctx, req, response); err != nil {
		return nil, err
	}

	return response, nil
}
