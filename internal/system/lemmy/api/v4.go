package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/mrusme/neonmodem/internal/system/text"
	"github.com/ogen-go/ogen/ogenerrors"
	"github.com/ogen-go/ogen/validate"
	lemmyv4 "go.elara.ws/go-lemmy/v4"
)

const day = int(24 * time.Hour / time.Second)

type bearer string

func (b bearer) BearerAuth(context.Context, lemmyv4.OperationName) (lemmyv4.BearerAuth, error) {
	if b == "" {
		return lemmyv4.BearerAuth{}, ogenerrors.ErrSkipClientSecurity
	}
	return lemmyv4.BearerAuth{Token: string(b)}, nil
}

type v4Backend struct {
	read  *lemmyv4.Client
	write *lemmyv4.Client
}

func newV4Backend(baseURL string, cfg Config) (*v4Backend, error) {
	read, err := lemmyv4.NewClient(baseURL, bearer(cfg.Token), lemmyv4.WithClient(cfg.Read))
	if err != nil {
		return nil, err
	}
	write, err := lemmyv4.NewClient(baseURL, bearer(cfg.Token), lemmyv4.WithClient(cfg.Write))
	if err != nil {
		return nil, err
	}

	return &v4Backend{read: read, write: write}, nil
}

func v4Error(err error) error {
	use, ok := errors.AsType[*validate.UnexpectedStatusCodeError](err)
	if !ok {
		return err
	}

	e := &Error{Status: use.StatusCode}
	if use.Payload == nil || use.Payload.Body == nil {
		return e
	}
	body, err := io.ReadAll(use.Payload.Body)
	if err != nil {
		return e
	}
	var reply struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &reply) == nil {
		e.Code, e.Message = reply.Error, reply.Message
	}
	return e
}

func v4Listing(listing Listing) lemmyv4.ListingType {
	switch listing {
	case ListingAll:
		return lemmyv4.ListingTypeAll
	case ListingLocal:
		return lemmyv4.ListingTypeLocal
	default:
		return lemmyv4.ListingTypeSubscribed
	}
}

func v4Sort(sort Sort) (lemmyv4.PostSortType, int) {
	switch sort {
	case SortNewComments:
		return lemmyv4.PostSortTypeNewComments, 0
	case SortHot:
		return lemmyv4.PostSortTypeHot, 0
	case SortTopDay:
		return lemmyv4.PostSortTypeTop, day
	case SortTopWeek:
		return lemmyv4.PostSortTypeTop, 7 * day
	case SortTopMonth:
		return lemmyv4.PostSortTypeTop, 30 * day
	case SortTopYear:
		return lemmyv4.PostSortTypeTop, 365 * day
	case SortTopAll:
		return lemmyv4.PostSortTypeTop, 0
	case SortMostComments:
		return lemmyv4.PostSortTypeMostComments, 0
	default:
		return lemmyv4.PostSortTypeNew, 0
	}
}

func v4Post(pv lemmyv4.PostView) Post {
	return Post{
		ID:                int64(pv.Post.ID),
		Name:              pv.Post.Name,
		URL:               string(pv.Post.URL.Or("")),
		Body:              pv.Post.Body.Or(""),
		CreatorID:         int64(pv.Post.CreatorID),
		CreatorName:       pv.Creator.Name,
		CommunityID:       int64(pv.Post.CommunityID),
		CommunityName:     pv.Community.Name,
		FeaturedCommunity: pv.Post.FeaturedCommunity,
		FeaturedLocal:     pv.Post.FeaturedLocal,
		Locked:            pv.Post.Locked,
		Deleted:           pv.Post.Deleted,
		Removed:           pv.Post.Removed,
		Published:         text.Time(pv.Post.PublishedAt),
		Comments:          int64(pv.Post.Comments),
		Score:             int64(pv.Post.Score),
	}
}

func (b *v4Backend) Login(ctx context.Context, l Login) (string, error) {
	login := &lemmyv4.Login{
		UsernameOrEmail: lemmyv4.SensitiveString(l.Username),
		Password:        lemmyv4.SensitiveString(l.Password),
		StayLoggedIn:    lemmyv4.NewOptBool(true),
	}
	if l.Code != "" {
		login.Totp2FAToken = lemmyv4.NewOptString(l.Code)
	}

	resp, err := b.write.Login(ctx, login)
	if err != nil {
		return "", v4Error(err)
	}
	jwt := string(resp.Jwt.Or(""))
	if jwt == "" {
		return "", ErrNoToken
	}
	return jwt, nil
}

func (b *v4Backend) ValidateAuth(ctx context.Context) error {
	_, err := b.read.ValidateAuth(ctx)
	return v4Error(err)
}

