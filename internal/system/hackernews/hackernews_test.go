package hackernews

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/mrusme/neonmodem/internal/models/forum"
	"github.com/mrusme/neonmodem/internal/models/post"
	"github.com/mrusme/neonmodem/internal/models/reply"
	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/hackernews/api"
	"github.com/mrusme/neonmodem/internal/system/httpx"
)

func parseHTML(t *testing.T, html string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func apiFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("api", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

const (
	loggedInPage = `<html><body><span class="pagetop"><a id="me" href="user?id=tester">tester</a> | <a id="logout" href="logout">logout</a></span></body></html>`
	loginPage    = `<html><body>You have to be logged in to submit.<form action="login" method="post"><input type="text" name="acct"><input type="password" name="pw"></form></body></html>`
	submitPage   = `<html><body><table><tr><td><span class="pagetop"><b>Submit</b></span></td></tr></table><form action="/r" method="post"><input type="hidden" name="fnid" value="FNID123"><input type="hidden" name="fnop" value="submit-page"><input type="text" name="title"><input type="url" name="url"><textarea name="text"></textarea><input type="submit" value="submit"></form></body></html>`
)

type fakeSite struct {
	mu sync.Mutex

	algoliaOK    bool
	firebaseHits int

	loggedIn  bool
	logins    int
	badLogin  bool
	captcha   bool
	dropLogin int

	comments       []url.Values
	postingTooFast bool

	submissions []url.Values
	submitReply func(r *http.Request, attempt int) (status int, location string, body string)
}

func newFakeSite(t *testing.T) (*fakeSite, *httptest.Server) {
	t.Helper()
	site := &fakeSite{algoliaOK: true}
	mux := http.NewServeMux()

	writeJSON := func(w http.ResponseWriter, body []byte) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}
	writeHTML := func(w http.ResponseWriter, body string) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(body))
	}

	mux.HandleFunc("/firebase/topstories.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []byte(`[49953495, 1]`))
	})
	mux.HandleFunc("/firebase/askstories.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []byte(`[49953495]`))
	})
	mux.HandleFunc("/firebase/newstories.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []byte(`[49953495]`))
	})
	mux.HandleFunc("/firebase/item/49953495.json", func(w http.ResponseWriter, r *http.Request) {
		site.mu.Lock()
		site.firebaseHits++
		site.mu.Unlock()
		writeJSON(w, apiFixture(t, "item_story.json"))
	})
	mux.HandleFunc("/firebase/item/49953496.json", func(w http.ResponseWriter, r *http.Request) {
		site.mu.Lock()
		site.firebaseHits++
		site.mu.Unlock()
		writeJSON(w, apiFixture(t, "item_comment.json"))
	})
	mux.HandleFunc("/firebase/item/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []byte("null"))
	})
	mux.HandleFunc("/algolia/items/49953495", func(w http.ResponseWriter, r *http.Request) {
		if !site.algoliaOK {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		writeJSON(w, apiFixture(t, "algolia_item.json"))
	})

	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeHTML(w, string(fixture(t, "login_page.html")))
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("parsing the form failed: %v", err)
		}
		site.mu.Lock()
		defer site.mu.Unlock()
		site.logins++
		switch {
		case site.captcha:
			writeHTML(w, `<html><head><script src="https://www.google.com/recaptcha/api.js"></script></head><body><div class="g-recaptcha"></div></body></html>`)
		case site.badLogin || r.Form.Get("acct") != "tester" || r.Form.Get("pw") != "secret":
			writeHTML(w, `<html><body>Bad login.</body></html>`)
		default:
			site.loggedIn = true
			http.Redirect(w, r, "/news", http.StatusFound)
		}
	})
	mux.HandleFunc("/news", func(w http.ResponseWriter, r *http.Request) {
		writeHTML(w, loggedInPage)
	})

	mux.HandleFunc("/item", func(w http.ResponseWriter, r *http.Request) {
		page := string(fixture(t, "item_page.html"))
		var rows strings.Builder
		rows.WriteString(`<table><tr class="comtr" id="555"><td><span class="hnuser">someone</span><div class="commtext">Please try again later, it worked for me.</div></td></tr>`)
		site.mu.Lock()
		for i, c := range site.comments {
			fmt.Fprintf(&rows, `<tr class="comtr" id="%d"><td><span class="hnuser">tester</span><div class="commtext">%s</div></td></tr>`, 777+i, c.Get("text"))
		}
		site.mu.Unlock()
		writeHTML(w, strings.Replace(page, "</body>", rows.String()+"</table></body>", 1))
	})
	mux.HandleFunc("/comment", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parsing the form failed: %v", err)
		}
		site.mu.Lock()
		defer site.mu.Unlock()
		if !site.loggedIn {
			writeHTML(w, loginPage)
			return
		}
		if site.postingTooFast {
			writeHTML(w, `<html><body>You're posting too fast. Please slow down. Thanks.</body></html>`)
			return
		}
		if r.Form.Get("hmac") == "" {
			writeHTML(w, `<html><body>Please try again.</body></html>`)
			return
		}
		site.comments = append(site.comments, r.Form)
		http.Redirect(w, r, "/"+r.Form.Get("goto"), http.StatusFound)
	})

	mux.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		site.mu.Lock()
		defer site.mu.Unlock()
		if site.dropLogin > 0 {
			site.dropLogin--
			site.loggedIn = false
		}
		if !site.loggedIn {
			writeHTML(w, loginPage)
			return
		}
		writeHTML(w, submitPage)
	})
	mux.HandleFunc("/r", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parsing the form failed: %v", err)
		}
		site.mu.Lock()
		site.submissions = append(site.submissions, r.Form)
		attempt := len(site.submissions)
		reply := site.submitReply
		site.mu.Unlock()

		status, location, body := http.StatusFound, "/newest", ""
		if reply != nil {
			status, location, body = reply(r, attempt)
		}
		if location != "" {
			http.Redirect(w, r, location, status)
			return
		}
		writeHTML(w, body)
	})
	mux.HandleFunc("/newest", func(w http.ResponseWriter, r *http.Request) {
		writeHTML(w, loggedInPage)
	})
	mux.HandleFunc("/x", func(w http.ResponseWriter, r *http.Request) {
		writeHTML(w, `<html><body><span class="pagetop"><b>Submit</b></span><p>Please limit titles to 80 characters.</p><form action="/r" method="post"><input type="hidden" name="fnid" value="FNID999"></form></body></html>`)
	})
	mux.HandleFunc("/submitted", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("id") != "tester" {
			writeHTML(w, `<html><body>No such user.</body></html>`)
			return
		}
		writeHTML(w, `<html><body><table><tr class="athing submission" id="555555"><td>Ask HN: newest</td></tr><tr class="athing submission" id="111"><td>older</td></tr></table></body></html>`)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return site, srv
}

