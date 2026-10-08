package feed

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/prompt"
)

type fake struct {
	forums      []forum.Forum
	ordering    system.Ordering
	lists       map[system.Order][]post.Post
	unavailable map[system.Order]bool
	err         error
	requested   []system.Order
	calls       int
}

func (f *fake) Kind() string                      { return "fake" }
func (f *fake) URL() string                       { return "" }
func (f *fake) Title() string                     { return "fake" }
func (f *fake) Description() string               { return "fake" }
func (f *fake) Capabilities() system.Capabilities { return system.CapRead }
func (f *fake) Connect(context.Context, prompt.Prompter, string) (system.Settings, error) {
	return system.Settings{}, nil
}
func (f *fake) ListForums(context.Context) ([]forum.Forum, error) {
	f.calls++
	return f.forums, f.err
}

func (f *fake) Orders(string) system.Ordering {
	ordering := f.ordering
	if ordering.Default == "" {
		ordering = system.Only(system.OrderNew)
	}
	for order := range f.unavailable {
		ordering = ordering.Without(order)
	}
	return ordering
}

func (f *fake) ListPosts(_ context.Context, _ string, order system.Order) ([]post.Post, error) {
	f.calls++
	f.requested = append(f.requested, order)
	if f.unavailable[order] {
		return nil, system.ErrOrderUnavailable
	}
	return f.lists[order], f.err
}
func (f *fake) LoadPost(context.Context, *post.Post) error      { return nil }
func (f *fake) CreatePost(context.Context, *post.Post) error    { return nil }
func (f *fake) CreateReply(context.Context, *reply.Reply) error { return nil }

func at(h int) time.Time {
	return time.Date(2026, 10, 5, h, 0, 0, 0, time.UTC)
}

func ids(posts []post.Post) string {
	out := make([]string, 0, len(posts))
	for _, p := range posts {
		out = append(out, p.ID)
	}
	return strings.Join(out, ",")
}

var (
	everything = system.Ordering{Default: system.OrderNew, Supported: system.AllOrders()}
	lobsters   = system.Ordering{
		Default:   system.OrderNew,
		Supported: []system.Order{system.OrderNew, system.OrderActive, system.OrderHot},
	}
	hackerNews = system.Ordering{
		Default:   system.OrderNew,
		Supported: append([]system.Order{system.OrderNew, system.OrderHot}, system.TopOrders()...),
	}
	hyperuplink = system.Only(system.OrderActive)
)

func TestChooseUsesTheOrderOrItsFallback(t *testing.T) {
	for _, tc := range []struct {
		name     string
		ordering system.Ordering
		want     system.Order
		listed   system.Order
	}{
		{"supported", lobsters, system.OrderHot, system.OrderHot},
		{"no top", lobsters, system.OrderTopWeek, system.OrderNew},
		{"comments from active", lobsters, system.OrderComments, system.OrderActive},
		{"comments from hot", hackerNews, system.OrderComments, system.OrderHot},
		{"comments from the only list", hyperuplink, system.OrderComments, system.OrderActive},
		{"new from the only list", hyperuplink, system.OrderNew, system.OrderActive},
		{"native comments", everything, system.OrderComments, system.OrderComments},
		{"tag pages", system.Only(system.OrderNew), system.OrderComments, system.OrderNew},
	} {
		if got := Choose(tc.ordering, tc.want); got != tc.listed {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.listed)
		}
	}
}

func TestListRequestsTheChosenOrder(t *testing.T) {
	f := &fake{ordering: lobsters, lists: map[system.Order][]post.Post{
		system.OrderActive: {{ID: "a1"}},
	}}

	res, err := feedWith(3, f).List(context.Background(), 3, "", system.OrderComments)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if res.System != 3 || res.Order != system.OrderActive || ids(res.Posts) != "a1" {
		t.Errorf("got %+v", res)
	}
	if !slices.Equal(f.requested, []system.Order{system.OrderActive}) {
		t.Errorf("requested %v", f.requested)
	}
}

