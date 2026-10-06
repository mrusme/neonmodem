package discourse

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/mrusme/neonmodem/internal/models/author"
	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/discourse/api"
	"github.com/mrusme/neonmodem/internal/system/httpx"
	"github.com/mrusme/neonmodem/internal/system/text"
)

const (
	Kind = "discourse"

	categoryCacheTTL = 5 * time.Minute
	defaultChunkSize = 20
)

type System struct {
	idx      int
	settings system.Settings
	logger   *slog.Logger
	client   *api.Client

	catMu      sync.Mutex
	categories []api.CategoryModel
	catLoaded  time.Time

	ordersMu   sync.Mutex
	hotMissing bool
}

func New(env system.Env) (system.System, error) {
	sys := &System{
		idx:      env.Index,
		settings: env.Settings,
		logger:   env.Log(),
	}

	if env.Settings.URL != "" {
		if err := sys.connectClient(env.Proxy); err != nil {
			return nil, err
		}
	}

	return sys, nil
}

func (sys *System) connectClient(proxy string) error {
	httpClient := httpx.NewHTTPClient(httpx.Options{
		Proxy:   proxy,
		Retries: 3,
		Logger:  sys.logger,
	})

	client, err := api.NewClient(httpClient, sys.settings.URL, api.Credentials{
		ClientID: sys.settings.Credential(system.CredentialClientID),
		Key:      sys.settings.Credential(system.CredentialKey),
	})
	if err != nil {
		return err
	}
	sys.client = client

	return nil
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
	if sys.settings.Credential(system.CredentialKey) == "" {
		return "Discourse (read-only)"
	}
	return "Discourse"
}

func (sys *System) Capabilities() system.Capabilities {
	caps := system.CapRead
	if sys.settings.Credential(system.CredentialKey) != "" {
		caps |= system.CapWrite
	}
	return caps
}

func (sys *System) loadCategories(ctx context.Context) ([]api.CategoryModel, error) {
	sys.catMu.Lock()
	defer sys.catMu.Unlock()

	if sys.categories != nil && time.Since(sys.catLoaded) < categoryCacheTTL {
		return sys.categories, nil
	}

	cats, err := sys.client.Categories.List(ctx)
	if err != nil {
		return nil, err
	}

	sys.categories = cats.CategoryList.Categories
	sys.catLoaded = time.Now()

	return sys.categories, nil
}

type flatCategory struct {
	api.CategoryModel
	Path     string
	SlugPath string
}

func flatten(cats []api.CategoryModel, parent *flatCategory) []flatCategory {
	var out []flatCategory
	for i := range cats {
		fc := flatCategory{CategoryModel: cats[i], Path: cats[i].Name, SlugPath: cats[i].Slug}
		if parent != nil {
			fc.Path = parent.Path + " / " + cats[i].Name
			fc.SlugPath = parent.SlugPath + "/" + cats[i].Slug
		}
		out = append(out, fc)
		out = append(out, flatten(cats[i].SubcategoryList, &fc)...)
	}
	return out
}

func (sys *System) ListForums(ctx context.Context) ([]forum.Forum, error) {
	cats, err := sys.loadCategories(ctx)
	if err != nil {
		return nil, err
	}

	var models []forum.Forum
	for _, c := range flatten(cats, nil) {
		models = append(models, forum.Forum{
			ID:     strconv.Itoa(c.ID),
			Name:   c.Path,
			Info:   text.FirstNonEmpty(c.DescriptionText.String, c.Description.String),
			SysIDX: sys.idx,
		})
	}

	return models, nil
}

func (sys *System) Orders(string) system.Ordering {
	ordering := system.Ordering{Default: system.OrderNew, Supported: system.AllOrders()}

	sys.ordersMu.Lock()
	defer sys.ordersMu.Unlock()
	if sys.hotMissing {
		return ordering.Without(system.OrderHot)
	}
	return ordering
}

