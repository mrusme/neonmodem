package hyperuplink

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
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

const (
	Kind = "hyperuplink"

	clientRetries = 3
	replyPageSize = 100
	topicKind     = "regular"
)

var sorts = map[system.Order]string{
	system.OrderNew:      api.SortNew,
	system.OrderActive:   api.SortActive,
	system.OrderComments: api.SortReplies,
}

type System struct {
	idx         int
	settings    system.Settings
	origin      string
	proxy       string
	logger      *slog.Logger
	readTimeout time.Duration
	client      *api.Client

	sessionMu sync.Mutex
	ended     error
}

func New(env system.Env) (system.System, error) {
	origin, _ := apiOrigin(env.Settings.URL)
	sys := &System{
		idx:         env.Index,
		settings:    env.Settings,
		origin:      origin,
		proxy:       env.Proxy,
		logger:      env.Log(),
		readTimeout: env.ReadTimeout,
	}

	if origin != "" {
		client, err := newClient(origin, sys.token(), env.Proxy, sys.logger, clientRetries)
		if err != nil {
			return nil, err
		}
		sys.client = client
	}

	return sys, nil
}

func newClient(origin string, token string, proxy string, logger *slog.Logger, retries int) (*api.Client, error) {
	httpClient := httpx.NewHTTPClient(httpx.Options{
		Proxy:   proxy,
		Retries: retries,
		Logger:  logger,
	})
	return api.NewClient(httpClient, origin, token)
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
	return system.Describe("Hyperuplink", sys.hasAccount())
}

func (sys *System) Capabilities() system.Capabilities {
	return system.AccountCapabilities(sys.hasAccount())
}

func (sys *System) token() string {
	return sys.settings.Credential(system.CredentialToken)
}

func (sys *System) hasAccount() bool {
	return sys.token() != ""
}

func (sys *System) connectCommand() string {
	return "`" + system.ConnectCommand(Kind, sys.settings.URL) + "`"
}

func (sys *System) sessionErr() error {
	sys.sessionMu.Lock()
	defer sys.sessionMu.Unlock()
	return sys.ended
}

func (sys *System) writable() error {
	if !sys.hasAccount() {
		return system.NoCredentials(Kind, sys.settings.URL)
	}
	return sys.sessionErr()
}

func (sys *System) fail(err error) error {
	switch {
	case err == nil:
		return nil
	case httpx.StatusOf(err) == http.StatusUnauthorized:
		return sys.end(err)
	case errors.Is(err, api.ErrNotAnAPI):
		return fmt.Errorf("%w; the board may run a Hyperuplink older than its API %s", err, api.Version)
	}
	return err
}

func (sys *System) end(cause error) error {
	sys.sessionMu.Lock()
	defer sys.sessionMu.Unlock()

	if sys.ended == nil {
		host := system.HostTitle(sys.settings.URL)
		message := fmt.Sprintf("the API key for %s was rejected; connect again with %s",
			host, sys.connectCommand())
		problem, isProblem := api.ProblemOf(cause)
		if !sys.hasAccount() || (isProblem && problem.Code == api.CodeAuthenticationRequired) {
			message = fmt.Sprintf("%s no longer serves guests; connect with an account using %s",
				host, sys.connectCommand())
		}
		sys.ended = system.NeedsConnect(message)
		sys.logger.Warn("the connection was rejected", "url", sys.settings.URL, "error", cause)
	}
	return sys.ended
}

func forumName(category string, name string) string {
	return category + "/" + name
}

func (sys *System) ListForums(ctx context.Context) ([]forum.Forum, error) {
	if err := sys.sessionErr(); err != nil {
		return nil, err
	}

	forums, err := sys.client.Forums(ctx)
	if err != nil {
		return nil, sys.fail(err)
	}

	models := make([]forum.Forum, 0, len(forums))
	for _, f := range forums {
		models = append(models, forum.Forum{
			ID:     f.ID,
			Name:   forumName(f.Category.Name, f.Name),
			Info:   f.Description,
			SysIDX: sys.idx,
		})
	}

	return models, nil
}

func (sys *System) Orders(string) system.Ordering {
	return system.Ordering{
		Default:   system.OrderActive,
		Supported: []system.Order{system.OrderNew, system.OrderActive, system.OrderComments},
	}
}

