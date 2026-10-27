package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	routeV4Login         = "POST /api/v4/account/auth/login"
	routeV4Validate      = "GET /api/v4/account/validate_auth"
	routeV4Communities   = "GET /api/v4/community/list"
	routeV4Posts         = "GET /api/v4/post/list"
	routeV4Post          = "GET /api/v4/post"
	routeV4Comments      = "GET /api/v4/comment/list"
	routeV4CreatePost    = "POST /api/v4/post"
	routeV4CreateComment = "POST /api/v4/comment"

	v4LoginOK = `{"jwt":"jwt-1","registration_created":false,"verify_email_sent":false}`
)

func TestV4PostsSendTheListingSortTimeRangeAndReadMarks(t *testing.T) {
	f, sysURL := newFakeServer(t, V4, map[string][]reply{routeV4Posts: ok(fixture(t, "v4/posts.json"))})
	c := newTestClient(t, sysURL, "")
	ctx := context.Background()

	for _, tc := range []struct {
		sort      Sort
		want      string
		timeRange string
	}{
		{SortNew, "new", "0"},
		{SortNewComments, "new_comments", "0"},
		{SortHot, "hot", "0"},
		{SortTopDay, "top", "86400"},
		{SortTopWeek, "top", "604800"},
		{SortTopMonth, "top", "2592000"},
		{SortTopYear, "top", "31536000"},
		{SortTopAll, "top", "0"},
		{SortMostComments, "most_comments", "0"},
	} {
		if _, err := c.Posts(ctx, PostQuery{Listing: ListingAll, Sort: tc.sort, Limit: 50}); err != nil {
			t.Fatal(err)
		}
		q := f.last(t).Query
		if q.Get("sort") != tc.want || q.Get("time_range_seconds") != tc.timeRange {
			t.Errorf("sort %d sent %v, want sort=%s and time_range_seconds=%s", tc.sort, q, tc.want, tc.timeRange)
		}
		if q.Get("type_") != "all" || q.Get("limit") != "50" || q.Get("mark_as_read") != "false" ||
			q.Has("community_id") || q.Has("page_cursor") || len(q) != 5 {
			t.Errorf("sort %d sent %v", tc.sort, q)
		}
	}

	for _, tc := range []struct {
		listing Listing
		want    string
	}{
		{ListingAll, "all"},
		{ListingLocal, "local"},
		{ListingSubscribed, "subscribed"},
	} {
		if _, err := c.Posts(ctx, PostQuery{Listing: tc.listing, CommunityID: 3, Limit: 50, MarkAsRead: true}); err != nil {
			t.Fatal(err)
		}
		q := f.last(t).Query
		if q.Get("type_") != tc.want || q.Get("community_id") != "3" || q.Get("mark_as_read") != "true" {
			t.Errorf("listing %s sent %v, want type_=%s", tc.listing, q, tc.want)
		}
	}
}

