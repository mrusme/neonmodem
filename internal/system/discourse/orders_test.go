package discourse

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/system"
)

func TestListPostsRequestsTheListForEachOrder(t *testing.T) {
	f, srv := newFakeDiscourse(t)
	sys := testSystem(t, srv, nil)

	want := map[system.Order]string{
		system.OrderNew:      "latest.json?order=created",
		system.OrderActive:   "latest.json",
		system.OrderHot:      "hot.json",
		system.OrderTopDay:   "top.json?period=daily",
		system.OrderTopWeek:  "top.json?period=weekly",
		system.OrderTopMonth: "top.json?period=monthly",
		system.OrderTopYear:  "top.json?period=yearly",
		system.OrderTopAll:   "top.json?period=all",
		system.OrderComments: "latest.json?order=posts",
	}

	for _, order := range system.AllOrders() {
		if !sys.Orders("").Supports(order) {
			t.Errorf("Discourse should support %q", order)
		}
		posts, err := sys.ListPosts(context.Background(), "", order)
		if err != nil {
			t.Fatalf("ListPosts(%s): %v", order, err)
		}
		if len(posts) == 0 || posts[0].Score.Unit != post.ScoreLikes {
			t.Fatalf("ListPosts(%s): no likes in %+v", order, posts)
		}
		if _, err := sys.ListPosts(context.Background(), "6", order); err != nil {
			t.Fatalf("ListPosts(%s) in a subcategory: %v", order, err)
		}
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	for i, order := range system.AllOrders() {
		site, category := f.lists[2*i], f.lists[2*i+1]
		if site != "/"+want[order] {
			t.Errorf("%s requested %q, want %q", order, site, "/"+want[order])
		}
		if category != "/c/testing/nested/6/l/"+want[order] {
			t.Errorf("%s requested %q in a category", order, category)
		}
	}
}

func TestMissingHotFallsBackAndIsRemembered(t *testing.T) {
	f, srv := newFakeDiscourse(t)
	f.hotMissing = true
	sys := testSystem(t, srv, nil)

	_, err := sys.ListPosts(context.Background(), "", system.OrderHot)
	if !errors.Is(err, system.ErrOrderUnavailable) {
		t.Fatalf("got %v, want ErrOrderUnavailable", err)
	}
	ordering := sys.Orders("")
	if ordering.Supports(system.OrderHot) {
		t.Error("Hot should no longer be offered after a 404")
	}
	if !slices.Contains(ordering.Supported, system.OrderTopWeek) {
		t.Error("only Hot should be dropped")
	}
}
