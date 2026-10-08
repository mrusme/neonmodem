package system

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrusme/neonmodem/internal/models/post"
)

func TestParseOrder(t *testing.T) {
	for _, o := range AllOrders() {
		got, err := ParseOrder(" " + strings.ToUpper(string(o)) + " ")
		if err != nil || got != o {
			t.Errorf("ParseOrder(%q) = %q, %v", o, got, err)
		}
		if o.Label() == string(o) || o.Info() == "" {
			t.Errorf("%q has no label or description", o)
		}
	}

	got, err := ParseOrder("best")
	if err == nil || got != OrderNew || !strings.Contains(err.Error(), "top-week") {
		t.Errorf("an unknown order should fall back to new and list the keys: %q, %v", got, err)
	}
}

func TestOrderingResolvesAndFallsBack(t *testing.T) {
	o := Ordering{Default: OrderNew, Supported: []Order{OrderNew, OrderActive, OrderHot}}

	if o.Resolve(OrderHot) != OrderHot || o.Resolve(OrderTopWeek) != OrderNew {
		t.Error("Resolve should keep supported orders and fall back to the default")
	}
	if o.First(OrderComments.Sources()) != OrderActive {
		t.Error("First should pick the first supported source")
	}
	if Only(OrderActive).First(OrderComments.Sources()) != OrderActive {
		t.Error("a system whose only order is Active should use it as the source")
	}
	if (Ordering{Default: OrderNew, Supported: []Order{OrderNew}}).First(OrderComments.Sources()) != OrderNew {
		t.Error("without a source, First should return the default")
	}

	without := o.Without(OrderHot)
	if without.Supports(OrderHot) || !o.Supports(OrderHot) {
		t.Error("Without should drop the order from a copy only")
	}
	if !o.Without(OrderNew).Supports(OrderNew) {
		t.Error("Without must never drop the default")
	}
}

func TestCompareSortsByCreationOrReplies(t *testing.T) {
	now := time.Now()
	posts := []post.Post{
		{ID: "old-busy", CreatedAt: now.Add(-3 * time.Hour), ReplyCount: 50},
		{ID: "new-quiet", CreatedAt: now, ReplyCount: 1},
		{ID: "mid-busy", CreatedAt: now.Add(-time.Hour), ReplyCount: 50},
	}

	byNew := slices.Clone(posts)
	slices.SortStableFunc(byNew, OrderNew.Compare())
	if ids(byNew) != "new-quiet,mid-busy,old-busy" {
		t.Errorf("New sorted as %s", ids(byNew))
	}

	byReplies := slices.Clone(posts)
	slices.SortStableFunc(byReplies, OrderComments.Compare())
	if ids(byReplies) != "mid-busy,old-busy,new-quiet" {
		t.Errorf("Most comments sorted as %s", ids(byReplies))
	}

	for _, o := range []Order{OrderActive, OrderHot, OrderTopWeek} {
		if o.Compare() != nil || o.Sources() != nil {
			t.Errorf("%q must not be sorted locally", o)
		}
	}
}

func ids(posts []post.Post) string {
	out := make([]string, 0, len(posts))
	for _, p := range posts {
		out = append(out, p.ID)
	}
	return strings.Join(out, ",")
}
