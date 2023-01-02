package aggregator

import (
	"encoding/json"
	"os"
	"sort"

	"github.com/mrusme/gobbs/models/post"
	"github.com/mrusme/gobbs/models/reply"
	"github.com/mrusme/gobbs/ui/ctx"
)

type Aggregator struct {
	ctx *ctx.Ctx
}

func New(c *ctx.Ctx) (*Aggregator, error) {
	a := new(Aggregator)
	a.ctx = c

	return a, nil
}

func (a *Aggregator) ListPosts() ([]post.Post, []error) {
	var errs []error = make([]error, len(a.ctx.Systems))
	var posts []post.Post

	// TODO: Clean up implementation
	if os.Getenv("GOBBS_TEST") == "true" {
		jsonPosts, err := os.ReadFile("posts.db")
		if err == nil {
			err = json.Unmarshal(jsonPosts, &posts)
			if err == nil {
				return posts, nil
			}
		}
	}

	for idx, sys := range a.ctx.Systems {
		sysPosts, err := (*sys).ListPosts()
		if err != nil {
			errs[idx] = err
			continue
		}
		posts = append(posts, sysPosts...)
	}

	sort.SliceStable(posts, func(i, j int) bool {
		return posts[i].CreatedAt.After(posts[j].CreatedAt)
	})

	// TODO: Clean up implementation
	jsonPosts, err := json.Marshal(posts)
	if err == nil {
		os.WriteFile("posts.db", jsonPosts, 0600)
	}

	return posts, errs
}

func (a *Aggregator) LoadPost(p *post.Post) error {
	return (*a.ctx.Systems[p.SysIDX]).LoadPost(p)
}

func (a *Aggregator) CreatePost(p *post.Post) error {
	return (*a.ctx.Systems[p.SysIDX]).CreatePost(p)
}

func (a *Aggregator) CreateReply(r *reply.Reply) error {
	return (*a.ctx.Systems[r.SysIDX]).CreateReply(r)
}
