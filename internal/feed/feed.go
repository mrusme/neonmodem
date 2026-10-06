package feed

import (
	"cmp"
	"context"
	"errors"
	"slices"
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

type Result struct {
	System int
	Order  system.Order
	Posts  []post.Post
}

func Choose(ordering system.Ordering, want system.Order) system.Order {
	if ordering.Supports(want) {
		return want
	}
	if want.Compare() != nil {
		return ordering.First(want.Sources())
	}
	return ordering.Default
}

func SortedLocally(listed system.Order, want system.Order) bool {
	return listed != want && want.Compare() != nil
}

func List(
	ctx context.Context,
	idx int,
	sys system.System,
	forumID string,
	want system.Order,
) (Result, error) {
	ordering := sys.Orders(forumID)
	order := Choose(ordering, want)

	posts, err := sys.ListPosts(ctx, forumID, order)
	if errors.Is(err, system.ErrOrderUnavailable) {
		retry := Choose(sys.Orders(forumID), want)
		if retry == order {
			retry = ordering.Default
		}
		if retry != order {
			order = retry
			posts, err = sys.ListPosts(ctx, forumID, order)
		}
	}

	return Result{System: idx, Order: order, Posts: posts}, err
}

func Merge(results []Result, want system.Order) []post.Post {
	results = slices.Clone(results)
	slices.SortStableFunc(results, func(a, b Result) int {
		return cmp.Compare(a.System, b.System)
	})

	if want == system.OrderNew {
		var all []post.Post
		for _, r := range results {
			all = append(all, r.Posts...)
		}
		slices.SortStableFunc(all, want.Compare())
		return all
	}

	type ranked struct {
		post   post.Post
		key    float64
		system int
	}

	var items []ranked
	for _, r := range results {
		posts := r.Posts
		if SortedLocally(r.Order, want) {
			posts = slices.Clone(posts)
			slices.SortStableFunc(posts, want.Compare())
		}
		for rank, p := range posts {
			items = append(items, ranked{
				post:   p,
				key:    float64(rank) / float64(len(posts)),
				system: r.System,
			})
		}
	}

	slices.SortStableFunc(items, func(a, b ranked) int {
		if c := cmp.Compare(a.key, b.key); c != 0 {
			return c
		}
		return cmp.Compare(a.system, b.system)
	})

	merged := make([]post.Post, 0, len(items))
	for _, it := range items {
		merged = append(merged, it.post)
	}
	return merged
}

type Use struct {
	Name  string
	Order system.Order
}

func Status(want system.Order, uses []Use) string {
	type group struct {
		local bool
		order system.Order
		names []string
	}

	var groups []*group
	for _, u := range uses {
		if u.Order == want {
			continue
		}
		local := SortedLocally(u.Order, want)
		idx := slices.IndexFunc(groups, func(g *group) bool {
			return g.local == local && g.order == u.Order
		})
		if idx < 0 {
			groups = append(groups, &group{local: local, order: u.Order})
			idx = len(groups) - 1
		}
		groups[idx].names = append(groups[idx].names, u.Name)
	}

	phrases := make([]string, 0, len(groups))
	for i, g := range groups {
		one := len(g.names) == 1
		names := joinNames(g.names)
		switch {
		case g.local && i > 0:
			phrases = append(phrases, names+" "+possessive(one)+" "+sourceWord(g.order)+" posts")
		case g.local && one:
			phrases = append(phrases, names+" sorts its "+sourceWord(g.order)+
				" posts by "+valueWord(want))
		case g.local:
			phrases = append(phrases, names+" sort their "+sourceWord(g.order)+
				" posts by "+valueWord(want))
		case one:
			phrases = append(phrases, names+" keeps "+g.order.Label())
		default:
			phrases = append(phrases, names+" keep "+g.order.Label())
		}
	}

	return strings.Join(phrases, ", ")
}

func possessive(one bool) string {
	if one {
		return "its"
	}
	return "their"
}

func joinNames(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

func sourceWord(order system.Order) string {
	if order == system.OrderNew {
		return "newest"
	}
	return strings.ToLower(order.Label())
}

func valueWord(want system.Order) string {
	if want == system.OrderNew {
		return "date"
	}
	return "replies"
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
