package api

import (
	"context"
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

func testServer(t *testing.T, seen *http.Header) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	serve := func(path string, name string) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			if seen != nil {
				*seen = r.Header.Clone()
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Write(fixture(t, name))
		})
	}
	serve("/categories.json", "categories.json")
	serve("/latest.json", "latest.json")
	serve("/t/9.json", "topic_long.json")
	serve("/t/9/posts.json", "topic_posts.json")
	mux.HandleFunc("/forbidden.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"errors":["You are not permitted to view the requested resource."],"error_type":"invalid_access"}`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestAnonymousClientSendsNoKeyHeaders(t *testing.T) {
	var seen http.Header
	srv := testServer(t, &seen)

	c, err := NewClient(httpx.NewHTTPClient(httpx.Options{}), srv.URL, Credentials{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Categories(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := seen["User-Api-Key"]; ok {
		t.Error("anonymous client sent a User-Api-Key header")
	}
	if _, ok := seen["User-Api-Client-Id"]; ok {
		t.Error("anonymous client sent a User-Api-Client-Id header")
	}
	if seen.Get("User-Agent") != httpx.DefaultUserAgent {
		t.Errorf("user agent %q", seen.Get("User-Agent"))
	}
}

func TestAuthenticatedClientSendsKeyHeaders(t *testing.T) {
	var seen http.Header
	srv := testServer(t, &seen)

	c, err := NewClient(httpx.NewHTTPClient(httpx.Options{}), srv.URL, Credentials{ClientID: "cid", Key: "k"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Categories(context.Background()); err != nil {
		t.Fatal(err)
	}
	if seen.Get("User-Api-Key") != "k" || seen.Get("User-Api-Client-Id") != "cid" {
		t.Errorf("key headers missing: %v", seen)
	}
}

func TestErrorBodyIsDecoded(t *testing.T) {
	srv := testServer(t, nil)
	c, _ := NewClient(httpx.NewHTTPClient(httpx.Options{}), srv.URL, Credentials{})

	err := c.http.Get(context.Background(), "/forbidden.json", nil, &map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "not permitted") {
		t.Fatalf("expected the Discourse error message, got %v", err)
	}
	if httpx.StatusOf(err) != http.StatusForbidden {
		t.Errorf("status not carried: %v", err)
	}
}

func TestCategoriesAndTopicsDecode(t *testing.T) {
	srv := testServer(t, nil)
	c, _ := NewClient(httpx.NewHTTPClient(httpx.Options{}), srv.URL, Credentials{})

	cats, err := c.Categories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var testing_ *CategoryModel
	for i := range cats.CategoryList.Categories {
		if cats.CategoryList.Categories[i].Name == "Testing" {
			testing_ = &cats.CategoryList.Categories[i]
		}
	}
	if testing_ == nil {
		t.Fatal("Testing category missing from the fixture")
	}
	if len(testing_.SubcategoryList) != 1 || testing_.SubcategoryList[0].Name != "Nested" {
		t.Errorf("subcategory list not decoded: %+v", testing_.SubcategoryList)
	}

	topic, err := c.Topic(context.Background(), "9")
	if err != nil {
		t.Fatal(err)
	}
	if topic.ChunkSize != 20 || len(topic.PostStream.Stream) != 25 || len(topic.PostStream.Posts) != 20 {
		t.Errorf("topic paging fields: chunk=%d stream=%d posts=%d",
			topic.ChunkSize, len(topic.PostStream.Stream), len(topic.PostStream.Posts))
	}
	if topic.PostStream.Posts[0].PostNumber != 1 || topic.PostStream.Posts[0].Cooked == "" {
		t.Errorf("first post not decoded: %+v", topic.PostStream.Posts[0])
	}

	window, err := c.TopicPosts(context.Background(), "9", []int{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(window.PostStream.Posts) == 0 {
		t.Error("posts window empty")
	}

	latest, err := c.Topics(context.Background(), "latest", "", 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(latest.TopicList.Topics) == 0 || len(latest.Users) == 0 {
		t.Errorf("latest not decoded: topics=%d users=%d", len(latest.TopicList.Topics), len(latest.Users))
	}
}
