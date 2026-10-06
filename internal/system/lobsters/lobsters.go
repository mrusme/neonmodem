package lobsters

import (
	"context"
	"log/slog"
	"net/url"

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
	idx      int
	settings system.Settings
	proxy    string
	logger   *slog.Logger
	client   *api.Client
	web      *webSession
}

func New(env system.Env) (system.System, error) {
	sys := &System{
		idx:      env.Index,
		settings: env.Settings,
		proxy:    env.Proxy,
		logger:   env.Log(),
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

func (sys *System) Title() string {
	u, err := url.Parse(sys.settings.URL)
	if err != nil || u.Hostname() == "" {
		return sys.settings.URL
	}
	return u.Hostname()
}

func (sys *System) Description() string {
	if !sys.hasAccount() {
		return "Lobsters (read-only)"
	}
	return "Lobsters"
}

func (sys *System) Capabilities() system.Capabilities {
	caps := system.CapRead
	if sys.hasAccount() {
		caps |= system.CapWrite
	}
	return caps
}

func (sys *System) ListForums(ctx context.Context) ([]forum.Forum, error) {
	tags, err := sys.client.Tags.List(ctx)
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

func (sys *System) ListPosts(ctx context.Context, forumID string) ([]post.Post, error) {
	items, err := sys.client.Stories.List(ctx, forumID)
	if err != nil {
		return nil, err
	}

	models := make([]post.Post, 0, len(items))
	for i := range items {
		models = append(models, sys.toPost(&items[i]))
	}

	return models, nil
}

func (sys *System) toPost(s *api.StoryModel) post.Post {
	kind := post.KindText
	body := text.Markdown(s.Description)
	if s.URL != "" {
		kind = post.KindLink
		if body == "" {
			body = s.URL
		} else {
			body = s.URL + "\n\n" + body
		}
	}

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

		URL: postURL,

		SysIDX: sys.idx,
	}
}

func (sys *System) LoadPost(ctx context.Context, p *post.Post) error {
	item, err := sys.client.Stories.Show(ctx, p.ID)
	if err != nil {
		return err
	}

	fresh := sys.toPost(item)
	p.Subject = fresh.Subject
	p.Body = fresh.Body
	p.Kind = fresh.Kind
	p.Author = fresh.Author
	p.ReplyCount = fresh.ReplyCount
	p.URL = fresh.URL

	p.Replies = sys.buildTree(p.ID, item.Comments)

	return nil
}

func (sys *System) buildTree(storyID string, comments []api.CommentModel) []reply.Reply {
	nodes := make([]reply.Reply, 0, len(comments))
	index := make(map[string]int, len(comments))
	for _, c := range comments {
		body := text.Markdown(c.Comment)
		if body == "" {
			body = c.CommentPlain
		}

		nodes = append(nodes, reply.Reply{
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
		index[c.ShortID] = len(nodes) - 1
	}

	children := make(map[int][]int)
	var roots []int
	for i, n := range nodes {
		if parentIdx, ok := index[n.ParentID]; ok && n.ParentID != "" {
			children[parentIdx] = append(children[parentIdx], i)
		} else {
			nodes[i].ParentID = ""
			roots = append(roots, i)
		}
	}

	var build func(indexes []int) []reply.Reply
	build = func(indexes []int) []reply.Reply {
		out := make([]reply.Reply, 0, len(indexes))
		for _, i := range indexes {
			r := nodes[i]
			r.Replies = build(children[i])
			out = append(out, r)
		}
		return out
	}

	return build(roots)
}

func (sys *System) CreatePost(ctx context.Context, p *post.Post) error {
	if sys.web == nil {
		return system.ErrNoCredentials
	}

	id, err := sys.web.SubmitStory(ctx, p.Subject, p.Body, p.Kind == post.KindLink, p.Forum.ID)
	if err != nil {
		return err
	}

	p.ID = id
	return nil
}

func (sys *System) CreateReply(ctx context.Context, r *reply.Reply) error {
	if sys.web == nil {
		return system.ErrNoCredentials
	}

	id, err := sys.web.PostComment(ctx, r.PostID, r.ParentID, r.Body)
	if err != nil {
		return err
	}

	r.ID = id
	return nil
}