func topicList(order system.Order) (string, url.Values) {
	query := url.Values{}
	switch order {
	case system.OrderActive:
		return "latest", query
	case system.OrderHot:
		return "hot", query
	case system.OrderTopDay:
		query.Set("period", "daily")
		return "top", query
	case system.OrderTopWeek:
		query.Set("period", "weekly")
		return "top", query
	case system.OrderTopMonth:
		query.Set("period", "monthly")
		return "top", query
	case system.OrderTopYear:
		query.Set("period", "yearly")
		return "top", query
	case system.OrderTopAll:
		query.Set("period", "all")
		return "top", query
	case system.OrderComments:
		query.Set("order", "posts")
		return "latest", query
	}
	query.Set("order", "created")
	return "latest", query
}

func (sys *System) ListPosts(
	ctx context.Context,
	forumID string,
	order system.Order,
) ([]post.Post, error) {
	cats, err := sys.loadCategories(ctx)
	if err != nil {
		return nil, err
	}
	flat := flatten(cats, nil)

	byID := make(map[int]flatCategory, len(flat))
	for _, c := range flat {
		byID[c.ID] = c
	}

	catID := 0
	slugPath := ""
	if forumID != "" {
		catID, err = strconv.Atoi(forumID)
		if err != nil {
			return nil, fmt.Errorf("invalid category id %q: %w", forumID, err)
		}
		c, ok := byID[catID]
		if !ok {
			return nil, fmt.Errorf("unknown category id %d", catID)
		}
		slugPath = c.SlugPath
	}

	list, query := topicList(order)
	items, err := sys.client.Topics.List(ctx, list, slugPath, catID, query)
	if err != nil {
		if order == system.OrderHot && httpx.StatusOf(err) == http.StatusNotFound {
			sys.ordersMu.Lock()
			sys.hotMissing = true
			sys.ordersMu.Unlock()
			return nil, fmt.Errorf("%w: %w", system.ErrOrderUnavailable, err)
		}
		return nil, err
	}

	users := make(map[int]api.UserModel, len(items.Users))
	for _, u := range items.Users {
		users[u.ID] = u
	}

	var models []post.Post
	for _, t := range items.TopicList.Topics {
		var poster author.Author
		if len(t.Posters) > 0 {
			u := users[t.Posters[0].UserID]
			poster = author.Author{
				ID:   strconv.Itoa(t.Posters[0].UserID),
				Name: text.FirstNonEmpty(u.Name, u.Username),
			}
		}

		forumName := ""
		if c, ok := byID[t.CategoryID]; ok {
			forumName = c.Name
		}

		replies := t.PostsCount - 1
		if replies < 0 {
			replies = 0
		}

		models = append(models, post.Post{
			ID:      strconv.Itoa(t.ID),
			Subject: t.Title,
			Kind:    post.KindText,

			Pinned: t.Pinned,
			Closed: t.Closed || t.Archived,

			CreatedAt:       text.Time(t.CreatedAt),
			LastCommentedAt: text.Time(t.LastPostedAt),

			Author: poster,

			Forum: forum.Forum{
				ID:     strconv.Itoa(t.CategoryID),
				Name:   forumName,
				SysIDX: sys.idx,
			},

			ReplyCount: replies,
			Score:      post.Score{Value: t.LikeCount, Unit: post.ScoreLikes},

			URL: fmt.Sprintf("%s/t/%d", sys.settings.URL, t.ID),

			SysIDX: sys.idx,
		})
	}

	return models, nil
}

