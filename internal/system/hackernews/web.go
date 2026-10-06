package hackernews

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/mrusme/neonmodem/internal/system/httpx"
)

var (
	ErrBadLogin = errors.New("Hacker News rejected the username or password")
	ErrCaptcha  = errors.New(
		"Hacker News asks for a captcha before logging in; log in once in a " +
			"browser and try again later")
	ErrExpiredForm = errors.New("the submission form expired")
	ErrLoggedOut   = errors.New(
		"Hacker News shows the login form again right after logging in")
)

const messageLength = 300

type webSession struct {
	base     *url.URL
	http     *http.Client
	username string
	password string
	logger   *slog.Logger

	mu       sync.Mutex
	loggedIn bool
}

type page struct {
	url *url.URL
	doc *goquery.Document
}

func newWebSession(
	baseURL string,
	username string,
	password string,
	proxy string,
	logger *slog.Logger,
) *webSession {
	base, _ := url.Parse(baseURL)
	jar, _ := cookiejar.New(nil)

	return &webSession{
		base: base,
		http: httpx.NewHTTPClient(httpx.Options{
			Proxy:   proxy,
			Timeout: 30 * time.Second,
			Jar:     jar,
			Logger:  logger,
		}),
		username: username,
		password: password,
		logger:   logger,
	}
}

func (w *webSession) url(path string, query url.Values) string {
	u := w.base.JoinPath(path)
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}
	return u.String()
}

func (w *webSession) do(req *http.Request) (page, error) {
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	res, err := w.http.Do(req)
	if err != nil {
		return page{}, err
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return page{}, err
	}

	return page{url: res.Request.URL, doc: doc}, nil
}

func (w *webSession) get(ctx context.Context, path string, query url.Values) (page, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.url(path, query), nil)
	if err != nil {
		return page{}, err
	}
	return w.do(req)
}

func (w *webSession) postForm(ctx context.Context, target string, form url.Values) (page, error) {
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, target, strings.NewReader(form.Encode()))
	if err != nil {
		return page{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return w.do(req)
}

func isLoggedIn(doc *goquery.Document) bool {
	return doc != nil && doc.Find("a#logout").Length() > 0
}

func requiresLogin(doc *goquery.Document) bool {
	return doc != nil && doc.Find(`input[name="acct"]`).Length() > 0
}

func pageText(doc *goquery.Document) string {
	if doc == nil {
		return ""
	}
	return strings.Join(strings.Fields(doc.Find("body").Text()), " ")
}

func pageMessage(doc *goquery.Document) string {
	text := pageText(doc)
	if len(text) > messageLength {
		text = text[:messageLength] + " ..."
	}
	return text
}

func hasCaptcha(doc *goquery.Document) bool {
	if doc == nil {
		return false
	}
	if doc.Find(`.g-recaptcha, script[src*="recaptcha"], iframe[src*="recaptcha"]`).Length() > 0 {
		return true
	}
	return strings.Contains(strings.ToLower(pageText(doc)), "validation required")
}

func inputValue(form *goquery.Selection, name string, fallback string) string {
	if v, ok := form.Find(`input[name="` + name + `"]`).Attr("value"); ok && v != "" {
		return v
	}
	return fallback
}

func resolveAction(pageURL *url.URL, form *goquery.Selection) string {
	action, _ := form.Attr("action")
	ref, err := url.Parse(action)
	if err != nil || action == "" {
		return pageURL.String()
	}
	return pageURL.ResolveReference(ref).String()
}

func samePath(p page, target string) bool {
	t, err := url.Parse(target)
	if err != nil {
		return false
	}
	return p.url.Path == t.Path
}

func (w *webSession) login(ctx context.Context) error {
	form := url.Values{}
	form.Set("acct", w.username)
	form.Set("pw", w.password)
	form.Set("goto", "news")

	p, err := w.postForm(ctx, w.url("/login", nil), form)
	if err != nil {
		return err
	}

	if isLoggedIn(p.doc) {
		return nil
	}

	switch {
	case hasCaptcha(p.doc):
		return ErrCaptcha
	case strings.Contains(strings.ToLower(pageText(p.doc)), "bad login"):
		return ErrBadLogin
	}

	return fmt.Errorf("%w: unexpected response", ErrBadLogin)
}

func (w *webSession) ensureLogin(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.loggedIn {
		return nil
	}
	if err := w.login(ctx); err != nil {
		return err
	}
	w.loggedIn = true

	return nil
}

func (w *webSession) invalidate() {
	w.mu.Lock()
	w.loggedIn = false
	w.mu.Unlock()
}

func (w *webSession) Verify(ctx context.Context) error {
	return w.ensureLogin(ctx)
}

func (w *webSession) formAt(
	ctx context.Context,
	path string,
	query url.Values,
	field string,
) (*goquery.Selection, string, error) {
	for attempt := 0; attempt < 2; attempt++ {
		if err := w.ensureLogin(ctx); err != nil {
			return nil, "", err
		}

		p, err := w.get(ctx, path, query)
		if err != nil {
			return nil, "", err
		}

		input := p.doc.Find(`input[name="` + field + `"]`).First()
		if input.Length() > 0 {
			form := input.Closest("form")
			return form, resolveAction(p.url, form), nil
		}

		if !requiresLogin(p.doc) {
			return nil, "", fmt.Errorf("the page has no form for this: %s", pageMessage(p.doc))
		}

		w.invalidate()
	}

	return nil, "", ErrLoggedOut
}

func (w *webSession) Comment(ctx context.Context, parentID string, body string) (string, error) {
	query := url.Values{}
	query.Set("id", parentID)

	for attempt := 0; attempt < 2; attempt++ {
		form, target, err := w.formAt(ctx, "/item", query, "hmac")
		if err != nil {
			return "", fmt.Errorf("can't comment on Hacker News item %s: %w", parentID, err)
		}

		values := url.Values{}
		values.Set("parent", inputValue(form, "parent", parentID))
		values.Set("goto", inputValue(form, "goto", "item?id="+parentID))
		values.Set("hmac", inputValue(form, "hmac", ""))
		values.Set("text", body)

		p, err := w.postForm(ctx, target, values)
		if err != nil {
			return "", err
		}

		if requiresLogin(p.doc) {
			w.invalidate()
			continue
		}
		if p.url.Path != "/item" {
			return "", fmt.Errorf("Hacker News did not accept the comment: %s", pageMessage(p.doc))
		}

		return findOwnComment(p.doc, w.username, body), nil
	}

	return "", ErrLoggedOut
}

func findOwnComment(doc *goquery.Document, username string, body string) string {
	if doc == nil {
		return ""
	}

	needle := strings.TrimSpace(body)
	if len(needle) > 40 {
		needle = needle[:40]
	}

	id := ""
	doc.Find("tr.comtr").EachWithBreak(func(_ int, row *goquery.Selection) bool {
		if strings.TrimSpace(row.Find(".hnuser").First().Text()) != username {
			return true
		}
		if needle != "" && !strings.Contains(row.Find(".commtext").Text(), needle) {
			return true
		}
		id, _ = row.Attr("id")
		return false
	})

	return id
}

func (w *webSession) Submit(ctx context.Context, title string, link string, body string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		id, err := w.submitOnce(ctx, title, link, body)
		if err == nil {
			return id, nil
		}
		lastErr = err
		if !errors.Is(err, ErrExpiredForm) {
			break
		}
	}

	return "", lastErr
}

