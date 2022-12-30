package aggregator

import (
	"sort"

	"github.com/mrusme/gobbs/models/post"
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

	for idx, sys := range a.ctx.Systems {
		sysPosts, err := (*sys).ListPosts(idx)
		if err != nil {
			errs[idx] = err
			continue
		}
		posts = append(posts, sysPosts...)
	}

	sort.SliceStable(posts, func(i, j int) bool {
		return posts[i].CreatedAt.After(posts[j].CreatedAt)
	})

	return posts, errs
}

func (a *Aggregator) LoadPost(p *post.Post) error {
	return (*a.ctx.Systems[p.SysIDX]).LoadPost(p)
}
