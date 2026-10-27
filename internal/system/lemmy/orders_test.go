package lemmy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/system"
)

func TestListPostsSendsTheSortType(t *testing.T) {
	eachFlavor(t, func(t *testing.T, fl flavor) {
		var mu sync.Mutex
		var queries []url.Values

		body := fl.onePost(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == nodeinfoPath {
				fl.serveNodeinfo(w)
				return
			}
			if r.Method+" "+r.URL.Path != fl.posts {
				http.NotFound(w, r)
				return
			}
			mu.Lock()
			queries = append(queries, r.URL.Query())
			mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, body)
		}))
		t.Cleanup(srv.Close)

		sys, err := New(system.Env{Settings: system.Settings{URL: srv.URL}})
		if err != nil {
			t.Fatal(err)
		}

		ordering := sys.Orders("")
		for _, order := range system.AllOrders() {
			if !ordering.Supports(order) {
				t.Errorf("Lemmy should support %q", order)
			}
			posts, err := sys.ListPosts(context.Background(), "", order)
			if err != nil {
				t.Fatalf("ListPosts(%s): %v", order, err)
			}
			if len(posts) != 1 || posts[0].Score != (post.Score{Value: 1, Unit: post.ScorePoints}) {
				t.Fatalf("ListPosts(%s): %+v", order, posts)
			}
		}
		if _, err := sys.ListPosts(context.Background(), "3", system.OrderTopWeek); err != nil {
			t.Fatalf("community list: %v", err)
		}

		mu.Lock()
		defer mu.Unlock()
		for i, order := range system.AllOrders() {
			q := queries[i]
			if q.Get("sort") != fl.sorts[order] {
				t.Errorf("%s sent sort=%q, want %q", order, q.Get("sort"), fl.sorts[order])
			}
			if got, want := q.Get("time_range_seconds"), fl.timeRanges[order]; got != want || q.Has("time_range_seconds") != (want != "") {
				t.Errorf("%s sent time_range_seconds=%q, want %q", order, got, want)
			}
		}
		last := queries[len(queries)-1]
		if last.Get("sort") != fl.sorts[system.OrderTopWeek] || last.Get("community_id") != "3" {
			t.Errorf("community list sent %v", last)
		}
	})
}

func TestListPostsSendsTheReadMarks(t *testing.T) {
	eachFlavor(t, func(t *testing.T, fl flavor) {
		for _, tc := range []struct {
			option string
			want   string
		}{
			{"", "false"},
			{"no", "false"},
			{"yes", "true"},
		} {
			f, srv := newFakeLemmy(t, fl, map[string]route{fl.posts: {200, fl.onePost(t)}})
			var queries []url.Values
			f.onQuery = func(q url.Values) { queries = append(queries, q) }

			settings := system.Settings{URL: srv.URL}
			if tc.option != "" {
				settings.Options = map[string]string{OptionMarkAsRead: tc.option}
			}
			sys, err := New(system.Env{Settings: settings})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := sys.ListPosts(context.Background(), "", system.OrderNew); err != nil {
				t.Fatal(err)
			}

			f.mu.Lock()
			q := queries[0]
			f.mu.Unlock()
			if fl.name == "v3" {
				if q.Has("mark_as_read") {
					t.Errorf("%q sent %v", tc.option, q)
				}
				continue
			}
			if q.Get("mark_as_read") != tc.want {
				t.Errorf("%q sent mark_as_read=%q, want %q", tc.option, q.Get("mark_as_read"), tc.want)
			}
		}
	})
}
