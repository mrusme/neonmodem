package lemmy

import (
	"errors"
	"testing"
	"time"

	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
	"go.elara.ws/go-lemmy"
)

func view(id int64, path string, content string, deleted bool) lemmy.CommentView {
	return lemmy.CommentView{
		Comment: lemmy.Comment{
			ID:        id,
			Path:      path,
			Content:   content,
			Deleted:   deleted,
			Published: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC),
		},
		Creator: lemmy.Person{Name: "u" + path},
	}
}

func TestBuildTreeThreadsByPath(t *testing.T) {
	sys := &System{idx: 3}

	views := []lemmy.CommentView{
		view(1, "0.1", "top one", false),
		view(2, "0.1.2", "child of one", false),
		view(3, "0.3", "top three", true),
		view(4, "0.1.2.4", "grandchild", false),
		view(5, "0.9.5", "orphan whose parent is not loaded", false),
	}

	tree := sys.buildTree("42", views)

	if len(tree) != 3 {
		t.Fatalf("expected 3 top-level replies (two roots and one orphan), got %d", len(tree))
	}
	if reply.Count(tree) != 5 {
		t.Fatalf("expected all 5 comments in the tree, got %d", reply.Count(tree))
	}

	one := tree[0]
	if one.ID != "1" || one.ParentID != "" || one.PostID != "42" || one.SysIDX != 3 {
		t.Errorf("unexpected root: %+v", one)
	}
	if len(one.Replies) != 1 || one.Replies[0].ID != "2" || one.Replies[0].ParentID != "1" {
		t.Fatalf("child not attached: %+v", one.Replies)
	}
	if got := one.Replies[0].Replies; len(got) != 1 || got[0].ID != "4" || got[0].ParentID != "2" {
		t.Errorf("grandchild not attached: %+v", got)
	}
	if !tree[1].Deleted {
		t.Error("deleted comment not flagged")
	}
	if tree[2].ID != "5" || tree[2].ParentID != "" {
		t.Errorf("orphan should become a root without parent: %+v", tree[2])
	}
}

func TestListingTypeFallsBackForAnonymous(t *testing.T) {
	anon := &System{settings: system.Settings{}}
	if got := anon.listingType(); got != lemmy.ListingTypeLocal {
		t.Errorf("anonymous subscribed should fall back to local, got %s", got)
	}

	account := &System{settings: system.Settings{
		Credentials: map[string]string{"username": "u", "token": "t"},
	}}
	if got := account.listingType(); got != lemmy.ListingTypeSubscribed {
		t.Errorf("default with an account should be subscribed, got %s", got)
	}

	all := &System{settings: system.Settings{Options: map[string]string{OptionListing: "ALL"}}}
	if got := all.listingType(); got != lemmy.ListingTypeAll {
		t.Errorf("option all should give All, got %s", got)
	}
}

func TestIsAuthError(t *testing.T) {
	if !isAuthError(lemmy.Error{Code: 401}) {
		t.Error("401 should be an auth error")
	}
	if !isAuthError(lemmy.Error{ErrStr: "not_logged_in", Code: 400}) {
		t.Error("not_logged_in should be an auth error")
	}
	if isAuthError(lemmy.Error{ErrStr: "couldnt_find_post", Code: 400}) {
		t.Error("a missing post is not an auth error")
	}
	if isAuthError(errors.New("network down")) {
		t.Error("a plain error is not an auth error")
	}
}

func TestCapabilitiesFollowCredentials(t *testing.T) {
	anon := &System{}
	if anon.Capabilities().Has(system.CapCreatePost) {
		t.Error("anonymous lemmy must not report write capabilities")
	}
	account := &System{settings: system.Settings{
		Credentials: map[string]string{"username": "u", "token": "t"},
	}}
	if !account.Capabilities().Has(system.CapWrite) {
		t.Error("an account must report write capabilities")
	}
}

func TestLinkPostsCarryTheirLink(t *testing.T) {
	sys := &System{settings: system.Settings{URL: "https://lemmy.example"}}

	link := sys.toPost(lemmy.PostView{Post: lemmy.Post{ID: 7, Name: "Linux 7.3",
		URL: lemmy.NewOptional("https://kernel.org/")}})
	if link.Link != "https://kernel.org/" || link.URL != "https://lemmy.example/post/7" {
		t.Errorf("link post: link=%q url=%q", link.Link, link.URL)
	}

	text := sys.toPost(lemmy.PostView{Post: lemmy.Post{ID: 8, Name: "A question",
		Body: lemmy.NewOptional("Why?")}})
	if text.Link != "" {
		t.Errorf("a text post has no link, got %q", text.Link)
	}
}
