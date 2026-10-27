package lemmy

import (
	"testing"
	"time"

	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/lemmy/api"
)

func comment(id int64, path string, content string, deleted bool) api.Comment {
	return api.Comment{
		ID:          id,
		Path:        path,
		Content:     content,
		Deleted:     deleted,
		Published:   time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC),
		CreatorName: "u" + path,
	}
}

func TestBuildTreeThreadsByPath(t *testing.T) {
	sys := &System{idx: 3}

	comments := []api.Comment{
		comment(1, "0.1", "top one", false),
		comment(2, "0.1.2", "child of one", false),
		comment(3, "0.3", "top three", true),
		comment(4, "0.1.2.4", "grandchild", false),
		comment(5, "0.9.5", "orphan whose parent is not loaded", false),
	}

	tree := sys.buildTree("42", comments)

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
	if got := anon.listingType(); got != api.ListingLocal {
		t.Errorf("anonymous subscribed should fall back to local, got %s", got)
	}

	account := &System{settings: system.Settings{
		Credentials: map[string]string{"username": "u", "token": "t"},
	}}
	if got := account.listingType(); got != api.ListingSubscribed {
		t.Errorf("default with an account should be subscribed, got %s", got)
	}

	all := &System{settings: system.Settings{Options: map[string]string{OptionListing: "ALL"}}}
	if got := all.listingType(); got != api.ListingAll {
		t.Errorf("option all should give All, got %s", got)
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

	link := sys.toPost(api.Post{ID: 7, Name: "Linux 7.3", URL: "https://kernel.org/"})
	if link.Link != "https://kernel.org/" || link.URL != "https://lemmy.example/post/7" {
		t.Errorf("link post: link=%q url=%q", link.Link, link.URL)
	}

	text := sys.toPost(api.Post{ID: 8, Name: "A question", Body: "Why?"})
	if text.Link != "" {
		t.Errorf("a text post has no link, got %q", text.Link)
	}
}

func TestNewReadsMarkAsRead(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
		err   string
	}{
		{"", false, ""},
		{"no", false, ""},
		{"yes", true, ""},
		{" Yes ", true, ""},
		{"maybe", false, `the option mark_as_read is "maybe"; use yes or no`},
		{"true", false, `the option mark_as_read is "true"; use yes or no`},
	} {
		settings := system.Settings{URL: "https://lemmy.example"}
		if tc.value != "" {
			settings.Options = map[string]string{OptionMarkAsRead: tc.value}
		}

		sys, err := New(system.Env{Settings: settings})
		if tc.err != "" {
			if err == nil || err.Error() != tc.err {
				t.Errorf("%q: got %v, want %q", tc.value, err, tc.err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: %v", tc.value, err)
		}
		if got := sys.(*System).markAsRead; got != tc.want {
			t.Errorf("%q: got %v, want %v", tc.value, got, tc.want)
		}
	}
}
