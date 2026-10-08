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
	"go.elara.ws/go-lemmy"
)

const (
	Kind = "lemmy"

	OptionListing = "listing"

	ListingSubscribed = "subscribed"
	ListingLocal      = "local"
	ListingAll        = "all"

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
	client      *lemmy.Client

	sessionMu sync.Mutex
	ended     error
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
		client, err := newClient(env.Settings.URL, env.Proxy, sys.logger)
		if err != nil {
			return nil, err
		}
		client.Token = sys.token()
		sys.client = client
	}

	return sys, nil
}

func newClient(sysURL string, proxy string, logger *slog.Logger) (*lemmy.Client, error) {
	return lemmy.NewWithClient(sysURL, httpx.NewHTTPClient(httpx.Options{
		Proxy:   proxy,
		Retries: 3,
		Logger:  logger,
	}))
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

func (sys *System) listingType() lemmy.ListingType {
	listing := sys.settings.Option(OptionListing, ListingSubscribed)
	switch strings.ToLower(listing) {
	case ListingAll:
		return lemmy.ListingTypeAll
	case ListingLocal:
		return lemmy.ListingTypeLocal
	default:
		if !sys.hasAccount() {
			return lemmy.ListingTypeLocal
		}
		return lemmy.ListingTypeSubscribed
	}
}

func errorString(err error) string {
	if le, ok := errors.AsType[lemmy.Error](err); ok {
		return le.ErrStr
	}
	return ""
}

func isAuthError(err error) bool {
	le, ok := errors.AsType[lemmy.Error](err)
	if !ok {
		return false
	}
	return le.Code == 401 ||
		le.ErrStr == "not_logged_in" ||
		le.ErrStr == "incorrect_login"
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
	if sys.token() == "" {
		return call(ctx)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	checked := make(chan error, 1)
	go func() {
		_, err := sys.client.ValidateAuth(ctx)
		if err != nil {
			cancel()
		}
		checked <- err
	}()

	err := call(ctx)
	if checkErr := <-checked; checkErr != nil {
		if isAuthError(checkErr) {
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
	if err != nil && sys.token() != "" && isAuthError(err) {
		return sys.end()
	}
	return err
}

func (sys *System) ListForums(ctx context.Context) ([]forum.Forum, error) {
	var models []forum.Forum

	err := sys.listing(ctx, func(ctx context.Context) error {
		for page := int64(1); page <= maxPages; page++ {
			resp, err := sys.client.Communities(ctx, lemmy.ListCommunities{
				Type:  lemmy.NewOptional(sys.listingType()),
				Sort:  lemmy.NewOptional(lemmy.SortTypeTopAll),
				Limit: lemmy.NewOptional(int64(pageSize)),
				Page:  lemmy.NewOptional(page),
			})
			if err != nil {
				return err
			}

			for _, c := range resp.Communities {
				models = append(models, forum.Forum{
					ID:     strconv.FormatInt(c.Community.ID, 10),
					Name:   c.Community.Name,
					Info:   c.Community.Description.ValueOr(c.Community.Title),
					SysIDX: sys.idx,
				})
			}

			if len(resp.Communities) < pageSize {
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return models, nil
}

func (sys *System) Orders(string) system.Ordering {
	return system.Ordering{Default: system.OrderNew, Supported: system.AllOrders()}
}

func sortType(order system.Order) lemmy.SortType {
	switch order {
	case system.OrderActive:
		return lemmy.SortTypeNewComments
	case system.OrderHot:
		return lemmy.SortTypeHot
	case system.OrderTopDay:
		return lemmy.SortTypeTopDay
	case system.OrderTopWeek:
		return lemmy.SortTypeTopWeek
	case system.OrderTopMonth:
		return lemmy.SortTypeTopMonth
	case system.OrderTopYear:
		return lemmy.SortTypeTopYear
	case system.OrderTopAll:
		return lemmy.SortTypeTopAll
	case system.OrderComments:
		return lemmy.SortTypeMostComments
	default:
		return lemmy.SortTypeNew
	}
}

func (sys *System) ListPosts(
	ctx context.Context,
	forumID string,
	order system.Order,
) ([]post.Post, error) {
	params := lemmy.GetPosts{
		Type:  lemmy.NewOptional(sys.listingType()),
		Sort:  lemmy.NewOptional(sortType(order)),
		Limit: lemmy.NewOptional(int64(pageSize)),
	}
	if forumID != "" {
		communityID, err := strconv.ParseInt(forumID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid community id %q: %w", forumID, err)
		}
		params.CommunityID = lemmy.NewOptional(communityID)
	}

	var resp *lemmy.GetPostsResponse
	err := sys.listing(ctx, func(ctx context.Context) error {
		var err error
		resp, err = sys.client.Posts(ctx, params)
		return err
	})
	if err != nil {
		return nil, err
	}

	var models []post.Post
	for _, pv := range resp.Posts {
		models = append(models, sys.toPost(pv))
	}

	return models, nil
}

func (sys *System) toPost(pv lemmy.PostView) post.Post {
	kind, body := post.LinkBody(pv.Post.URL.ValueOr(""), pv.Post.Body.ValueOr(""))

	return post.Post{
		ID:      strconv.FormatInt(pv.Post.ID, 10),
		Subject: pv.Post.Name,
		Body:    body,
		Kind:    kind,

		Pinned: pv.Post.FeaturedCommunity || pv.Post.FeaturedLocal,
		Closed: pv.Post.Locked || pv.Post.Deleted || pv.Post.Removed,

		CreatedAt: pv.Post.Published,

		Author: author.Author{
			ID:   strconv.FormatInt(pv.Post.CreatorID, 10),
			Name: pv.Creator.Name,
		},

		Forum: forum.Forum{
			ID:     strconv.FormatInt(pv.Post.CommunityID, 10),
			Name:   pv.Community.Name,
			SysIDX: sys.idx,
		},

		ReplyCount: int(pv.Counts.Comments),
		Score:      post.Score{Value: int(pv.Counts.Score), Unit: post.ScorePoints},

		URL:  fmt.Sprintf("%s/post/%d", sys.settings.URL, pv.Post.ID),
		Link: pv.Post.URL.ValueOr(""),

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

	postResp, err := sys.client.Post(ctx, lemmy.GetPost{ID: lemmy.NewOptional(postID)})
	if err != nil {
		return err
	}
	comments, err := sys.client.Comments(ctx, lemmy.GetComments{
		PostID:   lemmy.NewOptional(postID),
		MaxDepth: lemmy.NewOptional(int64(commentDepth)),
		Sort:     lemmy.NewOptional(lemmy.CommentSortTypeOld),
		Type:     lemmy.NewOptional(lemmy.ListingTypeAll),
	})
	if err != nil {
		return err
	}

	p.Refresh(sys.toPost(postResp.PostView))
	p.Replies = sys.buildTree(p.ID, comments.Comments)

	return nil
}

func (sys *System) buildTree(postID string, views []lemmy.CommentView) []reply.Reply {
	replies := make([]reply.Reply, 0, len(views))
	for _, cv := range views {
		parentID := ""
		if parts := strings.Split(cv.Comment.Path, "."); len(parts) >= 3 {
			parentID = parts[len(parts)-2]
		}

		replies = append(replies, reply.Reply{
			ID:        strconv.FormatInt(cv.Comment.ID, 10),
			PostID:    postID,
			ParentID:  parentID,
			Body:      cv.Comment.Content,
			Deleted:   cv.Comment.Deleted || cv.Comment.Removed,
			CreatedAt: cv.Comment.Published,
			Author: author.Author{
				ID:   strconv.FormatInt(cv.Comment.CreatorID, 10),
				Name: cv.Creator.Name,
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

	create := lemmy.CreatePost{
		Name:        p.Subject,
		CommunityID: communityID,
		NSFW:        lemmy.NewOptional(false),
	}
	if p.Kind == post.KindLink {
		create.URL = lemmy.NewOptional(strings.TrimSpace(p.Body))
	} else {
		create.Body = lemmy.NewOptional(p.Body)
	}

	var resp *lemmy.PostResponse
	err = sys.write(ctx, func(ctx context.Context) error {
		var err error
		resp, err = sys.client.CreatePost(ctx, create)
		return err
	})
	if err != nil {
		return err
	}

	p.ID = strconv.FormatInt(resp.PostView.Post.ID, 10)
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

	create := lemmy.CreateComment{
		PostID:  postID,
		Content: r.Body,
	}
	if r.ParentID != "" {
		parentID, err := strconv.ParseInt(r.ParentID, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid comment id %q: %w", r.ParentID, err)
		}
		create.ParentID = lemmy.NewOptional(parentID)
	}

	var resp *lemmy.CommentResponse
	err = sys.write(ctx, func(ctx context.Context) error {
		var err error
		resp, err = sys.client.CreateComment(ctx, create)
		return err
	})
	if err != nil {
		return err
	}

	r.ID = strconv.FormatInt(resp.CommentView.Comment.ID, 10)
	return nil
}