func testSystem(t *testing.T, srv *httptest.Server, creds map[string]string) *System {
	t.Helper()

	client, err := api.NewClientWithBases(httpx.NewHTTPClient(httpx.Options{}), srv.URL+"/firebase", srv.URL+"/algolia")
	if err != nil {
		t.Fatal(err)
	}
	sys := &System{
		idx:      2,
		settings: system.Settings{URL: srv.URL, Credentials: creds},
		logger:   slog.New(slog.DiscardHandler),
	}
	sys, err = newWithClients(sys, client, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return sys
}

var account = map[string]string{"username": "tester", "password": "secret"}

func TestFirebaseListsServeHotAndTheFeedsNew(t *testing.T) {
	_, srv := newFakeSite(t)
	sys := testSystem(t, srv, nil)

	for _, tc := range []struct {
		forumID string
		order   system.Order
	}{
		{"", system.OrderHot},
		{"", system.OrderNew},
		{"ask", system.OrderHot},
	} {
		posts, err := sys.ListPosts(context.Background(), tc.forumID, tc.order)
		if err != nil {
			t.Fatalf("ListPosts(%q, %s): %v", tc.forumID, tc.order, err)
		}
		if len(posts) != 1 {
			t.Fatalf("ListPosts(%q, %s): expected the one existing story, got %d",
				tc.forumID, tc.order, len(posts))
		}

		p := posts[0]
		if p.Forum.ID != tc.forumID {
			t.Errorf("a story from %q should belong to that forum: %+v", tc.forumID, p.Forum)
		}
		if p.Kind != post.KindLink || !strings.HasPrefix(p.Body, "https://") {
			t.Errorf("a story with a url must be a link post: kind=%v body=%q", p.Kind, p.Body)
		}

		var item api.Item
		if err := json.Unmarshal(apiFixture(t, "item_story.json"), &item); err != nil {
			t.Fatal(err)
		}
		if p.ReplyCount != item.Descendants || p.Score != (post.Score{Value: item.Score, Unit: post.ScorePoints}) {
			t.Errorf("replies %d and score %+v, want %d and %d points",
				p.ReplyCount, p.Score, item.Descendants, item.Score)
		}
		if p.SysIDX != 2 || p.Author.Name != item.By {
			t.Errorf("unexpected post: %+v", p)
		}
	}
}

func TestLoadPostBuildsTreeFromAlgolia(t *testing.T) {
	_, srv := newFakeSite(t)
	sys := testSystem(t, srv, nil)

	p := post.Post{ID: "49953495", Kind: post.KindLink, Body: "https://x", SysIDX: 2}
	if err := sys.LoadPost(context.Background(), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Replies) != 3 {
		t.Fatalf("expected 3 top-level replies from the trimmed fixture, got %d", len(p.Replies))
	}
	if p.ReplyCount != reply.Count(p.Replies) {
		t.Errorf("reply count %d does not match the tree %d", p.ReplyCount, reply.Count(p.Replies))
	}
	first := p.Replies[0]
	if first.ParentID != "" || first.PostID != "49953495" || first.Author.Name == "" {
		t.Errorf("unexpected top-level reply: %+v", first)
	}
	if len(first.Replies) == 0 {
		t.Fatal("expected nested replies under the first comment")
	}
	if nested := first.Replies[0]; nested.ParentID != first.ID || nested.PostID != "49953495" {
		t.Errorf("nested reply not linked to its parent: %+v", nested)
	}
}

func TestLoadPostFallsBackToFirebase(t *testing.T) {
	site, srv := newFakeSite(t)
	site.algoliaOK = false
	sys := testSystem(t, srv, nil)

	p := post.Post{ID: "49953495", Forum: forum.Forum{ID: "top"}, SysIDX: 2}
	if err := sys.LoadPost(context.Background(), &p); err != nil {
		t.Fatalf("fallback failed: %v", err)
	}
	if p.Subject == "" {
		t.Error("fallback should fill the subject from the item")
	}
	if p.Link != "https://github.com/Niko1221/Strata" {
		t.Errorf("fallback should fill the link from the item, got %q", p.Link)
	}
	found := false
	for _, r := range p.Replies {
		if r.ID == "49953496" {
			found = true
		}
	}
	if !found {
		t.Errorf("the one comment known to the fake api should be in the tree: %+v", p.Replies)
	}
}

func TestLoadPostAfterAReplyUsesTheOfficialAPI(t *testing.T) {
	site, srv := newFakeSite(t)
	sys := testSystem(t, srv, account)

	p := post.Post{ID: "49953495", Forum: forum.Forum{ID: "top"}, SysIDX: 2}
	if err := sys.LoadPost(context.Background(), &p); err != nil {
		t.Fatal(err)
	}
	site.mu.Lock()
	before := site.firebaseHits
	site.mu.Unlock()
	if before != 0 {
		t.Fatalf("without recent writes the Algolia tree should be used, got %d official api requests", before)
	}

	r := reply.Reply{PostID: "49953495", Body: "fresh reply"}
	if err := sys.CreateReply(context.Background(), &r); err != nil {
		t.Fatal(err)
	}
	if err := sys.LoadPost(context.Background(), &p); err != nil {
		t.Fatal(err)
	}
	site.mu.Lock()
	after := site.firebaseHits
	site.mu.Unlock()
	if after == 0 {
		t.Fatal("Algolia's tree lacks the new reply, so the official api should have been used")
	}
}

func TestTitleFor(t *testing.T) {
	cases := map[[2]string]string{
		{"ask", "Is this a question?"}: "Ask HN: Is this a question?",
		{"ask", "ask hn: already"}:     "ask hn: already",
		{"show", " My project "}:       "Show HN: My project",
		{"top", "Plain"}:               "Plain",
		{"new", "Show HN: keep as is"}: "Show HN: keep as is",
	}
	for in, want := range cases {
		if got := titleFor(in[0], in[1]); got != want {
			t.Errorf("titleFor(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}

func TestWriteCapabilitiesNeedCredentials(t *testing.T) {
	_, srv := newFakeSite(t)
	anon := testSystem(t, srv, nil)
	if anon.Capabilities().Has(system.CapCreateReply) {
		t.Error("anonymous system reports write capabilities")
	}
	if err := anon.CreateReply(context.Background(), &reply.Reply{PostID: "1"}); !errors.Is(err, system.ErrNoCredentials) {
		t.Errorf("expected ErrNoCredentials, got %v", err)
	}
}

func TestCommentFlow(t *testing.T) {
	site, srv := newFakeSite(t)
	sys := testSystem(t, srv, account)

	r := reply.Reply{PostID: "49953495", Body: "hello from the test"}
	if err := sys.CreateReply(context.Background(), &r); err != nil {
		t.Fatalf("CreateReply: %v", err)
	}
	if r.ID != "777" {
		t.Errorf("expected the posted comment id 777, got %q", r.ID)
	}

	site.mu.Lock()
	defer site.mu.Unlock()
	if site.logins != 1 {
		t.Errorf("expected one login, got %d", site.logins)
	}
	if len(site.comments) != 1 {
		t.Fatalf("expected one comment post, got %d", len(site.comments))
	}
	form := site.comments[0]
	if form.Get("parent") != "49953495" || form.Get("hmac") == "" || form.Get("text") != "hello from the test" {
		t.Errorf("unexpected comment form: %v", form)
	}
	if form.Get("goto") != "item?id=49953495" {
		t.Errorf("unexpected goto: %q", form.Get("goto"))
	}
}

func TestCommentRejectionIsReported(t *testing.T) {
	site, srv := newFakeSite(t)
	site.postingTooFast = true
	sys := testSystem(t, srv, account)

	err := sys.CreateReply(context.Background(), &reply.Reply{PostID: "49953495", Body: "too soon"})
	if err == nil || !strings.Contains(err.Error(), "posting too fast") {
		t.Fatalf("expected the rate limit message, got %v", err)
	}
}

func TestBadLoginIsReported(t *testing.T) {
	site, srv := newFakeSite(t)
	site.badLogin = true
	sys := testSystem(t, srv, map[string]string{"username": "tester", "password": "wrong"})

	err := sys.CreateReply(context.Background(), &reply.Reply{PostID: "49953495", Body: "x"})
	if !errors.Is(err, ErrBadLogin) {
		t.Fatalf("expected ErrBadLogin, got %v", err)
	}
}

func TestCaptchaIsReported(t *testing.T) {
	site, srv := newFakeSite(t)
	site.captcha = true
	sys := testSystem(t, srv, account)

	err := sys.CreateReply(context.Background(), &reply.Reply{PostID: "49953495", Body: "x"})
	if !errors.Is(err, ErrCaptcha) {
		t.Fatalf("expected ErrCaptcha, got %v", err)
	}
}

func TestSubmitOnMinimalSubmitPage(t *testing.T) {
	site, srv := newFakeSite(t)
	sys := testSystem(t, srv, account)

	p := post.Post{Subject: "What do you use?", Body: "Curious about terminal clients.", Kind: post.KindText}
	p.Forum.ID = "ask"
	if err := sys.CreatePost(context.Background(), &p); err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	if p.ID != "555555" {
		t.Errorf("expected the newest submission's id, got %q", p.ID)
	}
	if p.Subject != "Ask HN: What do you use?" {
		t.Errorf("title should get the Ask HN prefix, got %q", p.Subject)
	}

	site.mu.Lock()
	defer site.mu.Unlock()
	if len(site.submissions) != 1 {
		t.Fatalf("expected one submission, got %d", len(site.submissions))
	}
	form := site.submissions[0]
	if form.Get("fnid") != "FNID123" || form.Get("fnop") != "submit-page" {
		t.Errorf("hidden fields not taken from the page: %v", form)
	}
	if form.Get("url") != "" || form.Get("text") != "Curious about terminal clients." {
		t.Errorf("a text post must send text and no url: %v", form)
	}
}

func TestSubmitRetriesExpiredForm(t *testing.T) {
	site, srv := newFakeSite(t)
	site.submitReply = func(r *http.Request, attempt int) (int, string, string) {
		if attempt == 1 {
			return http.StatusOK, "", `<html><body>Unknown or expired link.</body></html>`
		}
		return http.StatusFound, "/item?id=424242", ""
	}
	sys := testSystem(t, srv, account)

	p := post.Post{Subject: "expire me", Body: "https://example.com/x", Kind: post.KindLink}
	p.Forum.ID = "show"
	if err := sys.CreatePost(context.Background(), &p); err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	if p.ID != "424242" {
		t.Errorf("expected the redirected item id, got %q", p.ID)
	}
	if p.Subject != "Show HN: expire me" {
		t.Errorf("title should get the Show HN prefix, got %q", p.Subject)
	}

	site.mu.Lock()
	defer site.mu.Unlock()
	if len(site.submissions) != 2 {
		t.Fatalf("expected one retry after the expired form, got %d submits", len(site.submissions))
	}
	last := site.submissions[1]
	if last.Get("url") != "https://example.com/x" || last.Get("text") != "" {
		t.Errorf("a link post must send the url and no text: %v", last)
	}
}

func TestSubmitRejectionIsReported(t *testing.T) {
	site, srv := newFakeSite(t)
	site.submitReply = func(r *http.Request, attempt int) (int, string, string) {
		return http.StatusFound, "/x?fnid=abc", ""
	}
	sys := testSystem(t, srv, account)

	p := post.Post{Subject: strings.Repeat("long ", 30), Body: "text", Kind: post.KindText}
	err := sys.CreatePost(context.Background(), &p)
	if err == nil || !strings.Contains(err.Error(), "limit titles to 80 characters") {
		t.Fatalf("expected the page's message, got %v", err)
	}
	if p.ID != "" {
		t.Errorf("a rejected submission must not get an id, got %q", p.ID)
	}
}

func TestSubmitLogsInAgainWhenTheFormPageShowsTheLoginForm(t *testing.T) {
	site, srv := newFakeSite(t)
	sys := testSystem(t, srv, account)

	if err := sys.web.Verify(context.Background()); err != nil {
		t.Fatal(err)
	}
	site.mu.Lock()
	site.dropLogin = 1
	site.mu.Unlock()

	p := post.Post{Subject: "after a dropped session", Body: "text", Kind: post.KindText}
	if err := sys.CreatePost(context.Background(), &p); err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	site.mu.Lock()
	defer site.mu.Unlock()
	if site.logins != 2 {
		t.Errorf("expected a second login after the session dropped, got %d logins", site.logins)
	}
}

func TestResolveAction(t *testing.T) {
	page, _ := url.Parse("https://news.ycombinator.com/item?id=1")
	cases := map[string]string{
		"/r":      "https://news.ycombinator.com/r",
		"r":       "https://news.ycombinator.com/r",
		"comment": "https://news.ycombinator.com/comment",
		"":        "https://news.ycombinator.com/item?id=1",
	}
	for action, want := range cases {
		doc := parseHTML(t, `<form action="`+action+`"></form>`)
		if got := resolveAction(page, doc.Find("form")); got != want {
			t.Errorf("action %q resolved to %q, want %q", action, got, want)
		}
	}
}

func TestFindOwnCommentPicksMatchingRow(t *testing.T) {
	doc := parseHTML(t, `<table>
<tr class="comtr" id="1"><td><span class="hnuser">tester</span><div class="commtext">older comment</div></td></tr>
<tr class="comtr" id="2"><td><span class="hnuser">other</span><div class="commtext">new text here</div></td></tr>
<tr class="comtr" id="3"><td><span class="hnuser">tester</span><div class="commtext">new text here indeed</div></td></tr>
</table>`)
	if got := findOwnComment(doc, "tester", "new text here"); got != "3" {
		t.Errorf("expected row 3, got %q", got)
	}
	if got := findOwnComment(doc, "nobody", "x"); got != "" {
		t.Errorf("expected no match, got %q", got)
	}
}

func TestPostsCarryTheirLink(t *testing.T) {
	sys := &System{idx: 2}

	story := sys.toPost(&api.Item{ID: 1, Type: "story", Title: "Strata", URL: "https://example.com/strata"})
	if story.Kind != post.KindLink || story.Link != "https://example.com/strata" {
		t.Errorf("story: kind=%v link=%q", story.Kind, story.Link)
	}

	hit := sys.hitToPost(api.SearchHit{ObjectID: "2", Title: "Forth", URL: "https://example.com/forth"})
	if hit.Kind != post.KindLink || hit.Link != "https://example.com/forth" {
		t.Errorf("hit: kind=%v link=%q", hit.Kind, hit.Link)
	}

	ask := sys.toPost(&api.Item{ID: 3, Type: "story", Title: "Ask HN: Why?", Text: "<p>Because</p>"})
	if ask.Link != "" {
		t.Errorf("a text post has no link, got %q", ask.Link)
	}
}
