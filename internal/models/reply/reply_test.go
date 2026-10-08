package reply

import (
	"testing"
)

func TestTreeNestsByParent(t *testing.T) {
	flat := []Reply{
		{ID: "a"},
		{ID: "b", ParentID: "a"},
		{ID: "c", ParentID: "b"},
		{ID: "d"},
		{ID: "e", ParentID: "a"},
	}

	tree := Tree(flat)
	if len(tree) != 2 || tree[0].ID != "a" || tree[1].ID != "d" {
		t.Fatalf("roots: %+v", tree)
	}
	a := tree[0]
	if len(a.Replies) != 2 || a.Replies[0].ID != "b" || a.Replies[1].ID != "e" {
		t.Fatalf("children of a: %+v", a.Replies)
	}
	if len(a.Replies[0].Replies) != 1 || a.Replies[0].Replies[0].ID != "c" {
		t.Fatalf("children of b: %+v", a.Replies[0].Replies)
	}
	if a.Replies[0].Replies[0].ParentID != "b" {
		t.Error("a nested reply keeps its parent id")
	}
	if Count(tree) != 5 {
		t.Errorf("counted %d", Count(tree))
	}
}

func TestTreeMakesOrphansRoots(t *testing.T) {
	tree := Tree([]Reply{
		{ID: "x", ParentID: "missing"},
		{ID: "y", ParentID: "y"},
	})
	if len(tree) != 2 || tree[0].ParentID != "" || tree[1].ParentID != "" {
		t.Errorf("got %+v", tree)
	}
}

func TestTreeOfNothing(t *testing.T) {
	if tree := Tree(nil); len(tree) != 0 {
		t.Errorf("got %+v", tree)
	}
}
