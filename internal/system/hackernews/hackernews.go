package hackernews

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mrusme/neonmodem/internal/models/author"
	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/hackernews/api"
	"github.com/mrusme/neonmodem/internal/system/httpx"
	"github.com/mrusme/neonmodem/internal/system/text"
)

const (
	Kind = "hackernews"

	listLimit = 30

	recentWriteWindow = 10 * time.Minute
)

type hnForum struct {
	ID      string
	Name    string
	Info    string
	HotList string
	NewList string
	Tag     string
	Votes   bool
}

var (
	feedForum = hnForum{HotList: "top", NewList: "new", Tag: "story", Votes: true}

	forums = []hnForum{
		{ID: "ask", Name: "Ask HN", Info: "Ask Hacker News about the world",
			HotList: "ask", Tag: "ask_hn", Votes: true},
		{ID: "jobs", Name: "Jobs HN", Info: "... because we can't *all* become Astronauts",
			HotList: "jobs", Tag: "job"},
		{ID: "show", Name: "Show HN", Info: "Show Hacker News something awesome",
			HotList: "show", Tag: "show_hn", Votes: true},
	}
)

func forumByID(id string) (hnForum, bool) {
	if id == "" {
		return feedForum, true
	}
	for _, f := range forums {
		if f.ID == id {
			return f, true
		}
	}
	return hnForum{}, false
}

func (f hnForum) ordering() system.Ordering {
	supported := []system.Order{system.OrderNew, system.OrderHot}
	if f.Votes {
		supported = append(supported, system.TopOrders()...)
	}
	return system.Ordering{Default: system.OrderNew, Supported: supported}
}

func topSince(order system.Order, now time.Time) time.Time {
	switch order {
	case system.OrderTopDay:
		return now.Add(-24 * time.Hour)
	case system.OrderTopWeek:
		return now.Add(-7 * 24 * time.Hour)
	case system.OrderTopMonth:
		return now.Add(-30 * 24 * time.Hour)
	case system.OrderTopYear:
		return now.Add(-365 * 24 * time.Hour)
	default:
		return time.Time{}
	}
}

type recentWrite struct {
	at      time.Time
	replyID string
}

type System struct {
	idx         int
	settings    system.Settings
	proxy       string
	logger      *slog.Logger
	readTimeout time.Duration
	client      *api.Client
	web         *webSession

	writesMu sync.Mutex
	writes   map[string][]recentWrite
}

func New(env system.Env) (system.System, error) {
	sys := &System{
		idx:         env.Index,
		settings:    env.Settings,
		proxy:       env.Proxy,
		logger:      env.Log(),
		readTimeout: env.ReadTimeout,
	}
	if sys.settings.URL == "" {
		sys.settings.URL = api.SiteURL
	}

	httpClient := httpx.NewHTTPClient(httpx.Options{
		Proxy:   env.Proxy,
		Retries: 3,
		Logger:  sys.logger,
	})
	client, err := api.NewClient(httpClient)
	if err != nil {
		return nil, err
	}

	return newWithClients(sys, client, api.SiteURL)
}

func newWithClients(sys *System, client *api.Client, siteURL string) (*System, error) {
	sys.client = client

	if sys.hasAccount() {
		web, err := newWebSession(
			siteURL,
			sys.settings.Credential(system.CredentialUsername),
			sys.settings.Credential(system.CredentialPassword),
			sys.proxy,
			sys.logger,
		)
		if err != nil {
			return nil, err
		}
		sys.web = web
	}

	return sys, nil
}

func titleFor(forumID string, subject string) string {
	title := strings.TrimSpace(subject)
	lower := strings.ToLower(title)
	switch forumID {
	case "ask":
		if !strings.HasPrefix(lower, "ask hn") {
			title = "Ask HN: " + title
		}
	case "show":
		if !strings.HasPrefix(lower, "show hn") {
			title = "Show HN: " + title
		}
	}
	return title
}