func TestListFallsBackWhenAnOrderTurnsOutMissing(t *testing.T) {
	f := &fake{
		ordering:    everything,
		unavailable: map[system.Order]bool{system.OrderHot: true},
		lists:       map[system.Order][]post.Post{system.OrderNew: {{ID: "n1"}}},
	}

	res, err := feedWith(0, &onceUnavailable{fake: f}).List(context.Background(), 0, "", system.OrderHot)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if res.Order != system.OrderNew || ids(res.Posts) != "n1" {
		t.Errorf("got %+v", res)
	}
	if !slices.Equal(f.requested, []system.Order{system.OrderHot, system.OrderNew}) {
		t.Errorf("requested %v", f.requested)
	}
}

type onceUnavailable struct {
	*fake
	failed bool
}

func (o *onceUnavailable) Orders(forumID string) system.Ordering {
	if !o.failed {
		return everything
	}
	return o.fake.Orders(forumID)
}

func (o *onceUnavailable) ListPosts(ctx context.Context, forumID string, order system.Order) ([]post.Post, error) {
	posts, err := o.fake.ListPosts(ctx, forumID, order)
	if errors.Is(err, system.ErrOrderUnavailable) {
		o.failed = true
	}
	return posts, err
}

func TestListFallsBackToTheDefaultWhenTheSystemKeepsOffering(t *testing.T) {
	f := &fake{
		ordering: everything,
		lists:    map[system.Order][]post.Post{system.OrderNew: {{ID: "n1"}}},
	}
	stubborn := &stubbornUnavailable{fake: f}

	res, err := feedWith(0, stubborn).List(context.Background(), 0, "", system.OrderHot)
	if err != nil || res.Order != system.OrderNew {
		t.Errorf("got %+v, %v", res, err)
	}
}

type stubbornUnavailable struct{ *fake }

func (s *stubbornUnavailable) ListPosts(ctx context.Context, forumID string, order system.Order) ([]post.Post, error) {
	if order == system.OrderHot {
		s.requested = append(s.requested, order)
		return nil, system.ErrOrderUnavailable
	}
	return s.fake.ListPosts(ctx, forumID, order)
}

func TestListPassesOtherErrorsOn(t *testing.T) {
	f := &fake{err: errors.New("down")}
	if _, err := feedWith(0, f).List(context.Background(), 0, "", system.OrderNew); err == nil {
		t.Error("expected the error")
	}
}

func TestMergeNewSortsByCreationTime(t *testing.T) {
	results := []Result{
		{System: 1, Order: system.OrderNew, Posts: []post.Post{{ID: "b2", CreatedAt: at(2)}}},
		{System: 0, Order: system.OrderNew, Posts: []post.Post{
			{ID: "a3", CreatedAt: at(3)}, {ID: "a1", CreatedAt: at(1)}}},
		{System: 2, Order: system.OrderActive, Posts: []post.Post{
			{ID: "h-old-but-active", CreatedAt: at(0)}, {ID: "h4", CreatedAt: at(4)}}},
	}

	if got := ids(Merge(results, system.OrderNew)); got != "h4,a3,b2,a1,h-old-but-active" {
		t.Errorf("got %s", got)
	}
}

func TestMergeInterleavesByRank(t *testing.T) {
	long := make([]post.Post, 4)
	for i := range long {
		long[i] = post.Post{ID: "L" + string(rune('1'+i))}
	}
	results := []Result{
		{System: 0, Order: system.OrderHot, Posts: long},
		{System: 1, Order: system.OrderHot, Posts: []post.Post{{ID: "S1"}, {ID: "S2"}}},
		{System: 2, Order: system.OrderNew, Posts: []post.Post{{ID: "F1"}}},
	}

	got := ids(Merge(results, system.OrderHot))
	if got != "L1,S1,F1,L2,L3,S2,L4" {
		t.Errorf("got %s", got)
	}
}

