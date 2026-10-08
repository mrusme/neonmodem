package hyperuplink

import (
	"context"
	"fmt"
	"log/slog"
	"time"

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
	idx         int
	settings    system.Settings
	proxy       string
	logger      *slog.Logger
	readTimeout time.Duration
	client      *api.Client
}

func New(env system.Env) (system.System, error) {
	sys := &System{
		idx:         env.Index,
		settings:    env.Settings,
		proxy:       env.Proxy,
		logger:      env.Log(),
		readTimeout: env.ReadTimeout,
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

func (sys *System) URL() string {
	return sys.settings.URL
}

func (sys *System) Title() string {
	return system.HostTitle(sys.settings.URL)
}

func (sys *System) Description() string {
	return system.Describe("Hyperuplink", true)
}

func (sys *System) Capabilities() system.Capabilities {
	return system.AccountCapabilities(true)
}

func (sys *System) ListForums(ctx context.Context) ([]forum.Forum, error) {
	board, err := sys.client.Board(ctx)
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
	resp, err := sys.client.Topics(ctx, forumID, 1)
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
	return post.Post{
		ID:      t.ID,
		Subject: t.Name,
		Body:    t.Text,
		Kind:    post.KindText,

		Pinned: t.Pinned,
		Closed: t.LockedAt != "",

		CreatedAt: text.Time(t.CreatedAt),

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
		resp, err := sys.client.Topic(ctx, p.ID, page)
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
	flat := make([]reply.Reply, 0, len(replies))
	for _, r := range replies {
		flat = append(flat, reply.Reply{
			ID:        r.ID,
			PostID:    topicID,
			ParentID:  r.ReplyID,
			Body:      r.Text,
			Deleted:   r.DeletedAt != "",
			CreatedAt: text.Time(r.CreatedAt),
			Author: author.Author{
				ID:   r.AuthorID,
				Name: r.AuthorUsername,
			},
			SysIDX: sys.idx,
		})
	}

	return reply.Tree(flat)
}

func (sys *System) CreatePost(ctx context.Context, p *post.Post) error {
	created, err := sys.client.CreatePost(ctx, &api.NewPostModel{
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

	created, err := sys.client.CreateReply(ctx, r.PostID, &body)
	if err != nil {
		return err
	}

	r.ID = created.ID
	return nil
}
