package api

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/mrusme/neonmodem/internal/system/httpx"
)

type Version int

const (
	V3 Version = 3
	V4 Version = 4
)

type Listing string

const (
	ListingAll        Listing = "all"
	ListingLocal      Listing = "local"
	ListingSubscribed Listing = "subscribed"
)

type Sort int

const (
	SortNew Sort = iota
	SortNewComments
	SortHot
	SortTopDay
	SortTopWeek
	SortTopMonth
	SortTopYear
	SortTopAll
	SortMostComments
)

type Community struct {
	ID          int64
	Name        string
	Title       string
	Summary     string
	Description string
}

type Post struct {
	ID                int64
	Name              string
	URL               string
	Body              string
	CreatorID         int64
	CreatorName       string
	CommunityID       int64
	CommunityName     string
	FeaturedCommunity bool
	FeaturedLocal     bool
	Locked            bool
	Deleted           bool
	Removed           bool
	Published         time.Time
	Comments          int64
	Score             int64
}

type Comment struct {
	ID          int64
	Path        string
	Content     string
	Deleted     bool
	Removed     bool
	Published   time.Time
	CreatorID   int64
	CreatorName string
}

type PostQuery struct {
	Listing     Listing
	CommunityID int64
	Sort        Sort
	Limit       int
	MarkAsRead  bool
}

type NewPost struct {
	CommunityID int64
	Name        string
	Body        string
	URL         string
}

type NewComment struct {
	PostID   int64
	ParentID int64
	Content  string
}

type Login struct {
	Username string
	Password string
	Code     string
}

type Config struct {
	URL   string
	Token string
	Read  *http.Client
	Write *http.Client
}

type backend interface {
	Login(ctx context.Context, l Login) (string, error)
	ValidateAuth(ctx context.Context) error
	Communities(ctx context.Context, listing Listing, limit int, pages int) ([]Community, error)
	Posts(ctx context.Context, q PostQuery) ([]Post, error)
	Post(ctx context.Context, id int64) (Post, error)
	Comments(ctx context.Context, postID int64, maxDepth int) ([]Comment, error)
	CreatePost(ctx context.Context, p NewPost) (int64, error)
	CreateComment(ctx context.Context, c NewComment) (int64, error)
}

type Client struct {
	site    *httpx.Client
	v3      backend
	v4      backend
	detect  chan struct{}
	version atomic.Int32
}

func NewClient(cfg Config) (*Client, error) {
	site, err := httpx.NewClient(cfg.Read, cfg.URL)
	if err != nil {
		return nil, err
	}
	base := site.Base.String()

	v3, err := newV3Backend(base, cfg)
	if err != nil {
		return nil, err
	}
	v4, err := newV4Backend(base, cfg)
	if err != nil {
		return nil, err
	}

	return &Client{site: site, v3: v3, v4: v4, detect: make(chan struct{}, 1)}, nil
}

func (c *Client) Version(ctx context.Context) (Version, error) {
	if v := Version(c.version.Load()); v != 0 {
		return v, nil
	}

	select {
	case c.detect <- struct{}{}:
	case <-ctx.Done():
		return 0, ctx.Err()
	}
	defer func() { <-c.detect }()

	if v := Version(c.version.Load()); v != 0 {
		return v, nil
	}
	v, err := c.readVersion(ctx)
	if err != nil {
		return 0, err
	}
	c.version.Store(int32(v))
	return v, nil
}

func (c *Client) backend(ctx context.Context) (backend, error) {
	v, err := c.Version(ctx)
	if err != nil {
		return nil, err
	}
	if v == V3 {
		return c.v3, nil
	}
	return c.v4, nil
}

func (c *Client) Login(ctx context.Context, l Login) (string, error) {
	b, err := c.backend(ctx)
	if err != nil {
		return "", err
	}
	return b.Login(ctx, l)
}

func (c *Client) ValidateAuth(ctx context.Context) error {
	b, err := c.backend(ctx)
	if err != nil {
		return err
	}
	return b.ValidateAuth(ctx)
}

func (c *Client) Communities(ctx context.Context, listing Listing, limit int, pages int) ([]Community, error) {
	b, err := c.backend(ctx)
	if err != nil {
		return nil, err
	}
	return b.Communities(ctx, listing, limit, pages)
}

func (c *Client) Posts(ctx context.Context, q PostQuery) ([]Post, error) {
	b, err := c.backend(ctx)
	if err != nil {
		return nil, err
	}
	return b.Posts(ctx, q)
}

func (c *Client) Post(ctx context.Context, id int64) (Post, error) {
	b, err := c.backend(ctx)
	if err != nil {
		return Post{}, err
	}
	return b.Post(ctx, id)
}

func (c *Client) Comments(ctx context.Context, postID int64, maxDepth int) ([]Comment, error) {
	b, err := c.backend(ctx)
	if err != nil {
		return nil, err
	}
	return b.Comments(ctx, postID, maxDepth)
}

func (c *Client) CreatePost(ctx context.Context, p NewPost) (int64, error) {
	b, err := c.backend(ctx)
	if err != nil {
		return 0, err
	}
	return b.CreatePost(ctx, p)
}

func (c *Client) CreateComment(ctx context.Context, cm NewComment) (int64, error) {
	b, err := c.backend(ctx)
	if err != nil {
		return 0, err
	}
	return b.CreateComment(ctx, cm)
}
