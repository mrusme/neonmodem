package api

import (
	"context"
	"errors"

	lemmyv3 "go.elara.ws/go-lemmy/v3"
)

type v3Backend struct {
	read  *lemmyv3.Client
	write *lemmyv3.Client
}

func newV3Backend(baseURL string, cfg Config) (*v3Backend, error) {
	read, err := lemmyv3.NewWithClient(baseURL, cfg.Read)
	if err != nil {
		return nil, err
	}
	write, err := lemmyv3.NewWithClient(baseURL, cfg.Write)
	if err != nil {
		return nil, err
	}
	read.Token = cfg.Token
	write.Token = cfg.Token

	return &v3Backend{read: read, write: write}, nil
}

func v3Error(err error) error {
	if le, ok := errors.AsType[lemmyv3.Error](err); ok {
		return &Error{Status: le.Code, Code: le.ErrStr}
	}
	return err
}

func v3Listing(listing Listing) lemmyv3.ListingType {
	switch listing {
	case ListingAll:
		return lemmyv3.ListingTypeAll
	case ListingLocal:
		return lemmyv3.ListingTypeLocal
	default:
		return lemmyv3.ListingTypeSubscribed
	}
}

func v3Sort(sort Sort) lemmyv3.SortType {
	switch sort {
	case SortNewComments:
		return lemmyv3.SortTypeNewComments
	case SortHot:
		return lemmyv3.SortTypeHot
	case SortTopDay:
		return lemmyv3.SortTypeTopDay
	case SortTopWeek:
		return lemmyv3.SortTypeTopWeek
	case SortTopMonth:
		return lemmyv3.SortTypeTopMonth
	case SortTopYear:
		return lemmyv3.SortTypeTopYear
	case SortTopAll:
		return lemmyv3.SortTypeTopAll
	case SortMostComments:
		return lemmyv3.SortTypeMostComments
	default:
		return lemmyv3.SortTypeNew
	}
}

func v3Post(pv lemmyv3.PostView) Post {
	return Post{
		ID:                pv.Post.ID,
		Name:              pv.Post.Name,
		URL:               pv.Post.URL.ValueOrZero(),
		Body:              pv.Post.Body.ValueOrZero(),
		CreatorID:         pv.Post.CreatorID,
		CreatorName:       pv.Creator.Name,
		CommunityID:       pv.Post.CommunityID,
		CommunityName:     pv.Community.Name,
		FeaturedCommunity: pv.Post.FeaturedCommunity,
		FeaturedLocal:     pv.Post.FeaturedLocal,
		Locked:            pv.Post.Locked,
		Deleted:           pv.Post.Deleted,
		Removed:           pv.Post.Removed,
		Published:         pv.Post.Published,
		Comments:          pv.Counts.Comments,
		Score:             pv.Counts.Score,
	}
}

func (b *v3Backend) Login(ctx context.Context, l Login) (string, error) {
	login := lemmyv3.Login{UsernameOrEmail: l.Username, Password: l.Password}
	if l.Code != "" {
		login.TOTP2FAToken = lemmyv3.NewOptional(l.Code)
	}

	resp, err := b.write.Login(ctx, login)
	if err != nil {
		return "", v3Error(err)
	}
	jwt, ok := resp.JWT.Value()
	if !ok || jwt == "" {
		return "", ErrNoToken
	}
	return jwt, nil
}

func (b *v3Backend) ValidateAuth(ctx context.Context) error {
	_, err := b.read.ValidateAuth(ctx)
	return v3Error(err)
}

