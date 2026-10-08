package lobsters

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/system"
)

func TestListPostsRequestsTheListForEachOrder(t *testing.T) {
	newest, err := os.ReadFile(filepath.Join("api", "testdata", "newest.json"))
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(newest)
	}))
	t.Cleanup(srv.Close)

	sys, err := New(system.Env{Settings: system.Settings{URL: srv.URL}})
	if err != nil {
		t.Fatal(err)
	}

	feed := sys.Orders("")
	for _, order := range []system.Order{system.OrderNew, system.OrderActive, system.OrderHot} {
		if !feed.Supports(order) {
			t.Errorf("the Lobsters feed should support %q", order)
		}
		posts, err := sys.ListPosts(context.Background(), "", order)
		if err != nil {
			t.Fatalf("ListPosts(%s): %v", order, err)
		}
		if len(posts) == 0 || posts[0].Score.Unit != post.ScorePoints {
			t.Fatalf("ListPosts(%s): no points", order)
		}
	}
	for _, order := range []system.Order{system.OrderTopWeek, system.OrderComments} {
		if feed.Supports(order) {
			t.Errorf("the Lobsters feed has no %q", order)
		}
	}

	tag := sys.Orders("programming")
	if !tag.Supports(system.OrderNew) || tag.Supports(system.OrderHot) {
		t.Errorf("tags should only support New, got %v", tag.Supported)
	}
	if _, err := sys.ListPosts(context.Background(), "programming", system.OrderNew); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	want := []string{"/newest.json", "/active.json", "/hottest.json", "/t/programming.json"}
	if len(paths) != len(want) {
		t.Fatalf("requested %v", paths)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Errorf("request %d went to %q, want %q", i, paths[i], want[i])
		}
	}
}

func TestATagPageNamesTheTagAsTheForum(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if r.URL.Path != "/t/rust.json" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `[{"short_id":"abc","title":"Burn 0.22","url":"https://example.com/burn",
			"created_at":"2026-10-07T12:00:00.000-05:00","submitter_user":"nrposner",
			"tags":["nix","rust"],"comments_url":"https://lobste.rs/s/abc/burn"}]`)
	}))
	t.Cleanup(srv.Close)

	sys, err := New(system.Env{Settings: system.Settings{URL: srv.URL}})
	if err != nil {
		t.Fatal(err)
	}
	posts, err := sys.ListPosts(context.Background(), "rust", system.OrderNew)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 || posts[0].Forum.ID != "rust" || posts[0].Forum.Name != "rust" {
		t.Errorf("got %+v", posts)
	}
}
