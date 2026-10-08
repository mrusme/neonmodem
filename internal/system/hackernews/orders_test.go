package hackernews

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/hackernews/api"
	"github.com/mrusme/neonmodem/internal/system/httpx"
)

func TestOrdersPerForum(t *testing.T) {
	sys := &System{logger: slog.New(slog.DiscardHandler)}
	top := append([]system.Order{system.OrderNew, system.OrderHot}, system.TopOrders()...)

	for _, forumID := range []string{"", "ask", "show"} {
		ordering := sys.Orders(forumID)
		if ordering.Default != system.OrderNew || !slices.Equal(ordering.Supported, top) {
			t.Errorf("forum %q: got %+v", forumID, ordering)
		}
	}

	jobs := sys.Orders("jobs")
	if !slices.Equal(jobs.Supported, []system.Order{system.OrderNew, system.OrderHot}) {
		t.Errorf("job posts have no votes, so Jobs HN has no Top: %+v", jobs)
	}
	if old := sys.Orders("best"); !slices.Equal(old.Supported, []system.Order{system.OrderNew}) {
		t.Errorf("an unknown forum should only offer New: %+v", old)
	}
}

func TestForumsAreAskShowAndJobs(t *testing.T) {
	sys := &System{idx: 2}
	forums, err := sys.ListForums(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	var ids []string
	for _, f := range forums {
		ids = append(ids, f.ID)
		if f.SysIDX != 2 || f.Name == "" {
			t.Errorf("unexpected forum %+v", f)
		}
	}
	if strings.Join(ids, ",") != "ask,jobs,show" {
		t.Errorf("got forums %v", ids)
	}
}

func TestTopAndForumNewComeFromAlgolia(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("api", "testdata", "algolia_search.json"))
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var requests []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.URL.Path+"?"+r.URL.Query().Encode())
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	}))
	t.Cleanup(srv.Close)

	client, err := api.NewClientWithBases(httpx.NewHTTPClient(httpx.Options{}), srv.URL+"/firebase", srv.URL+"/algolia")
	if err != nil {
		t.Fatal(err)
	}
	sys, err := newWithClients(&System{idx: 2, logger: slog.New(slog.DiscardHandler)}, client, srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	calls := []struct {
		forumID string
		order   system.Order
		path    string
		tags    string
		days    int
	}{
		{"", system.OrderTopWeek, "/algolia/search", "story", 7},
		{"", system.OrderTopAll, "/algolia/search", "story", 0},
		{"ask", system.OrderNew, "/algolia/search_by_date", "ask_hn", 0},
		{"show", system.OrderTopDay, "/algolia/search", "show_hn", 1},
		{"jobs", system.OrderNew, "/algolia/search_by_date", "job", 0},
	}

	var posts []post.Post
	for _, c := range calls {
		got, err := sys.ListPosts(context.Background(), c.forumID, c.order)
		if err != nil {
			t.Fatalf("ListPosts(%q, %s): %v", c.forumID, c.order, err)
		}
		posts = got
	}

	mu.Lock()
	defer mu.Unlock()
	for i, c := range calls {
		path, raw, _ := strings.Cut(requests[i], "?")
		query, err := url.ParseQuery(raw)
		if err != nil {
			t.Fatal(err)
		}
		if path != c.path || query.Get("tags") != c.tags || query.Get("hitsPerPage") != "30" {
			t.Errorf("call %d requested %s", i, requests[i])
		}
		filter := query.Get("numericFilters")
		if c.days == 0 {
			if filter != "" {
				t.Errorf("call %d should have no date filter: %q", i, filter)
			}
			continue
		}
		since, err := strconv.ParseInt(strings.TrimPrefix(filter, "created_at_i>"), 10, 64)
		want := time.Now().Add(-time.Duration(c.days) * 24 * time.Hour).Unix()
		if err != nil || since < want-60 || since > want+60 {
			t.Errorf("call %d has the date filter %q, want about %d", i, filter, want)
		}
	}

	if len(posts) != 4 {
		t.Fatalf("expected 4 posts, got %d", len(posts))
	}
	ask, show, tell, job := posts[0], posts[1], posts[2], posts[3]
	if tell.Forum.ID != "ask" || tell.Forum.Name != "Ask HN" {
		t.Errorf("Hacker News counts Tell HN as Ask HN: %+v", tell.Forum)
	}
	if ask.Forum.ID != "ask" || ask.Kind != post.KindText || ask.Body != "Curious about your setup." ||
		ask.ReplyCount != 501 || ask.Score != (post.Score{Value: 412, Unit: post.ScorePoints}) {
		t.Errorf("unexpected Ask HN post: %+v", ask)
	}
	if show.Forum.ID != "show" || show.Kind != post.KindLink || show.Body != "https://example.com/forth" ||
		show.URL != api.SiteURL+"/item?id=50000002" {
		t.Errorf("unexpected Show HN post: %+v", show)
	}
	if job.Forum.ID != "jobs" || job.Score != (post.Score{}) || job.ReplyCount != 0 {
		t.Errorf("a job post should be in Jobs HN without a score: %+v", job)
	}
}
