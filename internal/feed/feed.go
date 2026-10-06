package feed

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
)

const All = -1

func Selected(systems []system.System, only int) []int {
	if only >= 0 && only < len(systems) {
		return []int{only}
	}
	indexes := make([]int, 0, len(systems))
	for i := range systems {
		indexes = append(indexes, i)
	}
	return indexes
}

func fanOut[T any](
	ctx context.Context,
	systems []system.System,
	only int,
	call func(ctx context.Context, sys system.System) ([]T, error),
) ([]T, []error) {
	errs := make([]error, len(systems))
	results := make([][]T, len(systems))

	var wg sync.WaitGroup
	for _, idx := range Selected(systems, only) {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = call(ctx, systems[idx])
		}(idx)
	}
	wg.Wait()

	var merged []T
	for _, items := range results {
		merged = append(merged, items...)
	}

	return merged, errs
}

func ListForums(ctx context.Context, systems []system.System, only int) ([]forum.Forum, []error) {
	forums, errs := fanOut(ctx, systems, only,
		func(ctx context.Context, sys system.System) ([]forum.Forum, error) {
			return sys.ListForums(ctx)
		})

	sort.SliceStable(forums, func(i, j int) bool {
		return strings.Compare(forums[i].Title(), forums[j].Title()) < 0
	})

	return forums, errs
}

func ListPosts(ctx context.Context, systems []system.System, only int, forumID string) ([]post.Post, []error) {
	posts, errs := fanOut(ctx, systems, only,
		func(ctx context.Context, sys system.System) ([]post.Post, error) {
			return sys.ListPosts(ctx, forumID)
		})

	SortPosts(posts)

	return posts, errs
}

func SortPosts(posts []post.Post) {
	sort.SliceStable(posts, func(i, j int) bool {
		return posts[i].CreatedAt.After(posts[j].CreatedAt)
	})
}

func LoadPost(ctx context.Context, systems []system.System, p *post.Post) error {
	return systems[p.SysIDX].LoadPost(ctx, p)
}

func CreatePost(ctx context.Context, systems []system.System, p *post.Post) error {
	return systems[p.SysIDX].CreatePost(ctx, p)
}

func CreateReply(ctx context.Context, systems []system.System, r *reply.Reply) error {
	return systems[r.SysIDX].CreateReply(ctx, r)
}
