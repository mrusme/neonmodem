package lemmy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mrusme/neonmodem/internal/models/author"
	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/httpx"
	"github.com/mrusme/neonmodem/internal/system/lemmy/api"
	"github.com/mrusme/neonmodem/internal/system/text"
)

const (
	Kind = "lemmy"

	OptionListing    = "listing"
	OptionMarkAsRead = "mark_as_read"

	ListingSubscribed = "subscribed"
	ListingLocal      = "local"
	ListingAll        = "all"

	optionYes = "yes"
	optionNo  = "no"

	commentDepth = 8
	pageSize     = 50
	maxPages     = 10
)

type System struct {
	idx         int
	settings    system.Settings
	proxy       string
	logger      *slog.Logger
	readTimeout time.Duration
	markAsRead  bool
	client      *api.Client

	sessionMu sync.Mutex
	ended     error
}

func New(env system.Env) (system.System, error) {
	markAsRead, err := markAsRead(env.Settings)
	if err != nil {
		return nil, err
	}

	sys := &System{
		idx:         env.Index,
		settings:    env.Settings,
		proxy:       env.Proxy,
		logger:      env.Log(),
		readTimeout: env.ReadTimeout,
		markAsRead:  markAsRead,
	}

	if env.Settings.URL != "" {
		client, err := newClient(env.Settings.URL, sys.token(), env.Proxy, sys.logger, 3)
		if err != nil {
			return nil, err
		}
		sys.client = client
	}

	return sys, nil
}

func newClient(
	sysURL string,
	token string,
	proxy string,
	logger *slog.Logger,
	retries int,
) (*api.Client, error) {
	return api.NewClient(api.Config{
		URL:   sysURL,
		Token: token,
		Read: httpx.NewHTTPClient(httpx.Options{
			Proxy:   proxy,
			Retries: retries,
			Logger:  logger,
		}),
		Write: httpx.NewHTTPClient(httpx.Options{
			Proxy:  proxy,
			Logger: logger,
		}),
	})
}

func markAsRead(settings system.Settings) (bool, error) {
	value := settings.Option(OptionMarkAsRead, optionNo)
	switch strings.ToLower(strings.TrimSpace(value)) {
	case optionYes:
		return true, nil
	case optionNo:
		return false, nil
	default:
		return false, fmt.Errorf("the option %s is %q; use %s or %s",
			OptionMarkAsRead, value, optionYes, optionNo)
	}
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
	return system.Describe("Lemmy", sys.hasAccount())
}

func (sys *System) username() string {
	return sys.settings.Credential(system.CredentialUsername)
}

func (sys *System) token() string {
	return sys.settings.Credential(system.CredentialToken)
}

func (sys *System) hasAccount() bool {
	return sys.username() != "" || sys.token() != ""
}

func (sys *System) Capabilities() system.Capabilities {
	return system.AccountCapabilities(sys.hasAccount())
}

func (sys *System) listingType() api.Listing {
	listing := sys.settings.Option(OptionListing, ListingSubscribed)
	switch strings.ToLower(listing) {
	case ListingAll:
		return api.ListingAll
	case ListingLocal:
		return api.ListingLocal
	default:
		if !sys.hasAccount() {
			return api.ListingLocal
		}
		return api.ListingSubscribed
	}
}

func (sys *System) connectCommand() string {
	return "`" + system.ConnectCommand(Kind, sys.settings.URL) + "`"
}

func (sys *System) sessionErr() error {
	if sys.username() != "" && sys.token() == "" {
		return system.NeedsConnect(fmt.Sprintf(
			"there's no session token for %s; log in with %s",
			sys.username(), sys.connectCommand()))
	}

	sys.sessionMu.Lock()
	defer sys.sessionMu.Unlock()
	return sys.ended
}

func (sys *System) end() error {
	sys.sessionMu.Lock()
	defer sys.sessionMu.Unlock()

	if sys.ended == nil {
		whose := "the session"
		if sys.username() != "" {
			whose = "the session of " + sys.username()
		}
		sys.ended = system.NeedsConnect(fmt.Sprintf(
			"%s has ended; log in again with %s", whose, sys.connectCommand()))
		sys.logger.Warn("the session has ended", "url", sys.settings.URL)
	}
	return sys.ended
}

