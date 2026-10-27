package lemmy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrusme/neonmodem/internal/system"
)

const nodeinfoPath = "/nodeinfo/2.1"

type flavor struct {
	name string
	dir  string

	nodeinfo    string
	validate    string
	forums      string
	posts       string
	post        string
	comments    string
	createPost  string
	createReply string
	login       string

	sessionOK   string
	notLoggedIn string
	rateLimited route

	sorts      map[system.Order]string
	timeRanges map[system.Order]string
}

func fixture(t *testing.T, dir string, name string) string {
	t.Helper()

	b, err := os.ReadFile(filepath.Join("api", "testdata", dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func flavors(t *testing.T) []flavor {
	t.Helper()

	return []flavor{
		{
			name:        "v3",
			dir:         "v3",
			nodeinfo:    fixture(t, "v3", "nodeinfo.json"),
			validate:    "GET /api/v3/user/validate_auth",
			forums:      "GET /api/v3/community/list",
			posts:       "GET /api/v3/post/list",
			post:        "GET /api/v3/post",
			comments:    "GET /api/v3/comment/list",
			createPost:  "POST /api/v3/post",
			createReply: "POST /api/v3/comment",
			login:       "POST /api/v3/user/login",
			sessionOK:   `{"success":true}`,
			notLoggedIn: `{"error":"not_logged_in"}`,
			rateLimited: route{http.StatusBadRequest, `{"error":"rate_limit_error"}`},
			sorts: map[system.Order]string{
				system.OrderNew:      "New",
				system.OrderActive:   "NewComments",
				system.OrderHot:      "Hot",
				system.OrderTopDay:   "TopDay",
				system.OrderTopWeek:  "TopWeek",
				system.OrderTopMonth: "TopMonth",
				system.OrderTopYear:  "TopYear",
				system.OrderTopAll:   "TopAll",
				system.OrderComments: "MostComments",
			},
		},
		{
			name:        "v4",
			dir:         "v4",
			nodeinfo:    fixture(t, "v4", "nodeinfo.json"),
			validate:    "GET /api/v4/account/validate_auth",
			forums:      "GET /api/v4/community/list",
			posts:       "GET /api/v4/post/list",
			post:        "GET /api/v4/post",
			comments:    "GET /api/v4/comment/list",
			createPost:  "POST /api/v4/post",
			createReply: "POST /api/v4/comment",
			login:       "POST /api/v4/account/auth/login",
			sessionOK:   `{"success":true}`,
			notLoggedIn: `{"error":"not_logged_in","cause":"NotLoggedIn"}`,
			rateLimited: route{http.StatusTooManyRequests, `{"error":"too_many_requests"}`},
			sorts: map[system.Order]string{
				system.OrderNew:      "new",
				system.OrderActive:   "new_comments",
				system.OrderHot:      "hot",
				system.OrderTopDay:   "top",
				system.OrderTopWeek:  "top",
				system.OrderTopMonth: "top",
				system.OrderTopYear:  "top",
				system.OrderTopAll:   "top",
				system.OrderComments: "most_comments",
			},
			timeRanges: map[system.Order]string{
				system.OrderNew:      "0",
				system.OrderActive:   "0",
				system.OrderHot:      "0",
				system.OrderTopDay:   "86400",
				system.OrderTopWeek:  "604800",
				system.OrderTopMonth: "2592000",
				system.OrderTopYear:  "31536000",
				system.OrderTopAll:   "0",
				system.OrderComments: "0",
			},
		},
	}
}

func (fl flavor) list(t *testing.T, items []map[string]any) string {
	t.Helper()

	key := "items"
	if fl.dir == "v3" {
		key = "posts"
	}
	b, err := json.Marshal(map[string]any{key: items})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func (fl flavor) captured(t *testing.T, name string, key string) []map[string]any {
	t.Helper()

	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(fixture(t, fl.dir, name)), &doc); err != nil {
		t.Fatal(err)
	}
	var items []map[string]any
	if err := json.Unmarshal(doc[key], &items); err != nil {
		t.Fatal(err)
	}
	return items
}

func (fl flavor) onePost(t *testing.T) string {
	t.Helper()

	key := "items"
	if fl.dir == "v3" {
		key = "posts"
	}
	return fl.list(t, fl.captured(t, "posts.json", key)[:1])
}

func (fl flavor) noForums() string {
	if fl.dir == "v3" {
		return `{"communities":[]}`
	}
	return `{"items":[]}`
}

type community struct {
	id          int
	name        string
	title       string
	summary     string
	description string
}

func (fl flavor) forumList(t *testing.T, communities ...community) string {
	t.Helper()

	key := "items"
	if fl.dir == "v3" {
		key = "communities"
	}
	template := fl.captured(t, "communities.json", key)[0]

	items := make([]map[string]any, 0, len(communities))
	for _, c := range communities {
		raw, err := json.Marshal(template)
		if err != nil {
			t.Fatal(err)
		}
		var item map[string]any
		if err := json.Unmarshal(raw, &item); err != nil {
			t.Fatal(err)
		}

		fields := item["community"].(map[string]any)
		fields["id"] = c.id
		fields["name"] = c.name
		fields["title"] = c.title
		delete(fields, "description")
		delete(fields, "sidebar")
		delete(fields, "summary")
		if fl.dir == "v3" {
			if c.description != "" {
				fields["description"] = c.description
			}
		} else {
			if c.description != "" {
				fields["sidebar"] = c.description
			}
			if c.summary != "" {
				fields["summary"] = c.summary
			}
		}
		items = append(items, item)
	}

	b, err := json.Marshal(map[string]any{key: items})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func (fl flavor) serveNodeinfo(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, fl.nodeinfo)
}

func blockingLemmy(t *testing.T, fl flavor) string {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == nodeinfoPath {
			fl.serveNodeinfo(w)
			return
		}
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	t.Cleanup(func() {
		srv.CloseClientConnections()
		srv.Close()
	})
	return srv.URL
}