func (b *v4Backend) Communities(ctx context.Context, listing Listing, limit int, pages int) ([]Community, error) {
	params := lemmyv4.ListCommunitiesParams{
		Type:  lemmyv4.NewOptListingType(v4Listing(listing)),
		Sort:  lemmyv4.NewOptCommunitySortType(lemmyv4.CommunitySortTypeSubscribers),
		Limit: lemmyv4.NewOptInt(limit),
	}

	var communities []Community
	for range pages {
		resp, err := b.read.ListCommunities(ctx, params)
		if err != nil {
			return nil, v4Error(err)
		}

		for _, cv := range resp.Items {
			communities = append(communities, Community{
				ID:          int64(cv.Community.ID),
				Name:        cv.Community.Name,
				Title:       cv.Community.Title.Or(""),
				Summary:     cv.Community.Summary.Or(""),
				Description: cv.Community.Sidebar.Or(""),
			})
		}
		next, ok := resp.NextPage.Get()
		if !ok {
			break
		}
		params.PageCursor = lemmyv4.NewOptPaginationCursor(next)
	}
	return communities, nil
}

func (b *v4Backend) Posts(ctx context.Context, q PostQuery) ([]Post, error) {
	sort, timeRange := v4Sort(q.Sort)
	params := lemmyv4.GetPostsParams{
		Type:             lemmyv4.NewOptListingType(v4Listing(q.Listing)),
		Sort:             lemmyv4.NewOptPostSortType(sort),
		TimeRangeSeconds: lemmyv4.NewOptInt(timeRange),
		Limit:            lemmyv4.NewOptInt(q.Limit),
		MarkAsRead:       lemmyv4.NewOptBool(q.MarkAsRead),
	}
	if q.CommunityID != 0 {
		params.CommunityID = lemmyv4.NewOptCommunityId(lemmyv4.CommunityId(q.CommunityID))
	}

	resp, err := b.read.GetPosts(ctx, params)
	if err != nil {
		return nil, v4Error(err)
	}
	posts := make([]Post, 0, len(resp.Items))
	for _, pv := range resp.Items {
		posts = append(posts, v4Post(pv))
	}
	return posts, nil
}

func (b *v4Backend) Post(ctx context.Context, id int64) (Post, error) {
	resp, err := b.read.GetPost(ctx, lemmyv4.GetPostParams{
		ID: lemmyv4.NewOptPostId(lemmyv4.PostId(id)),
	})
	if err != nil {
		return Post{}, v4Error(err)
	}
	return v4Post(resp.PostView), nil
}

func (b *v4Backend) Comments(ctx context.Context, postID int64, maxDepth int) ([]Comment, error) {
	resp, err := b.read.GetComments(ctx, lemmyv4.GetCommentsParams{
		PostID:           lemmyv4.NewOptPostId(lemmyv4.PostId(postID)),
		MaxDepth:         lemmyv4.NewOptInt(maxDepth),
		Sort:             lemmyv4.NewOptCommentSortType(lemmyv4.CommentSortTypeOld),
		Type:             lemmyv4.NewOptListingType(lemmyv4.ListingTypeAll),
		TimeRangeSeconds: lemmyv4.NewOptInt(0),
	})
	if err != nil {
		return nil, v4Error(err)
	}

	comments := make([]Comment, 0, len(resp.Items))
	for _, cv := range resp.Items {
		comments = append(comments, Comment{
			ID:          int64(cv.Comment.ID),
			Path:        cv.Comment.Path,
			Content:     cv.Comment.Content,
			Deleted:     cv.Comment.Deleted,
			Removed:     cv.Comment.Removed,
			Published:   text.Time(cv.Comment.PublishedAt),
			CreatorID:   int64(cv.Comment.CreatorID),
			CreatorName: cv.Creator.Name,
		})
	}
	return comments, nil
}

func (b *v4Backend) CreatePost(ctx context.Context, p NewPost) (int64, error) {
	create := &lemmyv4.CreatePost{
		Name:        p.Name,
		CommunityID: lemmyv4.CommunityId(p.CommunityID),
		Nsfw:        lemmyv4.NewOptBool(false),
	}
	if p.URL != "" {
		create.URL = lemmyv4.NewOptString(p.URL)
	} else {
		create.Body = lemmyv4.NewOptString(p.Body)
	}

	resp, err := b.write.CreatePost(ctx, create)
	if err != nil {
		return 0, v4Error(err)
	}
	return int64(resp.PostView.Post.ID), nil
}

func (b *v4Backend) CreateComment(ctx context.Context, c NewComment) (int64, error) {
	create := &lemmyv4.CreateComment{
		PostID:  lemmyv4.PostId(c.PostID),
		Content: c.Content,
	}
	if c.ParentID != 0 {
		create.ParentID = lemmyv4.NewOptCommentId(lemmyv4.CommentId(c.ParentID))
	}

	resp, err := b.write.CreateComment(ctx, create)
	if err != nil {
		return 0, v4Error(err)
	}
	return int64(resp.CommentView.Comment.ID), nil
}
