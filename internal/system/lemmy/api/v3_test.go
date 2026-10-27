package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

const (
	routeV3Login         = "POST /api/v3/user/login"
	routeV3Validate      = "GET /api/v3/user/validate_auth"
	routeV3Communities   = "GET /api/v3/community/list"
	routeV3Posts         = "GET /api/v3/post/list"
	routeV3Post          = "GET /api/v3/post"
	routeV3Comments      = "GET /api/v3/comment/list"
	routeV3CreatePost    = "POST /api/v3/post"
	routeV3CreateComment = "POST /api/v3/comment"

	v3LoginOK = `{"jwt":"jwt-1","registration_created":false,"verify_email_sent":false}`
)

func TestV3PostsSendTheListingSortAndCommunity(t *testing.T) {
	f, sysURL := newFakeServer(t, V3, map[string][]reply{routeV3Posts: ok(fixture(t, "v3/posts.json"))})
	c := newTestClient(t, sysURL, "")
	ctx := context.Background()

	for _, tc := range []struct {
		sort Sort
		want string
	}{
		{SortNew, "New"},
		{SortNewComments, "NewComments"},
		{SortHot, "Hot"},
		{SortTopDay, "TopDay"},
		{SortTopWeek, "TopWeek"},
		{SortTopMonth, "TopMonth"},
		{SortTopYear, "TopYear"},
		{SortTopAll, "TopAll"},
		{SortMostComments, "MostComments"},
	} {
		if _, err := c.Posts(ctx, PostQuery{Listing: ListingAll, Sort: tc.sort, Limit: 50, MarkAsRead: true}); err != nil {
			t.Fatal(err)
		}
		q := f.last(t).Query
		if q.Get("sort") != tc.want || q.Get("type_") != "All" || q.Get("limit") != "50" {
			t.Errorf("sort %d sent %v, want sort=%s", tc.sort, q, tc.want)
		}
		if q.Has("community_id") || q.Has("mark_as_read") || q.Has("page") {
			t.Errorf("sort %d sent %v", tc.sort, q)
		}
	}

	for _, tc := range []struct {
		listing Listing
		want    string
	}{
		{ListingAll, "All"},
		{ListingLocal, "Local"},
		{ListingSubscribed, "Subscribed"},
	} {
		if _, err := c.Posts(ctx, PostQuery{Listing: tc.listing, CommunityID: 3, Limit: 50}); err != nil {
			t.Fatal(err)
		}
		q := f.last(t).Query
		if q.Get("type_") != tc.want || q.Get("community_id") != "3" || q.Get("sort") != "New" {
			t.Errorf("listing %s sent %v, want type_=%s", tc.listing, q, tc.want)
		}
	}
}