func (b *v3Backend) Communities(ctx context.Context, listing Listing, limit int, pages int) ([]Community, error) {
	var communities []Community
	for page := 1; page <= pages; page++ {
		resp, err := b.read.Communities(ctx, lemmyv3.ListCommunities{
			Type:  lemmyv3.NewOptional(v3Listing(listing)),
			Sort:  lemmyv3.NewOptional(lemmyv3.SortTypeTopAll),
			Limit: lemmyv3.NewOptional(int64(limit)),
			Page:  lemmyv3.NewOptional(int64(page)),
		})
		if err != nil {
			return nil, v3Error(err)
		}

		for _, cv := range resp.Communities {
			communities = append(communities, Community{
				ID:          cv.Community.ID,
				Name:        cv.Community.Name,
				Title:       cv.Community.Title,
				Description: cv.Community.Description.ValueOrZero(),
			})
		}
		if len(resp.Communities) < limit {
			break
		}
	}
	return communities, nil
}

func (b *v3Backend) Posts(ctx context.Context, q PostQuery) ([]Post, error) {
	params := lemmyv3.GetPosts{
		Type:  lemmyv3.NewOptional(v3Listing(q.Listing)),
		Sort:  lemmyv3.NewOptional(v3Sort(q.Sort)),
		Limit: lemmyv3.NewOptional(int64(q.Limit)),
	}
	if q.CommunityID != 0 {
		params.CommunityID = lemmyv3.NewOptional(q.CommunityID)
	}

	resp, err := b.read.Posts(ctx, params)
	if err != nil {
		return nil, v3Error(err)
	}
	posts := make([]Post, 0, len(resp.Posts))
	for _, pv := range resp.Posts {
		posts = append(posts, v3Post(pv))
	}
	return posts, nil
}

func (b *v3Backend) Post(ctx context.Context, id int64) (Post, error) {
	resp, err := b.read.Post(ctx, lemmyv3.GetPost{ID: lemmyv3.NewOptional(id)})
	if err != nil {
		return Post{}, v3Error(err)
	}
	return v3Post(resp.PostView), nil
}

func (b *v3Backend) Comments(ctx context.Context, postID int64, maxDepth int) ([]Comment, error) {
	resp, err := b.read.Comments(ctx, lemmyv3.GetComments{
		PostID:   lemmyv3.NewOptional(postID),
		MaxDepth: lemmyv3.NewOptional(int64(maxDepth)),
		Sort:     lemmyv3.NewOptional(lemmyv3.CommentSortTypeOld),
		Type:     lemmyv3.NewOptional(lemmyv3.ListingTypeAll),
	})
	if err != nil {
		return nil, v3Error(err)
	}

	comments := make([]Comment, 0, len(resp.Comments))
	for _, cv := range resp.Comments {
		comments = append(comments, Comment{
			ID:          cv.Comment.ID,
			Path:        cv.Comment.Path,
			Content:     cv.Comment.Content,
			Deleted:     cv.Comment.Deleted,
			Removed:     cv.Comment.Removed,
			Published:   cv.Comment.Published,
			CreatorID:   cv.Comment.CreatorID,
			CreatorName: cv.Creator.Name,
		})
	}
	return comments, nil
}

func (b *v3Backend) CreatePost(ctx context.Context, p NewPost) (int64, error) {
	create := lemmyv3.CreatePost{
		Name:        p.Name,
		CommunityID: p.CommunityID,
		NSFW:        lemmyv3.NewOptional(false),
	}
	if p.URL != "" {
		create.URL = lemmyv3.NewOptional(p.URL)
	} else {
		create.Body = lemmyv3.NewOptional(p.Body)
	}

	resp, err := b.write.CreatePost(ctx, create)
	if err != nil {
		return 0, v3Error(err)
	}
	return resp.PostView.Post.ID, nil
}

func (b *v3Backend) CreateComment(ctx context.Context, c NewComment) (int64, error) {
	create := lemmyv3.CreateComment{PostID: c.PostID, Content: c.Content}
	if c.ParentID != 0 {
		create.ParentID = lemmyv3.NewOptional(c.ParentID)
	}

	resp, err := b.write.CreateComment(ctx, create)
	if err != nil {
		return 0, v3Error(err)
	}
	return resp.CommentView.Comment.ID, nil
}