func (sys *System) hasAccount() bool {
	return sys.settings.Credential(system.CredentialUsername) != "" &&
		sys.settings.Credential(system.CredentialPassword) != ""
}

func (sys *System) Kind() string {
	return Kind
}

func (sys *System) URL() string {
	return api.SiteURL
}

func (sys *System) Title() string {
	return "news.ycombinator.com"
}

func (sys *System) Description() string {
	return system.Describe("Hacker News", sys.hasAccount())
}

func (sys *System) Capabilities() system.Capabilities {
	return system.AccountCapabilities(sys.hasAccount())
}

func (sys *System) ListForums(ctx context.Context) ([]forum.Forum, error) {
	out := make([]forum.Forum, 0, len(forums))
	for _, f := range forums {
		out = append(out, forum.Forum{
			ID:     f.ID,
			Name:   f.Name,
			Info:   f.Info,
			SysIDX: sys.idx,
		})
	}
	return out, nil
}

func (sys *System) Orders(forumID string) system.Ordering {
	f, ok := forumByID(forumID)
	if !ok {
		return system.Only(system.OrderNew)
	}
	return f.ordering()
}

func (sys *System) ListPosts(
	ctx context.Context,
	forumID string,
	order system.Order,
) ([]post.Post, error) {
	f, ok := forumByID(forumID)
	if !ok {
		return nil, fmt.Errorf("unknown Hacker News forum %q", forumID)
	}

	switch {
	case order == system.OrderHot:
		return sys.listStories(ctx, f.HotList, f)
	case slices.Contains(system.TopOrders(), order):
		return sys.searchStories(ctx, f.Tag, topSince(order, time.Now()))
	case f.NewList != "":
		return sys.listStories(ctx, f.NewList, f)
	}
	return sys.searchNewest(ctx, f.Tag)
}

func (sys *System) listStories(ctx context.Context, list string, scope hnForum) ([]post.Post, error) {
	ids, err := sys.client.Stories(ctx, list)
	if err != nil {
		return nil, err
	}
	if len(ids) > listLimit {
		ids = ids[:listLimit]
	}

	items, err := sys.client.Items(ctx, ids)
	if err != nil {
		return nil, err
	}

	models := make([]post.Post, 0, len(items))
	for _, item := range items {
		if item.Deleted || item.Dead {
			continue
		}
		p := sys.toPost(item)
		if scope.ID != "" {
			p.Forum = forum.Forum{ID: scope.ID, Name: scope.Name, SysIDX: sys.idx}
		}
		models = append(models, p)
	}

	return models, nil
}

func (sys *System) searchStories(ctx context.Context, tag string, since time.Time) ([]post.Post, error) {
	hits, err := sys.client.Search(ctx, tag, since, listLimit)
	if err != nil {
		return nil, err
	}
	return sys.hitsToPosts(hits), nil
}

func (sys *System) searchNewest(ctx context.Context, tag string) ([]post.Post, error) {
	hits, err := sys.client.SearchByDate(ctx, tag, listLimit)
	if err != nil {
		return nil, err
	}
	return sys.hitsToPosts(hits), nil
}

func (sys *System) hitsToPosts(hits []api.SearchHit) []post.Post {
	models := make([]post.Post, 0, len(hits))
	for _, h := range hits {
		models = append(models, sys.hitToPost(h))
	}
	return models
}

func (sys *System) forumFor(title string, job bool) forum.Forum {
	lower := strings.ToLower(strings.TrimSpace(title))

	id := ""
	switch {
	case job:
		id = "jobs"
	case strings.HasPrefix(lower, "ask hn"):
		id = "ask"
	case strings.HasPrefix(lower, "show hn"):
		id = "show"
	default:
		return forum.Forum{SysIDX: sys.idx}
	}

	f, _ := forumByID(id)
	return forum.Forum{ID: f.ID, Name: f.Name, SysIDX: sys.idx}
}

