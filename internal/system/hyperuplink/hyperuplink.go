package hyperuplink

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/mrusme/neonmodem/internal/models/author"
	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/httpx"
	"github.com/mrusme/neonmodem/internal/system/hyperuplink/api"
	"github.com/mrusme/neonmodem/internal/system/text"
)

const Kind = "hyperuplink"

type System struct {
	idx      int
	settings system.Settings
	proxy    string
	logger   *slog.Logger
	client   *api.Client
}

func New(env system.Env) (system.System, error) {
	sys := &System{
		idx:      env.Index,
		settings: env.Settings,
		proxy:    env.Proxy,
		logger:   env.Log(),
	}

	if env.Settings.URL != "" {
		client, err := newClient(env.Settings.URL, env.Settings.Credential(system.CredentialToken), env.Proxy, sys.logger)
		if err != nil {
			return nil, err
		}
		sys.client = client
	}

	return sys, nil
}

func newClient(endpoint string, token string, proxy string, logger *slog.Logger) (*api.Client, error) {
	httpClient := httpx.NewHTTPClient(httpx.Options{
		Proxy:   proxy,
		Retries: 3,
		Logger:  logger,
	})
	return api.NewClient(httpClient, endpoint, token)
}

func (sys *System) Kind() string {
	return Kind
}

func (sys *System) Title() string {
	u, err := url.Parse(sys.settings.URL)
	if err != nil || u.Hostname() == "" {
		return sys.settings.URL
	}
	return u.Hostname()
}

func (sys *System) Description() string {
	return "Hyperuplink"
}

func (sys *System) Capabilities() system.Capabilities {
	return system.CapRead | system.CapWrite
}

func (sys *System) ListForums(ctx context.Context) ([]forum.Forum, error) {
	board, err := sys.client.Board.Get(ctx)
	if err != nil {
		return nil, err
	}

	var models []forum.Forum
	for _, cf := range board.CategoriesForums {
		for _, f := range cf.Forums {
			models = append(models, forum.Forum{
				ID:     f.ID,
				Name:   fmt.Sprintf("%s/%s", cf.Category.Name, f.Name),
				Info:   f.Description,
				SysIDX: sys.idx,
			})
		}
	}

	return models, nil
}

func (sys *System) Orders(forumID string) system.Ordering {
	return system.Only(system.OrderActive)
}

func (sys *System) ListPosts(
	ctx context.Context,
	forumID string,
	order system.Order,
) ([]post.Post, error) {
	resp, err := sys.client.Topics.List(ctx, forumID, 1)
	if err != nil {
		return nil, err
	}

	models := make([]post.Post, 0, len(resp.Topics))
	for i := range resp.Topics {
		models = append(models, sys.topicToPost(&resp.Topics[i]))
	}

	return models, nil
}

func (sys *System) topicToPost(t *api.TopicModel) post.Post {
	createdAt := text.Time(t.CreatedAt)

	lastCommentedAt := createdAt
	if t.LastReplyAt != "" {
		if parsed := text.Time(t.LastReplyAt); !parsed.IsZero() {
			lastCommentedAt = parsed
		}
	}

	return post.Post{
		ID:      t.ID,
		Subject: t.Name,
		Body:    t.Text,
		Kind:    post.KindText,

		Pinned: t.Pinned,
		Closed: t.LockedAt != "",

		CreatedAt:       createdAt,
		LastCommentedAt: lastCommentedAt,

		Author: author.Author{
			ID:   t.AuthorID,
			Name: t.AuthorUsername,
		},

		Forum: forum.Forum{
			ID:     t.ForumID,
			Name:   fmt.Sprintf("%s/%s", t.CategoryName, t.ForumName),
			SysIDX: sys.idx,
		},

		ReplyCount: t.Replies,

		URL: fmt.Sprintf("%s/_%s/%s/%s",
			sys.settings.URL, t.CategorySlug, t.ForumSlug, t.Slug),

		SysIDX: sys.idx,
	}
}

func (sys *System) LoadPost(ctx context.Context, p *post.Post) error {
	var allReplies []api.ReplyModel

	page := 1
	for {
		resp, err := sys.client.Topics.Show(ctx, p.ID, page)
		if err != nil {
			return err
		}

		if page == 1 {
			p.Body = resp.Topic.Text
			p.Pinned = resp.Topic.Pinned
			p.Closed = resp.Topic.LockedAt != ""
		}

		allReplies = append(allReplies, resp.Replies...)

		if resp.Pages <= page {
			break
		}
		page++
	}

	p.Replies = sys.buildReplyTree(p.ID, allReplies)
	p.ReplyCount = len(allReplies)

	return nil
}

func (sys *System) buildReplyTree(
	topicID string,
	replies []api.ReplyModel,
) []reply.Reply {
	present := make(map[string]bool, len(replies))
	for _, r := range replies {
		present[r.ID] = true
	}

	childrenOf := make(map[string][]api.ReplyModel)
	for _, r := range replies {
		parent := r.ReplyID
		if parent != "" && !present[parent] {
			parent = ""
		}
		childrenOf[parent] = append(childrenOf[parent], r)
	}

	var build func(parentID string) []reply.Reply
	build = func(parentID string) []reply.Reply {
		out := []reply.Reply{}
		for _, r := range childrenOf[parentID] {
			out = append(out, reply.Reply{
				ID:       r.ID,
				PostID:   topicID,
				ParentID: parentID,

				Body: r.Text,

				Deleted: r.DeletedAt != "",

				CreatedAt: text.Time(r.CreatedAt),

				Author: author.Author{
					ID:   r.AuthorID,
					Name: r.AuthorUsername,
				},

				Replies: build(r.ID),

				SysIDX: sys.idx,
			})
		}
		return out
	}

	return build("")
}

func (sys *System) CreatePost(ctx context.Context, p *post.Post) error {
	created, err := sys.client.Posts.Create(ctx, &api.NewPostModel{
		Name:    p.Subject,
		Text:    p.Body,
		ForumID: p.Forum.ID,
		Kind:    "regular",
	})
	if err != nil {
		return err
	}

	p.ID = created.ID
	return nil
}

func (sys *System) CreateReply(ctx context.Context, r *reply.Reply) error {
	body := api.CreateReplyModel{
		Text:    r.Body,
		ReplyID: r.ParentID,
	}

	created, err := sys.client.Topics.CreateReply(ctx, r.PostID, &body)
	if err != nil {
		return err
	}

	r.ID = created.ID
	return nil
}