func TestMergeSortsFallbackListsByRepliesForMostComments(t *testing.T) {
	results := []Result{
		{System: 0, Order: system.OrderComments, Posts: []post.Post{
			{ID: "lemmy-2000", ReplyCount: 2000}, {ID: "lemmy-1500", ReplyCount: 1500}}},
		{System: 1, Order: system.OrderHot, Posts: []post.Post{
			{ID: "hn-12", ReplyCount: 12}, {ID: "hn-501", ReplyCount: 501}}},
	}

	got := ids(Merge(results, system.OrderComments))
	if got != "lemmy-2000,hn-501,lemmy-1500,hn-12" {
		t.Errorf("got %s", got)
	}
	if results[1].Posts[0].ID != "hn-12" {
		t.Error("Merge must not reorder the caller's lists")
	}
}

func TestMergeKeepsASingleListInItsOrder(t *testing.T) {
	results := []Result{{System: 0, Order: system.OrderHot, Posts: []post.Post{
		{ID: "x", CreatedAt: at(1)}, {ID: "y", CreatedAt: at(9)}}}}
	if got := ids(Merge(results, system.OrderHot)); got != "x,y" {
		t.Errorf("got %s", got)
	}
}

func TestStatusNamesSystemsThatDontUseTheOrder(t *testing.T) {
	for _, tc := range []struct {
		want system.Order
		uses []Use
		text string
	}{
		{system.OrderTopWeek, []Use{
			{"lemmy.ml", system.OrderTopWeek},
			{"lobste.rs", system.OrderNew},
			{"hup.example", system.OrderActive},
		}, "lobste.rs keeps New, hup.example keeps Active"},
		{system.OrderComments, []Use{
			{"lemmy.ml", system.OrderComments},
			{"lobste.rs", system.OrderActive},
			{"news.ycombinator.com", system.OrderHot},
			{"hup.example", system.OrderActive},
		}, "lobste.rs and hup.example sort their active posts by replies, " +
			"news.ycombinator.com its hot posts"},
		{system.OrderNew, []Use{
			{"lemmy.ml", system.OrderNew},
			{"hup.example", system.OrderActive},
		}, "hup.example sorts its active posts by date"},
		{system.OrderTopDay, []Use{
			{"a", system.OrderNew}, {"b", system.OrderNew}, {"c", system.OrderNew},
		}, "a, b and c keep New"},
		{system.OrderHot, []Use{{"lemmy.ml", system.OrderHot}}, ""},
	} {
		if got := Status(tc.want, tc.uses); got != tc.text {
			t.Errorf("%s: got %q, want %q", tc.want, got, tc.text)
		}
	}
}

func TestListForumsSortsByTitle(t *testing.T) {
	a := &fake{forums: []forum.Forum{{ID: "z", Name: "zebra"}}}
	b := &fake{forums: []forum.Forum{{ID: "m", Name: "mango"}, {ID: "a", Name: "apple"}}}

	forums, _ := New([]system.System{a, b}, 0, 0).ListForums(context.Background(), All)
	if len(forums) != 3 || forums[0].Name != "apple" || forums[2].Name != "zebra" {
		t.Fatalf("unexpected order: %+v", forums)
	}
}

func TestSelected(t *testing.T) {
	systems := []system.System{&fake{}, &fake{}, &fake{}}
	if got := Selected(systems, All); !slices.Equal(got, []int{0, 1, 2}) {
		t.Errorf("All: %v", got)
	}
	if got := Selected(systems, 1); !slices.Equal(got, []int{1}) {
		t.Errorf("one: %v", got)
	}
}

func feedWith(idx int, sys system.System) *Feed {
	systems := make([]system.System, idx+1)
	systems[idx] = sys
	return New(systems, 0, 0)
}
