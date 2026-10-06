package hackernews

import (
	"context"
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
	"github.com/mrusme/neonmodem/internal/system/hackernews/api"
	"github.com/mrusme/neonmodem/internal/system/httpx"
	"github.com/mrusme/neonmodem/internal/system/text"
)

const (
	Kind = "hackernews"

	listLimit   = 30
	defaultList = "new"

	recentWriteWindow = 10 * time.Minute
)

type list struct {
	ID   string
	Name string
	Info string
}

var lists = []list{
	{ID: "top", Name: "Top HN Stories", Info: "Top stories on Hacker News"},
	{ID: "best", Name: "Best HN Stories", Info: "Best stories on Hacker News"},
	{ID: "new", Name: "New HN Stories", Info: "New stories on Hacker News"},
	{ID: "ask", Name: "Ask HN", Info: "Ask Hacker News about the world"},
	{ID: "show", Name: "Show HN", Info: "Show Hacker News something awesome"},
	{ID: "jobs", Name: "Jobs HN", Info: "... because we can't *all* become Astronauts"},
}

func listByID(id string) list {
	for _, l := range lists {
		if l.ID == id {
			return l
		}
	}
	return list{ID: id, Name: id}
}

type recentWrite struct {
	at      time.Time
	replyID string
}

type System struct {
	idx      int
	settings system.Settings
	proxy    string
	logger   *slog.Logger
	client   *api.Client
	web      *webSession

	writesMu sync.Mutex
	writes   map[string][]recentWrite
}

func New(env system.Env) (system.System, error) {
	sys := &System{
		idx:      env.Index,
		settings: env.Settings,
		proxy:    env.Proxy,
		logger:   env.Log(),
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

	return newWithClients(sys, client, api.SiteURL), nil
}

func newWithClients(sys *System, client *api.Client, siteURL string) *System {
	sys.client = client

	if sys.hasAccount() {
		sys.web = newWebSession(
			siteURL,
			sys.settings.Credential(system.CredentialUsername),
			sys.settings.Credential(system.CredentialPassword),
			sys.proxy,
			sys.logger,
		)
	}

	return sys
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

func (sys *System) Title() string {
	return "news.ycombinator.com"
}

func (sys *System) Description() string {
	if !sys.hasAccount() {
		return "Hacker News (read-only)"
	}
	return "Hacker News"
}

func (sys *System) Capabilities() system.Capabilities {
	caps := system.CapRead
	if sys.hasAccount() {
		caps |= system.CapWrite
	}
	return caps
}

func (sys *System) ListForums(ctx context.Context) ([]forum.Forum, error) {
	forums := make([]forum.Forum, 0, len(lists))
	for _, l := range lists {
		forums = append(forums, forum.Forum{
			ID:     l.ID,
			Name:   l.Name,
			Info:   l.Info,
			SysIDX: sys.idx,
		})
	}
	return forums, nil
}

func (sys *System) ListPosts(ctx context.Context, forumID string) ([]post.Post, error) {
	listID := forumID
	if listID == "" {
		listID = defaultList
	}
	l := listByID(listID)

	ids, err := sys.client.Stories(ctx, l.ID)
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
		models = append(models, sys.toPost(item, l))
	}

	return models, nil
}

func (sys *System) toPost(item *api.Item, l list) post.Post {
	kind := post.KindText
	body := text.Markdown(item.Text)
	if item.URL != "" {
		kind = post.KindLink
		if body == "" {
			body = item.URL
		} else {
			body = item.URL + "\n\n" + body
		}
	}

	createdAt := time.Unix(item.Time, 0)

	return post.Post{
		ID:      strconv.Itoa(item.ID),
		Subject: item.Title,
		Body:    body,
		Kind:    kind,

		Closed: item.Deleted || item.Dead,

		CreatedAt: createdAt,

		Author: author.Author{
			ID:   item.By,
			Name: item.By,
		},

		Forum: forum.Forum{
			ID:     l.ID,
			Name:   l.Name,
			SysIDX: sys.idx,
		},

		ReplyCount: item.Descendants,

		URL: fmt.Sprintf("%s/item?id=%d", api.SiteURL, item.ID),

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

	fresh := sys.toPost(root, listByID(p.Forum.ID))
	p.Subject = fresh.Subject
	p.Body = fresh.Body
	p.Kind = fresh.Kind
	p.ReplyCount = fresh.ReplyCount

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
	if sys.web == nil {
		return system.ErrNoCredentials
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
	if sys.web == nil {
		return system.ErrNoCredentials
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