func (sys *System) listing(ctx context.Context, call func(ctx context.Context) error) error {
	if err := sys.sessionErr(); err != nil {
		return err
	}
	if _, err := sys.client.Version(ctx); err != nil {
		return err
	}
	if sys.token() == "" {
		return call(ctx)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	checked := make(chan error, 1)
	go func() {
		err := sys.client.ValidateAuth(ctx)
		if err != nil {
			cancel()
		}
		checked <- err
	}()

	err := call(ctx)
	if checkErr := <-checked; checkErr != nil {
		if errors.Is(checkErr, api.ErrSessionInvalid) {
			return sys.end()
		}
		return fmt.Errorf("checking the session: %w", checkErr)
	}
	return err
}

func (sys *System) write(ctx context.Context, call func(ctx context.Context) error) error {
	if err := sys.sessionErr(); err != nil {
		return err
	}

	err := call(ctx)
	if err != nil && sys.token() != "" && errors.Is(err, api.ErrSessionInvalid) {
		return sys.end()
	}
	return err
}

func (sys *System) ListForums(ctx context.Context) ([]forum.Forum, error) {
	var communities []api.Community
	err := sys.listing(ctx, func(ctx context.Context) error {
		var err error
		communities, err = sys.client.Communities(ctx, sys.listingType(), pageSize, maxPages)
		return err
	})
	if err != nil {
		return nil, err
	}

	var models []forum.Forum
	for _, c := range communities {
		models = append(models, forum.Forum{
			ID:     strconv.FormatInt(c.ID, 10),
			Name:   c.Name,
			Info:   text.FirstNonEmpty(text.FirstParagraph(c.Summary), text.FirstParagraph(c.Description), c.Title),
			SysIDX: sys.idx,
		})
	}

	return models, nil
}

func (sys *System) Orders(string) system.Ordering {
	return system.Ordering{Default: system.OrderNew, Supported: system.AllOrders()}
}

func sortType(order system.Order) api.Sort {
	switch order {
	case system.OrderActive:
		return api.SortNewComments
	case system.OrderHot:
		return api.SortHot
	case system.OrderTopDay:
		return api.SortTopDay
	case system.OrderTopWeek:
		return api.SortTopWeek
	case system.OrderTopMonth:
		return api.SortTopMonth
	case system.OrderTopYear:
		return api.SortTopYear
	case system.OrderTopAll:
		return api.SortTopAll
	case system.OrderComments:
		return api.SortMostComments
	default:
		return api.SortNew
	}
}

func (sys *System) ListPosts(
	ctx context.Context,
	forumID string,
	order system.Order,
) ([]post.Post, error) {
	query := api.PostQuery{
		Listing:    sys.listingType(),
		Sort:       sortType(order),
		Limit:      pageSize,
		MarkAsRead: sys.markAsRead,
	}
	if forumID != "" {
		communityID, err := strconv.ParseInt(forumID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid community id %q: %w", forumID, err)
		}
		query.CommunityID = communityID
	}

	var posts []api.Post
	err := sys.listing(ctx, func(ctx context.Context) error {
		var err error
		posts, err = sys.client.Posts(ctx, query)
		return err
	})
	if err != nil {
		return nil, err
	}

	var models []post.Post
	for _, p := range posts {
		models = append(models, sys.toPost(p))
	}

	return models, nil
}

func (sys *System) toPost(p api.Post) post.Post {
	kind, body := post.LinkBody(p.URL, p.Body)

	return post.Post{
		ID:      strconv.FormatInt(p.ID, 10),
		Subject: p.Name,
		Body:    body,
		Kind:    kind,

		Pinned: p.FeaturedCommunity || p.FeaturedLocal,
		Closed: p.Locked || p.Deleted || p.Removed,

		CreatedAt: p.Published,

		Author: author.Author{
			ID:   strconv.FormatInt(p.CreatorID, 10),
			Name: p.CreatorName,
		},

		Forum: forum.Forum{
			ID:     strconv.FormatInt(p.CommunityID, 10),
			Name:   p.CommunityName,
			SysIDX: sys.idx,
		},

		ReplyCount: int(p.Comments),
		Score:      post.Score{Value: int(p.Score), Unit: post.ScorePoints},

		URL:  fmt.Sprintf("%s/post/%d", sys.settings.URL, p.ID),
		Link: p.URL,

		SysIDX: sys.idx,
	}
}

func (sys *System) LoadPost(ctx context.Context, p *post.Post) error {
	postID, err := strconv.ParseInt(p.ID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid post id %q: %w", p.ID, err)
	}

	if err := sys.sessionErr(); err != nil {
		return err
	}

	loaded, err := sys.client.Post(ctx, postID)
	if err != nil {
		return err
	}
	comments, err := sys.client.Comments(ctx, postID, commentDepth)
	if err != nil {
		return err
	}

	p.Refresh(sys.toPost(loaded))
	p.Replies = sys.buildTree(p.ID, comments)

	return nil
}

func (sys *System) buildTree(postID string, comments []api.Comment) []reply.Reply {
	replies := make([]reply.Reply, 0, len(comments))
	for _, c := range comments {
		parentID := ""
		if parts := strings.Split(c.Path, "."); len(parts) >= 3 {
			parentID = parts[len(parts)-2]
		}

		replies = append(replies, reply.Reply{
			ID:        strconv.FormatInt(c.ID, 10),
			PostID:    postID,
			ParentID:  parentID,
			Body:      c.Content,
			Deleted:   c.Deleted || c.Removed,
			CreatedAt: c.Published,
			Author: author.Author{
				ID:   strconv.FormatInt(c.CreatorID, 10),
				Name: c.CreatorName,
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

	communityID, err := strconv.ParseInt(p.Forum.ID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid community id %q: %w", p.Forum.ID, err)
	}

	create := api.NewPost{CommunityID: communityID, Name: p.Subject}
	if p.Kind == post.KindLink {
		create.URL = strings.TrimSpace(p.Body)
	} else {
		create.Body = p.Body
	}

	var id int64
	err = sys.write(ctx, func(ctx context.Context) error {
		var err error
		id, err = sys.client.CreatePost(ctx, create)
		return err
	})
	if err != nil {
		return err
	}

	p.ID = strconv.FormatInt(id, 10)
	return nil
}

func (sys *System) CreateReply(ctx context.Context, r *reply.Reply) error {
	if !sys.Capabilities().Has(system.CapCreateReply) {
		return system.NoCredentials(Kind, sys.settings.URL)
	}

	postID, err := strconv.ParseInt(r.PostID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid post id %q: %w", r.PostID, err)
	}

	create := api.NewComment{PostID: postID, Content: r.Body}
	if r.ParentID != "" {
		parentID, err := strconv.ParseInt(r.ParentID, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid comment id %q: %w", r.ParentID, err)
		}
		create.ParentID = parentID
	}

	var id int64
	err = sys.write(ctx, func(ctx context.Context) error {
		var err error
		id, err = sys.client.CreateComment(ctx, create)
		return err
	})
	if err != nil {
		return err
	}

	r.ID = strconv.FormatInt(id, 10)
	return nil
}