func (sys *System) ListPosts(
	ctx context.Context,
	forumID string,
	order system.Order,
) ([]post.Post, error) {
	if err := sys.sessionErr(); err != nil {
		return nil, err
	}
	sort, ok := sorts[order]
	if !ok {
		return nil, system.ErrOrderUnavailable
	}

	resp, err := sys.client.Topics(ctx, forumID, sort, 1)
	if err != nil {
		return nil, sys.fail(err)
	}

	models := make([]post.Post, 0, len(resp.Topics))
	for i := range resp.Topics {
		models = append(models, sys.topicToPost(&resp.Topics[i]))
	}

	return models, nil
}

func (sys *System) topicToPost(t *api.Topic) post.Post {
	return post.Post{
		ID:      t.ID,
		Subject: t.Name,
		Body:    describe(sys.origin, t.Text, t.Poll, t.Attachments),
		Kind:    post.KindText,

		Pinned: t.Pinned,
		Closed: t.LockedAt != "",

		CreatedAt: text.Time(t.CreatedAt),

		Author: author.Author{
			ID:   t.Author.ID,
			Name: t.Author.Username,
		},

		Forum: forum.Forum{
			ID:     t.Forum.ID,
			Name:   forumName(t.Category.Name, t.Forum.Name),
			SysIDX: sys.idx,
		},

		ReplyCount: t.Replies,

		URL: t.URL,

		SysIDX: sys.idx,
	}
}

func (sys *System) LoadPost(ctx context.Context, p *post.Post) error {
	if err := sys.sessionErr(); err != nil {
		return err
	}

	topic, err := sys.client.Topic(ctx, p.ID)
	if err != nil {
		return sys.fail(err)
	}
	p.Body = describe(sys.origin, topic.Text, topic.Poll, topic.Attachments)
	p.Pinned = topic.Pinned
	p.Closed = topic.LockedAt != ""

	var replies []api.Reply
	var total int64
	for page := 1; ; page++ {
		resp, err := sys.client.Replies(ctx, p.ID, page, replyPageSize)
		if err != nil {
			return sys.fail(err)
		}

		replies = append(replies, resp.Replies...)
		total = resp.Pagination.Total
		if resp.Pagination.Pages <= page {
			break
		}
	}

	p.Replies = sys.buildReplyTree(p.ID, replies)
	p.ReplyCount = int(total)

	return nil
}

func (sys *System) buildReplyTree(topicID string, replies []api.Reply) []reply.Reply {
	flat := make([]reply.Reply, 0, len(replies))
	for i := range replies {
		r := &replies[i]
		flat = append(flat, reply.Reply{
			ID:        r.ID,
			PostID:    topicID,
			ParentID:  r.ParentID,
			Body:      describe(sys.origin, r.Text, nil, r.Attachments),
			CreatedAt: text.Time(r.CreatedAt),
			Author: author.Author{
				ID:   r.Author.ID,
				Name: r.Author.Username,
			},
			SysIDX: sys.idx,
		})
	}

	return reply.Tree(flat)
}

func (sys *System) CreatePost(ctx context.Context, p *post.Post) error {
	if err := sys.writable(); err != nil {
		return err
	}

	created, err := sys.client.CreateTopic(ctx, &api.NewTopic{
		ForumID: p.Forum.ID,
		Name:    p.Subject,
		Text:    p.Body,
		Kind:    topicKind,
	})
	if err != nil {
		return sys.fail(err)
	}

	p.ID = created.ID
	p.URL = created.URL
	return nil
}

func (sys *System) CreateReply(ctx context.Context, r *reply.Reply) error {
	if err := sys.writable(); err != nil {
		return err
	}

	created, err := sys.client.CreateReply(ctx, r.PostID, &api.NewReply{
		Text:     r.Body,
		ParentID: r.ParentID,
	})
	if err != nil {
		return sys.fail(err)
	}

	r.ID = created.ID
	return nil
}

func (sys *System) AuthorizeMedia(req *http.Request) {
	token := sys.token()
	if token == "" || !sameOrigin(req.URL, sys.origin) {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
}

func (sys *System) Revoke(ctx context.Context, old system.Settings) error {
	origin, _ := apiOrigin(old.URL)
	client, err := newClient(origin, old.Credential(system.CredentialToken), sys.proxy, sys.logger, 0)
	if err != nil {
		return err
	}

	bounded, cancel := system.Bound(ctx, sys.readTimeout)
	defer cancel()
	return system.Timeout(ctx, sys.readTimeout, client.Revoke(bounded))
}