func (sys *System) forumForHit(h api.SearchHit) forum.Forum {
	for _, f := range forums {
		if slices.Contains(h.Tags, f.Tag) {
			return forum.Forum{ID: f.ID, Name: f.Name, SysIDX: sys.idx}
		}
	}
	return sys.forumFor(h.Title, false)
}

func points(value int, job bool) post.Score {
	if job {
		return post.Score{}
	}
	return post.Score{Value: value, Unit: post.ScorePoints}
}

func (sys *System) toPost(item *api.Item) post.Post {
	kind, body := post.LinkBody(item.URL, text.Markdown(item.Text))
	job := item.Type == "job"

	return post.Post{
		ID:      strconv.Itoa(item.ID),
		Subject: item.Title,
		Body:    body,
		Kind:    kind,

		Closed: item.Deleted || item.Dead,

		CreatedAt: time.Unix(item.Time, 0),

		Author: author.Author{
			ID:   item.By,
			Name: item.By,
		},

		Forum: sys.forumFor(item.Title, job),

		ReplyCount: item.Descendants,
		Score:      points(item.Score, job),

		URL:  fmt.Sprintf("%s/item?id=%d", api.SiteURL, item.ID),
		Link: item.URL,

		SysIDX: sys.idx,
	}
}

func (sys *System) hitToPost(h api.SearchHit) post.Post {
	kind, body := post.LinkBody(h.URL, text.Markdown(h.Text()))
	job := slices.Contains(h.Tags, "job")

	return post.Post{
		ID:      h.ObjectID,
		Subject: h.Title,
		Body:    body,
		Kind:    kind,

		CreatedAt: time.Unix(h.CreatedAtI, 0),

		Author: author.Author{
			ID:   h.Author,
			Name: h.Author,
		},

		Forum: sys.forumForHit(h),

		ReplyCount: h.Comments(),
		Score:      points(h.PointsOrZero(), job),

		URL:  fmt.Sprintf("%s/item?id=%s", api.SiteURL, h.ObjectID),
		Link: h.URL,

		SysIDX: sys.idx,
	}
}

func (sys *System) LoadPost(ctx context.Context, p *post.Post) error {
	id, err := strconv.Atoi(p.ID)
	if err != nil {
		return fmt.Errorf("invalid item id %q: %w", p.ID, err)
	}

	pending := sys.recentReplies(p.ID)

	tree, err := sys.client.Tree(ctx, id)
	switch {
	case err != nil && ctx.Err() != nil:
		return err
	case err != nil:
		sys.logger.Warn("algolia tree failed, falling back to the official api",
			"id", id, "error", err)
	case !treeContains(tree, pending):
		sys.logger.Debug("algolia tree lacks a recent reply, using the official api",
			"id", id)
	default:
		sys.applyTree(p, tree)
		return nil
	}

	return sys.loadFromFirebase(ctx, p, id)
}

func (sys *System) recordWrite(postID string, replyID string) {
	sys.writesMu.Lock()
	defer sys.writesMu.Unlock()

	if sys.writes == nil {
		sys.writes = map[string][]recentWrite{}
	}
	if replyID == "" {
		replyID = "?"
	}
	sys.writes[postID] = append(sys.writes[postID], recentWrite{at: time.Now(), replyID: replyID})
}

func (sys *System) recentReplies(postID string) []string {
	sys.writesMu.Lock()
	defer sys.writesMu.Unlock()

	var ids []string
	var kept []recentWrite
	for _, w := range sys.writes[postID] {
		if time.Since(w.at) > recentWriteWindow {
			continue
		}
		kept = append(kept, w)
		ids = append(ids, w.replyID)
	}
	if len(kept) == 0 {
		delete(sys.writes, postID)
	} else {
		sys.writes[postID] = kept
	}

	return ids
}