func (w *webSession) submitOnce(ctx context.Context, title string, link string, body string) (string, error) {
	form, target, err := w.formAt(ctx, "/submit", nil, "fnid")
	if err != nil {
		return "", fmt.Errorf("can't submit to Hacker News: %w", err)
	}

	values := url.Values{}
	values.Set("fnid", inputValue(form, "fnid", ""))
	values.Set("fnop", inputValue(form, "fnop", "submit-page"))
	values.Set("title", title)
	values.Set("url", link)
	values.Set("text", body)

	p, err := w.postForm(ctx, target, values)
	if err != nil {
		return "", err
	}

	switch {
	case samePath(p, target):
		if strings.Contains(strings.ToLower(pageText(p.doc)), "expired link") {
			return "", ErrExpiredForm
		}
		return "", fmt.Errorf("Hacker News did not accept the submission: %s", pageMessage(p.doc))
	case requiresLogin(p.doc):
		w.invalidate()
		return "", fmt.Errorf("Hacker News did not accept the submission: %w", ErrLoggedOut)
	case p.url.Path == "/x" || p.url.Path == "/submit":
		return "", fmt.Errorf("Hacker News did not accept the submission: %s", pageMessage(p.doc))
	case p.url.Path == "/item":
		if id := p.url.Query().Get("id"); id != "" {
			return id, nil
		}
	}

	return w.latestSubmission(ctx)
}

func (w *webSession) latestSubmission(ctx context.Context) (string, error) {
	query := url.Values{}
	query.Set("id", w.username)

	p, err := w.get(ctx, "/submitted", query)
	if err != nil {
		return "", fmt.Errorf("Hacker News accepted the submission, but its id could not be looked up: %w", err)
	}

	id, _ := p.doc.Find("tr.athing").First().Attr("id")
	if id == "" {
		return "", errors.New("Hacker News accepted the submission, but it doesn't appear on the submissions page yet")
	}

	return id, nil
}
