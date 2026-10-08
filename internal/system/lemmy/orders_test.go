package lemmy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/system"
)

func TestListPostsSendsTheSortType(t *testing.T) {
	var mu sync.Mutex
	var sorts, communities []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/post/list" {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		sorts = append(sorts, r.URL.Query().Get("sort"))
		communities = append(communities, r.URL.Query().Get("community_id"))
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"posts":[{
			"post":{"id":7,"name":"Linux 7.3 released","creator_id":2,"community_id":3,
				"published":"2026-10-07T08:00:00Z"},
			"creator":{"name":"kernelfan"},
			"community":{"name":"linux"},
			"counts":{"comments":342,"score":1204}}]}`)
	}))
	t.Cleanup(srv.Close)

	sys, err := New(system.Env{Settings: system.Settings{URL: srv.URL}})
	if err != nil {
		t.Fatal(err)
	}

	ordering := sys.Orders("")
	want := map[system.Order]string{
		system.OrderNew:      "New",
		system.OrderActive:   "NewComments",
		system.OrderHot:      "Hot",
		system.OrderTopDay:   "TopDay",
		system.OrderTopWeek:  "TopWeek",
		system.OrderTopMonth: "TopMonth",
		system.OrderTopYear:  "TopYear",
		system.OrderTopAll:   "TopAll",
		system.OrderComments: "MostComments",
	}
	for _, order := range system.AllOrders() {
		if !ordering.Supports(order) {
			t.Errorf("Lemmy should support %q", order)
		}
		posts, err := sys.ListPosts(context.Background(), "", order)
		if err != nil {
			t.Fatalf("ListPosts(%s): %v", order, err)
		}
		if len(posts) != 1 || posts[0].Score != (post.Score{Value: 1204, Unit: post.ScorePoints}) {
			t.Fatalf("ListPosts(%s): %+v", order, posts)
		}
	}
	if _, err := sys.ListPosts(context.Background(), "3", system.OrderTopWeek); err != nil {
		t.Fatalf("community list: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	for i, order := range system.AllOrders() {
		if sorts[i] != want[order] {
			t.Errorf("%s sent sort=%q, want %q", order, sorts[i], want[order])
		}
	}
	if sorts[len(sorts)-1] != "TopWeek" || communities[len(communities)-1] != "3" {
		t.Errorf("community list sent sort=%q community_id=%q", sorts[len(sorts)-1], communities[len(communities)-1])
	}
}
