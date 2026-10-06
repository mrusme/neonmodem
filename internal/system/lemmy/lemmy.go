package lemmy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"sync"

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
	idx      int
	settings system.Settings
	proxy    string
	logger   *slog.Logger
	client   *lemmy.Client

	loginMu  sync.Mutex
	loggedIn bool
}

func New(env system.Env) (system.System, error) {
	sys := &System{
		idx:      env.Index,
		settings: env.Settings,
		proxy:    env.Proxy,
		logger:   env.Log(),
	}

	if env.Settings.URL != "" {
		client, err := newClient(env.Settings.URL, env.Proxy, sys.logger)
		if err != nil {
			return nil, err
		}
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

func (sys *System) Title() string {
	u, err := url.Parse(sys.settings.URL)
	if err != nil || u.Hostname() == "" {
		return sys.settings.URL
	}
	return u.Hostname()
}

func (sys *System) Description() string {
	if !sys.hasAccount() {
		return "Lemmy (read-only)"
	}
	return "Lemmy"
}

func (sys *System) hasAccount() bool {
	return sys.settings.Credential(system.CredentialUsername) != "" &&
		sys.settings.Credential(system.CredentialPassword) != ""
}

func (sys *System) Capabilities() system.Capabilities {
	caps := system.CapRead
	if sys.hasAccount() {
		caps |= system.CapWrite
	}
	return caps
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

func (sys *System) ensureLogin(ctx context.Context) error {
	if !sys.hasAccount() {
		return nil
	}

	sys.loginMu.Lock()
	defer sys.loginMu.Unlock()

	if sys.loggedIn {
		return nil
	}

	err := sys.client.ClientLogin(ctx, lemmy.Login{
		UsernameOrEmail: sys.settings.Credential(system.CredentialUsername),
		Password:        sys.settings.Credential(system.CredentialPassword),
	})
	if err != nil {
		return fmt.Errorf("logging in to %s: %w", sys.Title(), err)
	}
	sys.loggedIn = true

	return nil
}

func isAuthError(err error) bool {
	var le lemmy.Error
	if !errors.As(err, &le) {
		return false
	}
	return le.Code == 401 ||
		le.ErrStr == "not_logged_in" ||
		le.ErrStr == "incorrect_login"
}

func (sys *System) withLogin(ctx context.Context, call func() error) error {
	if err := sys.ensureLogin(ctx); err != nil {
		return err
	}

	err := call()
	if err == nil || !isAuthError(err) || !sys.hasAccount() {
		return err
	}

	sys.loginMu.Lock()
	sys.loggedIn = false
	sys.loginMu.Unlock()

	if err := sys.ensureLogin(ctx); err != nil {
		return err
	}
	return call()
}

func (sys *System) ListForums(ctx context.Context) ([]forum.Forum, error) {
	var models []forum.Forum

	err := sys.withLogin(ctx, func() error {
		models = nil
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

func (sys *System) ListPosts(ctx context.Context, forumID string) ([]post.Post, error) {
	params := lemmy.GetPosts{
		Type:  lemmy.NewOptional(sys.listingType()),
		Sort:  lemmy.NewOptional(lemmy.SortTypeNew),
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
	err := sys.withLogin(ctx, func() error {
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
	kind := post.KindText
	body := pv.Post.Body.ValueOr("")
	if u, ok := pv.Post.URL.Value(); ok && u != "" {
		kind = post.KindLink
		if body == "" {
			body = u
		} else {
			body = u + "\n\n" + body
		}
	}

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

		URL: fmt.Sprintf("%s/post/%d", sys.settings.URL, pv.Post.ID),

		SysIDX: sys.idx,
	}
}

func (sys *System) LoadPost(ctx context.Context, p *post.Post) error {
	postID, err := strconv.ParseInt(p.ID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid post id %q: %w", p.ID, err)
	}

	var postResp *lemmy.GetPostResponse
	var comments *lemmy.GetCommentsResponse
	err = sys.withLogin(ctx, func() error {
		var err error
		postResp, err = sys.client.Post(ctx, lemmy.GetPost{ID: lemmy.NewOptional(postID)})
		if err != nil {
			return err
		}
		comments, err = sys.client.Comments(ctx, lemmy.GetComments{
			PostID:   lemmy.NewOptional(postID),
			MaxDepth: lemmy.NewOptional(int64(commentDepth)),
			Sort:     lemmy.NewOptional(lemmy.CommentSortTypeOld),
			Type:     lemmy.NewOptional(lemmy.ListingTypeAll),
		})
		return err
	})
	if err != nil {
		return err
	}

	fresh := sys.toPost(postResp.PostView)
	p.Subject = fresh.Subject
	p.Body = fresh.Body
	p.Kind = fresh.Kind
	p.Pinned = fresh.Pinned
	p.Closed = fresh.Closed
	p.Author = fresh.Author
	p.ReplyCount = fresh.ReplyCount

	p.Replies = sys.buildTree(p.ID, comments.Comments)

	return nil
}

func (sys *System) buildTree(postID string, views []lemmy.CommentView) []reply.Reply {
	type node struct {
		reply    reply.Reply
		parentID string
	}

	nodes := make([]node, 0, len(views))
	index := make(map[string]int, len(views))

	for _, cv := range views {
		id := strconv.FormatInt(cv.Comment.ID, 10)
		parentID := ""
		if parts := strings.Split(cv.Comment.Path, "."); len(parts) >= 3 {
			parentID = parts[len(parts)-2]
		}

		nodes = append(nodes, node{
			reply: reply.Reply{
				ID:        id,
				PostID:    postID,
				Body:      cv.Comment.Content,
				Deleted:   cv.Comment.Deleted || cv.Comment.Removed,
				CreatedAt: cv.Comment.Published,
				Author: author.Author{
					ID:   strconv.FormatInt(cv.Comment.CreatorID, 10),
					Name: cv.Creator.Name,
				},
				SysIDX: sys.idx,
			},
			parentID: parentID,
		})
		index[id] = len(nodes) - 1
	}

	children := make(map[int][]int)
	var roots []int
	for i, n := range nodes {
		if parentIdx, ok := index[n.parentID]; ok && n.parentID != "" {
			nodes[i].reply.ParentID = n.parentID
			children[parentIdx] = append(children[parentIdx], i)
		} else {
			roots = append(roots, i)
		}
	}

	var build func(indexes []int) []reply.Reply
	build = func(indexes []int) []reply.Reply {
		out := make([]reply.Reply, 0, len(indexes))
		for _, i := range indexes {
			r := nodes[i].reply
			r.Replies = build(children[i])
			out = append(out, r)
		}
		return out
	}

	return build(roots)
}

func (sys *System) CreatePost(ctx context.Context, p *post.Post) error {
	if !sys.hasAccount() {
		return system.ErrNoCredentials
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
	err = sys.withLogin(ctx, func() error {
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
	if !sys.hasAccount() {
		return system.ErrNoCredentials
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
	err = sys.withLogin(ctx, func() error {
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