func TestV3PostsAreMapped(t *testing.T) {
	_, sysURL := newFakeServer(t, V3, map[string][]reply{routeV3Posts: ok(fixture(t, "v3/posts.json"))})
	c := newTestClient(t, sysURL, "")

	posts, err := c.Posts(context.Background(), PostQuery{Listing: ListingAll, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 2 {
		t.Fatalf("got %d posts, want 2", len(posts))
	}
	samePost(t, posts[0], Post{
		ID:            17,
		Name:          "Cobalt ridge almanac",
		URL:           "https://example.com/",
		CreatorID:     3,
		CreatorName:   "vera",
		CommunityID:   3,
		CommunityName: "testing",
		Published:     at("2026-10-27T01:31:11.587534Z"),
		Score:         1,
	})
	samePost(t, posts[1], Post{
		ID:            16,
		Name:          "Walnut prairie compass",
		Body:          "Hazel *quartz* meadow drifts by the cedar.",
		CreatorID:     3,
		CreatorName:   "vera",
		CommunityID:   3,
		CommunityName: "testing",
		Published:     at("2026-10-27T01:31:11.48571Z"),
		Comments:      3,
		Score:         1,
	})
}

func TestV3PostAndCommentsAreMapped(t *testing.T) {
	f, sysURL := newFakeServer(t, V3, map[string][]reply{
		routeV3Post:     ok(fixture(t, "v3/post.json")),
		routeV3Comments: ok(fixture(t, "v3/comments.json")),
	})
	c := newTestClient(t, sysURL, "")
	ctx := context.Background()

	p, err := c.Post(ctx, 16)
	if err != nil {
		t.Fatal(err)
	}
	if q := f.last(t).Query; q.Get("id") != "16" || len(q) != 1 {
		t.Errorf("the post request sent %v", q)
	}
	if p.ID != 16 || p.Name != "Walnut prairie compass" || p.Comments != 3 || p.CommunityName != "testing" {
		t.Errorf("got %+v", p)
	}

	comments, err := c.Comments(ctx, 16, 8)
	if err != nil {
		t.Fatal(err)
	}
	q := f.last(t).Query
	if q.Get("post_id") != "16" || q.Get("max_depth") != "8" || q.Get("sort") != "Old" ||
		q.Get("type_") != "All" || len(q) != 4 {
		t.Errorf("the comment request sent %v", q)
	}
	sameComments(t, comments, []Comment{
		{ID: 34, Path: "0.34", Content: "Juniper basalt orchard", CreatorID: 3, CreatorName: "vera",
			Published: at("2026-10-27T01:31:11.518413Z")},
		{ID: 36, Path: "0.36", Content: "Copper lagoon sparrow", CreatorID: 3, CreatorName: "vera",
			Published: at("2026-10-27T01:31:11.56935Z")},
		{ID: 35, Path: "0.34.35", Content: "Marble kestrel thistle", CreatorID: 3, CreatorName: "vera",
			Published: at("2026-10-27T01:31:11.5464Z")},
	})
}

func TestV3CommunitiesAreMapped(t *testing.T) {
	f, sysURL := newFakeServer(t, V3, map[string][]reply{routeV3Communities: ok(fixture(t, "v3/communities.json"))})
	c := newTestClient(t, sysURL, "")

	communities, err := c.Communities(context.Background(), ListingLocal, 50, 10)
	if err != nil {
		t.Fatal(err)
	}

	requests := f.all()
	if len(requests) != 1 {
		t.Fatalf("a short first page made %d requests", len(requests))
	}
	q := requests[0].Query
	if q.Get("type_") != "Local" || q.Get("sort") != "TopAll" || q.Get("limit") != "50" || q.Get("page") != "1" {
		t.Errorf("sent %v", q)
	}

	want := []Community{
		{ID: 2, Name: "general", Title: "General",
			Description: "## Rules\n\nThe general community of this instance.\n\n- Be kind"},
		{ID: 3, Name: "testing", Title: "Testing",
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

func communityPage(ids ...int) string {
	items := make([]string, 0, len(ids))
	for _, id := range ids {
		items = append(items, fmt.Sprintf(`{"community":{"id":%d,"name":"c%d","title":"C%d"}}`, id, id, id))
	}
	return `{"communities":[` + strings.Join(items, ",") + `]}`
}

func TestV3CommunitiesPageUntilAShortPage(t *testing.T) {
	full := make([]reply, 0, 12)
	for page := range 12 {
		full = append(full, reply{http.StatusOK, communityPage(2*page+1, 2*page+2)})
	}
	f, sysURL := newFakeServer(t, V3, map[string][]reply{routeV3Communities: full})
	c := newTestClient(t, sysURL, "")

	communities, err := c.Communities(context.Background(), ListingAll, 2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(communities) != 20 || communities[19].ID != 20 {
		t.Errorf("ten full pages gave %d communities", len(communities))
	}
	requests := f.all()
	if len(requests) != 10 || requests[9].Query.Get("page") != "10" {
		t.Errorf("made %d requests", len(requests))
	}

	f, sysURL = newFakeServer(t, V3, map[string][]reply{routeV3Communities: {
		{http.StatusOK, communityPage(1, 2)},
		{http.StatusOK, communityPage(3, 4)},
		{http.StatusOK, communityPage(5)},
	}})
	c = newTestClient(t, sysURL, "")

	communities, err = c.Communities(context.Background(), ListingAll, 2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(communities) != 5 || len(f.all()) != 3 {
		t.Errorf("a short third page gave %d communities after %d requests", len(communities), len(f.all()))
	}
}

func TestV3LoginSendsTheCodeOnlyWhenThereIsOne(t *testing.T) {
	f, sysURL := newFakeServer(t, V3, map[string][]reply{routeV3Login: ok(v3LoginOK)})
	c := newTestClient(t, sysURL, "")
	ctx := context.Background()

	jwt, err := c.Login(ctx, Login{Username: "vera", Password: "secret"})
	if err != nil || jwt != "jwt-1" {
		t.Fatalf("got %q and %v", jwt, err)
	}
	body := f.last(t).Body
	if body["username_or_email"] != "vera" || body["password"] != "secret" || body["totp_2fa_token"] != nil {
		t.Errorf("sent %v", body)
	}

	if _, err := c.Login(ctx, Login{Username: "vera", Password: "secret", Code: "123456"}); err != nil {
		t.Fatal(err)
	}
	if body := f.last(t).Body; body["totp_2fa_token"] != "123456" {
		t.Errorf("sent %v", body)
	}
}

func TestV3LoginWithoutATokenFails(t *testing.T) {
	_, sysURL := newFakeServer(t, V3, map[string][]reply{
		routeV3Login: ok(`{"jwt":null,"registration_created":false,"verify_email_sent":true}`),
	})
	c := newTestClient(t, sysURL, "")

	if _, err := c.Login(context.Background(), Login{Username: "vera", Password: "secret"}); !errors.Is(err, ErrNoToken) {
		t.Errorf("got %v, want ErrNoToken", err)
	}
}

func TestV3CreatesSendTheirFields(t *testing.T) {
	f, sysURL := newFakeServer(t, V3, map[string][]reply{
		routeV3CreatePost:    ok(fixture(t, "v3/create_post.json")),
		routeV3CreateComment: ok(fixture(t, "v3/create_comment.json")),
	})
	c := newTestClient(t, sysURL, "session-token")
	ctx := context.Background()

	id, err := c.CreatePost(ctx, NewPost{CommunityID: 3, Name: "Cobalt ridge almanac", URL: "https://example.com/"})
	if err != nil || id != 17 {
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

	id, err = c.CreateComment(ctx, NewComment{PostID: 16, ParentID: 34, Content: "Marble kestrel thistle"})
	if err != nil || id != 35 {
		t.Fatalf("got %d and %v", id, err)
	}
	body = f.last(t).Body
	if body["post_id"] != float64(16) || body["parent_id"] != float64(34) || body["content"] != "Marble kestrel thistle" {
		t.Errorf("the reply sent %v", body)
	}

	if _, err := c.CreateComment(ctx, NewComment{PostID: 16, Content: "Juniper"}); err != nil {
		t.Fatal(err)
	}
	if body := f.last(t).Body; body["parent_id"] != nil {
		t.Errorf("the top-level comment sent %v", body)
	}
}
