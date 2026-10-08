package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	serve := func(path string, name string) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Write(fixture(t, name))
		})
	}
	serve("/newest.json", "newest.json")
	serve("/t/programming.json", "newest.json")
	serve("/s/dldhpw.json", "story.json")
	serve("/tags.json", "tags.json")

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, err := NewClient(httpx.NewHTTPClient(httpx.Options{}), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestListDecodesCurrentSchema(t *testing.T) {
	c := testClient(t)

	stories, err := c.Stories(context.Background(), "newest")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(stories) == 0 {
		t.Fatal("no stories decoded")
	}
	s := stories[0]
	if s.ShortID == "" || s.Title == "" || s.SubmitterUser == "" {
		t.Errorf("story fields missing: %+v", s)
	}
	if len(s.Tags) == 0 {
		t.Errorf("story has no tags: %+v", s)
	}

	tagged, err := c.Tagged(context.Background(), "programming")
	if err != nil || len(tagged) == 0 {
		t.Fatalf("tag list failed: %v", err)
	}
}

func TestShowDecodesComments(t *testing.T) {
	c := testClient(t)

	story, err := c.Story(context.Background(), "dldhpw")
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if story.ShortID != "dldhpw" || story.SubmitterUser == "" {
		t.Errorf("story not decoded: %+v", story)
	}
	if len(story.Comments) == 0 {
		t.Fatal("no comments decoded")
	}

	nested := 0
	for _, cm := range story.Comments {
		if cm.ShortID == "" || cm.CommentingUser == "" || cm.Comment == "" {
			t.Errorf("comment fields missing: %+v", cm)
		}
		if cm.ParentComment != "" {
			nested++
			if cm.Depth == 0 {
				t.Errorf("comment %s has a parent but depth 0", cm.ShortID)
			}
		}
	}
	if nested == 0 {
		t.Error("expected at least one nested comment in the fixture")
	}
}

func TestTagsList(t *testing.T) {
	c := testClient(t)

	tags, err := c.Tags(context.Background())
	if err != nil {
		t.Fatalf("Tags: %v", err)
	}
	if len(tags) == 0 || tags[0].Tag == "" {
		t.Fatalf("tags not decoded: %+v", tags)
	}
}
