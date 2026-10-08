package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrusme/neonmodem/internal/system/httpx"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func testClient(t *testing.T) *Client {
	t.Helper()

	mux := http.NewServeMux()
	serveJSON := func(path string, body func() []byte) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Write(body())
		})
	}
	serveJSON("/firebase/topstories.json", func() []byte { return []byte(`[49953495, 49953496, 1]`) })
	serveJSON("/firebase/item/49953495.json", func() []byte { return fixture(t, "item_story.json") })
	serveJSON("/firebase/item/49953496.json", func() []byte { return fixture(t, "item_comment.json") })
	serveJSON("/firebase/item/1.json", func() []byte { return []byte("null") })
	serveJSON("/algolia/items/49953495", func() []byte { return fixture(t, "algolia_item.json") })
	mux.HandleFunc("/algolia/items/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"Item not found"}`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, err := NewClientWithBases(httpx.NewHTTPClient(httpx.Options{}), srv.URL+"/firebase", srv.URL+"/algolia")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestStories(t *testing.T) {
	c := testClient(t)

	ids, err := c.Stories(context.Background(), "top")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 || ids[0] != 49953495 {
		t.Fatalf("unexpected ids %v", ids)
	}

	if _, err := c.Stories(context.Background(), "nope"); err == nil {
		t.Fatal("an unknown list must be rejected")
	}
}

func TestItemNotFound(t *testing.T) {
	c := testClient(t)

	if _, err := c.Item(context.Background(), 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for a null item, got %v", err)
	}
}

func TestItemsKeepsOrderAndSkipsMissing(t *testing.T) {
	c := testClient(t)

	items, err := c.Items(context.Background(), []int{49953496, 1, 49953495})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != 49953496 || items[1].ID != 49953495 {
		t.Fatalf("unexpected items: %+v", items)
	}
	if items[1].Type != "story" || items[1].Descendants == 0 || items[1].Title == "" {
		t.Errorf("story fields not decoded: %+v", items[1])
	}
	if items[0].Type != "comment" || items[0].Parent != 49953495 {
		t.Errorf("comment fields not decoded: %+v", items[0])
	}
}

func TestItemsAllMissingIsAnError(t *testing.T) {
	c := testClient(t)

	if _, err := c.Items(context.Background(), []int{1}); err == nil {
		t.Fatal("expected an error when no item could be fetched")
	}
}

func TestTree(t *testing.T) {
	c := testClient(t)

	tree, err := c.Tree(context.Background(), 49953495)
	if err != nil {
		t.Fatal(err)
	}
	if tree.ID != 49953495 || tree.Title == nil || !strings.Contains(*tree.Title, "Qwen") {
		t.Fatalf("unexpected root: %+v", tree)
	}
	if len(tree.Children) != 3 {
		t.Fatalf("expected the trimmed fixture's 3 children, got %d", len(tree.Children))
	}
	child := tree.Children[0]
	if child.ParentID == nil || *child.ParentID != 49953495 || child.AuthorName() == "" || child.Body() == "" {
		t.Errorf("child not decoded: %+v", child)
	}
	if child.IsDeleted() {
		t.Error("a comment with author and text is not deleted")
	}

	deleted := AlgoliaItem{Type: "comment"}
	if !deleted.IsDeleted() {
		t.Error("a comment without author must count as deleted")
	}

	if _, err := c.Tree(context.Background(), 1); httpx.StatusOf(err) != http.StatusNotFound {
		t.Fatalf("expected a 404 error, got %v", err)
	}
}
