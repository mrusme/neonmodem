package aggregator

import (
	"encoding/json"
	"os"
	"sort"
	"strings"

	"github.com/mrusme/neonmodem/models/forum"
	"github.com/mrusme/neonmodem/models/post"
	"github.com/mrusme/neonmodem/models/reply"
	"github.com/mrusme/neonmodem/ui/ctx"
)

type Aggregator struct {
	ctx *ctx.Ctx
}

func New(c *ctx.Ctx) (*Aggregator, error) {
	a := new(Aggregator)
	a.ctx = c

	return a, nil
}

func (a *Aggregator) ListForums() ([]forum.Forum, []error) {
	var errs []error = make([]error, len(a.ctx.Systems))
	var forums []forum.Forum

	for idx, sys := range a.ctx.Systems {
		if curSysIDX := a.ctx.GetCurrentSystem(); curSysIDX != -1 {
			if idx != curSysIDX {
				continue
			}
		}

		sysForums, err := (*sys).ListForums()
		if err != nil {
			errs[idx] = err
			continue
		}
		forums = append(forums, sysForums...)
	}

	sort.SliceStable(forums, func(i, j int) bool {
		return strings.Compare(forums[i].Title(), forums[j].Title()) == -1
	})

	return forums, errs

}

func (a *Aggregator) ListPosts() ([]post.Post, []error) {
	var errs []error = make([]error, len(a.ctx.Systems))
	var posts []post.Post

	// TODO: Clean up implementation
	if os.Getenv("neonmodem_TEST") == "true" {
		jsonPosts, err := os.ReadFile("posts.db")
		if err == nil {
			err = json.Unmarshal(jsonPosts, &posts)
			if err == nil {
				return posts, nil
			}
		}
	}

	for idx, sys := range a.ctx.Systems {
		if curSysIDX := a.ctx.GetCurrentSystem(); curSysIDX != -1 {
			if idx != curSysIDX {
				continue
			}
		}

		sysPosts, err := (*sys).ListPosts(a.ctx.GetCurrentForum().ID)
		if err != nil {
			a.ctx.Logger.Errorf("aggregator error: %v\n", err)
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