func (sys *System) LoadPost(ctx context.Context, p *post.Post) error {
	item, err := sys.client.Topics.Show(ctx, p.ID)
	if err != nil {
		return err
	}

	chunk := item.ChunkSize
	if chunk <= 0 {
		chunk = defaultChunkSize
	}

	stream := item.PostStream.Stream
	total := len(stream) - 1
	if total < 0 {
		total = 0
	}

	maxOffset := total - chunk
	if maxOffset < 0 {
		maxOffset = 0
	}

	offset := maxOffset
	if p.ReplyPage.Total > 0 {
		offset = p.ReplyPage.Offset
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		offset = 0
	}

	posts := item.PostStream.Posts
	if len(posts) < len(stream) || offset > 0 {
		end := 1 + offset + chunk
		if end > len(stream) {
			end = len(stream)
		}
		ids := append([]int{stream[0]}, stream[1+offset:end]...)

		window, err := sys.client.Topics.ShowPosts(ctx, p.ID, ids)
		if err != nil {
			return err
		}
		posts = window.PostStream.Posts
	}

	sort.Slice(posts, func(i, j int) bool {
		return posts[i].PostNumber < posts[j].PostNumber
	})

	p.Replies = nil
	p.Subject = text.FirstNonEmpty(item.Title, p.Subject)
	p.Pinned = item.Pinned
	p.Closed = item.Closed || item.Archived
	p.ReplyCount = total
	p.ReplyPage = post.ReplyPage{Offset: offset, Size: chunk, Total: total}

	type node struct {
		reply  reply.Reply
		number int
		parent int
	}

	var nodes []node
	numberToIdx := make(map[int]int, len(posts))
	for _, pm := range posts {
		if pm.PostNumber == 1 {
			p.Body = text.Markdown(pm.Cooked)
			p.Author = author.Author{
				ID:   strconv.Itoa(pm.UserID),
				Name: text.FirstNonEmpty(pm.Name, pm.Username),
			}
			continue
		}

		nodes = append(nodes, node{
			reply: reply.Reply{
				ID:        strconv.Itoa(pm.ID),
				PostID:    p.ID,
				Body:      text.Markdown(pm.Cooked),
				Deleted:   pm.UserDeleted || pm.DeletedAt != "",
				CreatedAt: text.Time(pm.CreatedAt),
				Author: author.Author{
					ID:   strconv.Itoa(pm.UserID),
					Name: text.FirstNonEmpty(pm.Name, pm.Username),
				},
				SysIDX: sys.idx,
			},
			number: pm.PostNumber,
			parent: pm.ReplyToPostNumber,
		})
		numberToIdx[pm.PostNumber] = len(nodes) - 1
	}

	children := make(map[int][]int)
	var roots []int
	for i, n := range nodes {
		if parentIdx, ok := numberToIdx[n.parent]; ok && n.parent > 1 {
			nodes[i].reply.ParentID = nodes[parentIdx].reply.ID
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

	p.Replies = build(roots)

	return nil
}

func (sys *System) CreatePost(ctx context.Context, p *post.Post) error {
	if !sys.Capabilities().Has(system.CapCreatePost) {
		return system.ErrNoCredentials
	}

	categoryID, err := strconv.Atoi(p.Forum.ID)
	if err != nil {
		return fmt.Errorf("invalid category id %q: %w", p.Forum.ID, err)
	}

	created, err := sys.client.Posts.Create(ctx, &api.CreatePostModel{
		Title:    p.Subject,
		Raw:      p.Body,
		Category: categoryID,
	})
	if err != nil {
		return err
	}

	p.ID = strconv.Itoa(created.TopicID)
	return nil
}

func (sys *System) CreateReply(ctx context.Context, r *reply.Reply) error {
	if !sys.Capabilities().Has(system.CapCreateReply) {
		return system.ErrNoCredentials
	}

	topicID, err := strconv.Atoi(r.PostID)
	if err != nil {
		return fmt.Errorf("invalid topic id %q: %w", r.PostID, err)
	}

	model := api.CreatePostModel{
		Raw:     r.Body,
		TopicID: topicID,
	}

	if r.ParentID != "" {
		parent, err := sys.client.Posts.Show(ctx, r.ParentID)
		if err != nil {
			return fmt.Errorf("looking up the reply target: %w", err)
		}
		model.ReplyToPostNumber = parent.PostNumber
	}

	created, err := sys.client.Posts.Create(ctx, &model)
	if err != nil {
		return err
	}

	r.ID = strconv.Itoa(created.ID)
	return nil
}
