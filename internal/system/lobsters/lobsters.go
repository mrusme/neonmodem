package lobsters

import (
	"context"
	"log/slog"
	"time"

	"github.com/mrusme/neonmodem/internal/models/author"
	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/httpx"
	"github.com/mrusme/neonmodem/internal/system/lobsters/api"
	"github.com/mrusme/neonmodem/internal/system/text"
)

const Kind = "lobsters"

type System struct {
	idx         int
	settings    system.Settings
	proxy       string
	logger      *slog.Logger
	readTimeout time.Duration
	client      *api.Client
	web         *webSession
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
		if err := sys.connectClient(); err != nil {
			return nil, err
		}
	}

	return sys, nil
}

func (sys *System) connectClient() error {
	httpClient := httpx.NewHTTPClient(httpx.Options{
		Proxy:   sys.proxy,
		Retries: 3,
		Logger:  sys.logger,
	})

	client, err := api.NewClient(httpClient, sys.settings.URL)
	if err != nil {
		return err
	}
	sys.client = client

	if sys.hasAccount() {
		web, err := newWebSession(
			sys.settings.URL,
			sys.settings.Credential(system.CredentialUsername),
			sys.settings.Credential(system.CredentialPassword),
			sys.proxy,
			sys.logger,
		)
		if err != nil {
			return err
		}
		sys.web = web
	}

	return nil
}

func (sys *System) hasAccount() bool {
	return sys.settings.Credential(system.CredentialUsername) != "" &&
		sys.settings.Credential(system.CredentialPassword) != ""
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
	return system.Describe("Lobsters", sys.hasAccount())
}

func (sys *System) Capabilities() system.Capabilities {
	return system.AccountCapabilities(sys.hasAccount())
}

func (sys *System) ListForums(ctx context.Context) ([]forum.Forum, error) {
	tags, err := sys.client.Tags(ctx)
	if err != nil {
		return nil, err
	}

	var models []forum.Forum
	for _, tag := range tags {
		if !tag.Active {
			continue
		}
		models = append(models, forum.Forum{
			ID:     tag.Tag,
			Name:   tag.Tag,
			Info:   tag.Description,
			SysIDX: sys.idx,
		})
	}

	return models, nil
}

func (sys *System) Orders(forumID string) system.Ordering {
	if forumID != "" {
		return system.Only(system.OrderNew)
	}
	return system.Ordering{
		Default:   system.OrderNew,
		Supported: []system.Order{system.OrderNew, system.OrderActive, system.OrderHot},
	}
}

func storyList(order system.Order) string {
	switch order {
	case system.OrderActive:
		return "active"
	case system.OrderHot:
		return "hottest"
	default:
		return "newest"
	}
}

func (sys *System) ListPosts(
	ctx context.Context,
	forumID string,
	order system.Order,
) ([]post.Post, error) {
	var items []api.StoryModel
	var err error
	if forumID != "" {
		items, err = sys.client.Tagged(ctx, forumID)
	} else {
		items, err = sys.client.Stories(ctx, storyList(order))
	}
	if err != nil {
		return nil, err
	}

	models := make([]post.Post, 0, len(items))
	for i := range items {
		p := sys.toPost(&items[i])
		if forumID != "" {
			p.Forum = forum.Forum{ID: forumID, Name: forumID, SysIDX: sys.idx}
		}
		models = append(models, p)
	}

	return models, nil
}

func (sys *System) toPost(s *api.StoryModel) post.Post {
	kind, body := post.LinkBody(s.URL, text.Markdown(s.Description))

	tag := ""
	if len(s.Tags) > 0 {
		tag = s.Tags[0]
	}

	postURL := s.CommentsURL
	if postURL == "" {
		postURL = s.ShortIDURL
	}

	return post.Post{
		ID:      s.ShortID,
		Subject: s.Title,
		Body:    body,
		Kind:    kind,

		CreatedAt: text.Time(s.CreatedAt),

		Author: author.Author{
			ID:   s.SubmitterUser,
			Name: s.SubmitterUser,
		},

		Forum: forum.Forum{
			ID:     tag,
			Name:   tag,
			SysIDX: sys.idx,
		},

		ReplyCount: s.CommentCount,
		Score:      post.Score{Value: s.Score, Unit: post.ScorePoints},

		URL:  postURL,
		Link: s.URL,

		SysIDX: sys.idx,
	}
}

func (sys *System) LoadPost(ctx context.Context, p *post.Post) error {
	item, err := sys.client.Story(ctx, p.ID)
	if err != nil {
		return err
	}

	p.Refresh(sys.toPost(item))
	p.Replies = sys.buildTree(p.ID, item.Comments)

	return nil
}

func (sys *System) buildTree(storyID string, comments []api.CommentModel) []reply.Reply {
	replies := make([]reply.Reply, 0, len(comments))
	for _, c := range comments {
		body := text.Markdown(c.Comment)
		if body == "" {
			body = c.CommentPlain
		}

		replies = append(replies, reply.Reply{
			ID:        c.ShortID,
			PostID:    storyID,
			ParentID:  c.ParentComment,
			Body:      body,
			Deleted:   c.IsDeleted || c.IsModerated,
			CreatedAt: text.Time(c.CreatedAt),
			Author: author.Author{
				ID:   c.CommentingUser,
				Name: c.CommentingUser,
			},
			SysIDX: sys.idx,
		})
	}

	return reply.Tree(replies)
}

func (sys *System) CreatePost(ctx context.Context, p *post.Post) error {
	if !sys.Capabilities().Has(system.CapCreatePost) {
		return system.NoCredentials(Kind, sys.settings.URL)
	}

	id, err := sys.web.SubmitStory(ctx, p.Subject, p.Body, p.Kind == post.KindLink, p.Forum.ID)
	if err != nil {
		return err
	}

	p.ID = id
	return nil
}

func (sys *System) CreateReply(ctx context.Context, r *reply.Reply) error {
	if !sys.Capabilities().Has(system.CapCreateReply) {
		return system.NoCredentials(Kind, sys.settings.URL)
	}

	id, err := sys.web.PostComment(ctx, r.PostID, r.ParentID, r.Body)
	if err != nil {
		return err
	}

	r.ID = id
	return nil
}