func treeContains(tree *api.AlgoliaItem, ids []string) bool {
	if len(ids) == 0 {
		return true
	}

	present := map[string]bool{}
	var walk func(items []api.AlgoliaItem)
	walk = func(items []api.AlgoliaItem) {
		for i := range items {
			present[strconv.Itoa(items[i].ID)] = true
			walk(items[i].Children)
		}
	}
	walk(tree.Children)

	for _, id := range ids {
		if !present[id] {
			return false
		}
	}
	return true
}

func (sys *System) applyTree(p *post.Post, tree *api.AlgoliaItem) {
	if p.Kind == post.KindText && tree.Body() != "" {
		p.Body = text.Markdown(tree.Body())
	}
	if tree.Title != nil && *tree.Title != "" {
		p.Subject = *tree.Title
	}

	p.Replies = sys.fromAlgolia(p.ID, tree.Children)
	p.ReplyCount = reply.Count(p.Replies)
}

func (sys *System) fromAlgolia(postID string, children []api.AlgoliaItem) []reply.Reply {
	out := make([]reply.Reply, 0, len(children))
	for i := range children {
		c := &children[i]
		r := reply.Reply{
			ID:        strconv.Itoa(c.ID),
			PostID:    postID,
			Body:      text.Markdown(c.Body()),
			Deleted:   c.IsDeleted(),
			CreatedAt: time.Unix(c.CreatedAtI, 0),
			Author: author.Author{
				ID:   c.AuthorName(),
				Name: c.AuthorName(),
			},
			SysIDX: sys.idx,
		}
		if c.ParentID != nil && strconv.Itoa(*c.ParentID) != postID {
			r.ParentID = strconv.Itoa(*c.ParentID)
		}
		r.Replies = sys.fromAlgolia(postID, c.Children)
		out = append(out, r)
	}
	return out
}

func (sys *System) loadFromFirebase(ctx context.Context, p *post.Post, id int) error {
	root, err := sys.client.Item(ctx, id)
	if err != nil {
		return err
	}

	p.Refresh(sys.toPost(root))

	items, err := sys.client.Descendants(ctx, root)
	if err != nil && len(items) == 0 {
		return err
	}

	var build func(parentID string, kids []int) []reply.Reply
	build = func(parentID string, kids []int) []reply.Reply {
		out := make([]reply.Reply, 0, len(kids))
		for _, kid := range kids {
			item, ok := items[kid]
			if !ok {
				continue
			}
			r := reply.Reply{
				ID:        strconv.Itoa(item.ID),
				PostID:    p.ID,
				Body:      text.Markdown(item.Text),
				Deleted:   item.Deleted || item.Dead,
				CreatedAt: time.Unix(item.Time, 0),
				Author: author.Author{
					ID:   item.By,
					Name: item.By,
				},
				SysIDX: sys.idx,
			}
			if parentID != p.ID {
				r.ParentID = parentID
			}
			r.Replies = build(r.ID, item.Kids)
			out = append(out, r)
		}
		return out
	}

	p.Replies = build(p.ID, root.Kids)
	return nil
}

func (sys *System) CreatePost(ctx context.Context, p *post.Post) error {
	if !sys.Capabilities().Has(system.CapCreatePost) {
		return system.NoCredentials(Kind, sys.URL())
	}

	title := titleFor(p.Forum.ID, p.Subject)

	link := ""
	body := p.Body
	if p.Kind == post.KindLink {
		link = strings.TrimSpace(p.Body)
		body = ""
	}

	id, err := sys.web.Submit(ctx, title, link, body)
	if err != nil {
		return err
	}

	p.Subject = title
	p.ID = id
	sys.recordWrite(id, "")
	return nil
}

func (sys *System) CreateReply(ctx context.Context, r *reply.Reply) error {
	if !sys.Capabilities().Has(system.CapCreateReply) {
		return system.NoCredentials(Kind, sys.URL())
	}

	parent := r.ParentID
	if parent == "" {
		parent = r.PostID
	}

	id, err := sys.web.Comment(ctx, parent, r.Body)
	if err != nil {
		return err
	}

	r.ID = id
	sys.recordWrite(r.PostID, id)
	return nil
}