func TestV4PostsAreMapped(t *testing.T) {
	_, sysURL := newFakeServer(t, V4, map[string][]reply{routeV4Posts: ok(fixture(t, "v4/posts.json"))})
	c := newTestClient(t, sysURL, "")

	posts, err := c.Posts(context.Background(), PostQuery{Listing: ListingAll, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 2 {
		t.Fatalf("got %d posts, want 2", len(posts))
	}
	samePost(t, posts[0], Post{
		ID:            12,
		Name:          "Cobalt ridge almanac",
		URL:           "https://example.com/",
		CreatorID:     4,
		CreatorName:   "vera",
		CommunityID:   3,
		CommunityName: "testing",
		Published:     at("2026-10-27T01:45:11.229176Z"),
		Score:         1,
	})
	samePost(t, posts[1], Post{
		ID:            11,
		Name:          "Walnut prairie compass",
		Body:          "Hazel *quartz* meadow drifts by the cedar.",
		CreatorID:     4,
		CreatorName:   "vera",
		CommunityID:   3,
		CommunityName: "testing",
		Published:     at("2026-10-27T01:45:10.957352Z"),
		Comments:      3,
		Score:         1,
	})
}

func TestV4PostAndCommentsAreMapped(t *testing.T) {
	f, sysURL := newFakeServer(t, V4, map[string][]reply{
		routeV4Post:     ok(fixture(t, "v4/post.json")),
		routeV4Comments: ok(fixture(t, "v4/comments.json")),
	})
	c := newTestClient(t, sysURL, "")
	ctx := context.Background()

	p, err := c.Post(ctx, 11)
	if err != nil {
		t.Fatal(err)
	}
	if q := f.last(t).Query; q.Get("id") != "11" || len(q) != 1 {
		t.Errorf("the post request sent %v", q)
	}
	if p.ID != 11 || p.Name != "Walnut prairie compass" || p.Comments != 3 || p.CommunityName != "testing" {
		t.Errorf("got %+v", p)
	}

	comments, err := c.Comments(ctx, 11, 8)
	if err != nil {
		t.Fatal(err)
	}
	q := f.last(t).Query
	if q.Get("post_id") != "11" || q.Get("max_depth") != "8" || q.Get("sort") != "old" ||
		q.Get("type_") != "all" || q.Get("time_range_seconds") != "0" || len(q) != 5 {
		t.Errorf("the comment request sent %v", q)
	}
	sameComments(t, comments, []Comment{
		{ID: 13, Path: "0.13", Content: "Juniper basalt orchard", CreatorID: 4, CreatorName: "vera",
			Published: at("2026-10-27T01:45:11.057568Z")},
		{ID: 15, Path: "0.15", Content: "Copper lagoon sparrow", CreatorID: 4, CreatorName: "vera",
			Published: at("2026-10-27T01:45:11.186689Z")},
		{ID: 14, Path: "0.13.14", Content: "Marble kestrel thistle", CreatorID: 4, CreatorName: "vera",
			Published: at("2026-10-27T01:45:11.128238Z")},
	})
}

func TestV4CommunitiesAreMapped(t *testing.T) {
	f, sysURL := newFakeServer(t, V4, map[string][]reply{routeV4Communities: ok(fixture(t, "v4/communities.json"))})
	c := newTestClient(t, sysURL, "")

	communities, err := c.Communities(context.Background(), ListingLocal, 50, 10)
	if err != nil {
		t.Fatal(err)
	}

	requests := f.all()
	if len(requests) != 1 {
		t.Fatalf("a reply without next_page made %d requests", len(requests))
	}
	q := requests[0].Query
	if q.Get("type_") != "local" || q.Get("sort") != "subscribers" || q.Get("limit") != "50" || len(q) != 3 {
		t.Errorf("sent %v", q)
	}

	want := []Community{
		{ID: 2, Name: "general", Title: "General", Summary: "General talk",
			Description: "## Rules\n\nThe general community of this instance.\n\n- Be kind"},
		{ID: 3, Name: "testing", Title: "Testing", Summary: "Things to try",
			Description: "## Rules\n\nThe testing community of this instance.\n\n- Be kind"},
	}
	if len(communities) != len(want) {
		t.Fatalf("got %+v", communities)
	}
	for i := range want {
		if communities[i] != want[i] {
			t.Errorf("got %+v, want %+v", communities[i], want[i])
		}
	}
}

func cursorPage(t *testing.T, next string, ids ...int) string {
	t.Helper()

	var captured struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal([]byte(fixture(t, "v4/communities.json")), &captured); err != nil {
		t.Fatal(err)
	}

	items := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		raw, err := json.Marshal(captured.Items[0])
		if err != nil {
			t.Fatal(err)
		}
		var item map[string]any
		if err := json.Unmarshal(raw, &item); err != nil {
			t.Fatal(err)
		}
		community := item["community"].(map[string]any)
		community["id"] = id
		community["name"] = fmt.Sprintf("c%d", id)
		delete(community, "title")
		items = append(items, item)
	}

	page := map[string]any{"items": items}
	if next != "" {
		page["next_page"] = next
	}
	b, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestV4CommunitiesFollowTheCursor(t *testing.T) {
	f, sysURL := newFakeServer(t, V4, map[string][]reply{routeV4Communities: {
		{http.StatusOK, cursorPage(t, "Pc2", 1, 2)},
		{http.StatusOK, cursorPage(t, "Pc3", 3, 4)},
		{http.StatusOK, cursorPage(t, "", 5)},
	}})
	c := newTestClient(t, sysURL, "")

	communities, err := c.Communities(context.Background(), ListingAll, 2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(communities) != 5 || communities[4].ID != 5 || communities[4].Title != "" {
		t.Errorf("got %+v", communities)
	}
	requests := f.all()
	if len(requests) != 3 {
		t.Fatalf("made %d requests", len(requests))
	}
	for i, want := range []string{"", "Pc2", "Pc3"} {
		if got := requests[i].Query.Get("page_cursor"); got != want || requests[i].Query.Has("page_cursor") != (want != "") {
			t.Errorf("request %d sent page_cursor %q, want %q", i, got, want)
		}
	}

	pages := make([]reply, 0, 12)
	for page := range 12 {
		pages = append(pages, reply{http.StatusOK, cursorPage(t, fmt.Sprintf("P%d", page+1), 2*page+1, 2*page+2)})
	}
	f, sysURL = newFakeServer(t, V4, map[string][]reply{routeV4Communities: pages})
	c = newTestClient(t, sysURL, "")

	communities, err = c.Communities(context.Background(), ListingAll, 2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(communities) != 20 || len(f.all()) != 10 {
		t.Errorf("ten pages gave %d communities after %d requests", len(communities), len(f.all()))
	}
}

func TestV4LoginStaysLoggedIn(t *testing.T) {
	f, sysURL := newFakeServer(t, V4, map[string][]reply{routeV4Login: ok(v4LoginOK)})
	c := newTestClient(t, sysURL, "")
	ctx := context.Background()

	jwt, err := c.Login(ctx, Login{Username: "vera", Password: "secret"})
	if err != nil || jwt != "jwt-1" {
		t.Fatalf("got %q and %v", jwt, err)
	}
	r := f.last(t)
	if r.Body["username_or_email"] != "vera" || r.Body["password"] != "secret" ||
		r.Body["stay_logged_in"] != true || r.Body["totp_2fa_token"] != nil || r.Auth != "" {
		t.Errorf("sent %v with Authorization %q", r.Body, r.Auth)
	}

	if _, err := c.Login(ctx, Login{Username: "vera", Password: "secret", Code: "123456"}); err != nil {
		t.Fatal(err)
	}
	if body := f.last(t).Body; body["totp_2fa_token"] != "123456" || body["stay_logged_in"] != true {
		t.Errorf("sent %v", body)
	}
}

func TestV4LoginWithoutATokenFails(t *testing.T) {
	_, sysURL := newFakeServer(t, V4, map[string][]reply{
		routeV4Login: ok(`{"registration_created":false,"verify_email_sent":true}`),
	})
	c := newTestClient(t, sysURL, "")

	if _, err := c.Login(context.Background(), Login{Username: "vera", Password: "secret"}); !errors.Is(err, ErrNoToken) {
		t.Errorf("got %v, want ErrNoToken", err)
	}
}

func TestV4CallsThatNeedATokenFailWithoutOne(t *testing.T) {
	f, sysURL := newFakeServer(t, V4, flavors[1].fixtures(t))
	c := newTestClient(t, sysURL, "")
	ctx := context.Background()

	for i, err := range []error{
		c.ValidateAuth(ctx),
		second(c.CreatePost(ctx, NewPost{CommunityID: 3, Name: "Name", Body: "Body"})),
		second(c.CreateComment(ctx, NewComment{PostID: 11, Content: "Content"})),
	} {
		if err == nil || !strings.Contains(err.Error(), "security requirement is not satisfied") {
			t.Errorf("call %d: got %v", i, err)
		}
	}
	if requests := f.all(); len(requests) != 0 {
		t.Errorf("made requests %v", requests)
	}
}

func TestV4CreatesSendTheirFields(t *testing.T) {
	f, sysURL := newFakeServer(t, V4, map[string][]reply{
		routeV4CreatePost:    ok(fixture(t, "v4/create_post.json")),
		routeV4CreateComment: ok(fixture(t, "v4/create_comment.json")),
	})
	c := newTestClient(t, sysURL, "session-token")
	ctx := context.Background()

	id, err := c.CreatePost(ctx, NewPost{CommunityID: 3, Name: "Cobalt ridge almanac", URL: "https://example.com/"})
	if err != nil || id != 12 {
		t.Fatalf("got %d and %v", id, err)
	}
	body := f.last(t).Body
	if body["name"] != "Cobalt ridge almanac" || body["community_id"] != float64(3) ||
		body["url"] != "https://example.com/" || body["body"] != nil || body["nsfw"] != false {
		t.Errorf("the link post sent %v", body)
	}

	if _, err := c.CreatePost(ctx, NewPost{CommunityID: 3, Name: "Walnut prairie compass", Body: "Hazel"}); err != nil {
		t.Fatal(err)
	}
	if body := f.last(t).Body; body["body"] != "Hazel" || body["url"] != nil {
		t.Errorf("the text post sent %v", body)
	}

	id, err = c.CreateComment(ctx, NewComment{PostID: 11, ParentID: 13, Content: "Marble kestrel thistle"})
	if err != nil || id != 14 {
		t.Fatalf("got %d and %v", id, err)
	}
	body = f.last(t).Body
	if body["post_id"] != float64(11) || body["parent_id"] != float64(13) || body["content"] != "Marble kestrel thistle" {
		t.Errorf("the reply sent %v", body)
	}

	if _, err := c.CreateComment(ctx, NewComment{PostID: 11, Content: "Juniper"}); err != nil {
		t.Fatal(err)
	}
	if body := f.last(t).Body; body["parent_id"] != nil {
		t.Errorf("the top-level comment sent %v", body)
	}
}

func TestV4AnUnknownEnumValueFailsTheWholeList(t *testing.T) {
	_, sysURL := newFakeServer(t, V4, map[string][]reply{routeV4Posts: ok(fixture(t, "v4/posts_unknown_visibility.json"))})
	c := newTestClient(t, sysURL, "")

	posts, err := c.Posts(context.Background(), PostQuery{Listing: ListingAll, Limit: 50})
	if err == nil || !strings.Contains(err.Error(), "validate") || !strings.Contains(err.Error(), "followers_only") {
		t.Fatalf("got %d posts and %v, want the validation error", len(posts), err)
	}
	if _, ok := errors.AsType[*Error](err); ok {
		t.Errorf("the validation error became a Lemmy error: %v", err)
	}
}

func TestV4CallsEndWithTheirContext(t *testing.T) {
	info := fixture(t, "v4/nodeinfo.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == nodeinfoPath {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, info)
			return
		}
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	t.Cleanup(func() {
		srv.CloseClientConnections()
		srv.Close()
	})

	c := newTestClient(t, srv.URL, "session-token")
	if v, err := c.Version(context.Background()); err != nil || v != V4 {
		t.Fatalf("got version %d and %v", v, err)
	}

	for _, call := range []struct {
		name string
		run  func(ctx context.Context) error
	}{
		{"Login", func(ctx context.Context) error {
			return second(c.Login(ctx, Login{Username: "vera", Password: "secret"}))
		}},
		{"ValidateAuth", c.ValidateAuth},
		{"Communities", func(ctx context.Context) error { return second(c.Communities(ctx, ListingAll, 50, 10)) }},
		{"Posts", func(ctx context.Context) error { return second(c.Posts(ctx, PostQuery{Limit: 50})) }},
		{"Post", func(ctx context.Context) error { return second(c.Post(ctx, 11)) }},
		{"Comments", func(ctx context.Context) error { return second(c.Comments(ctx, 11, 8)) }},
		{"CreatePost", func(ctx context.Context) error {
			return second(c.CreatePost(ctx, NewPost{CommunityID: 3, Name: "Name", Body: "Body"}))
		}},
		{"CreateComment", func(ctx context.Context) error {
			return second(c.CreateComment(ctx, NewComment{PostID: 11, Content: "Content"}))
		}},
	} {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		start := time.Now()
		err := call.run(ctx)
		cancel()

		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("%s: got %v, want an error that matches the deadline", call.name, err)
		}
		if waited := time.Since(start); waited > time.Second {
			t.Errorf("%s returned after %s", call.name, waited)
		}
	}
}
