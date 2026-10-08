package lobsters

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/mrusme/neonmodem/internal/system/httpx"
)

var (
	ErrTwoFactor = errors.New(
		"this account has two-factor authentication enabled, which the " +
			"Lobsters form login can't complete; use the browser to post")
	ErrBadLogin = errors.New("Lobsters rejected the username or password")

	storyPathRE = regexp.MustCompile(`^/s/([a-z0-9]+)`)
)

type webSession struct {
	base     *url.URL
	http     *http.Client
	username string
	password string
	logger   *slog.Logger

	mu       sync.Mutex
	loggedIn bool
}

func newWebSession(
	baseURL string,
	username string,
	password string,
	proxy string,
	logger *slog.Logger,
) (*webSession, error) {
	base, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return nil, err
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

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
	}, nil
}

func (w *webSession) url(path string) string {
	return w.base.JoinPath(path).String()
}

func (w *webSession) do(req *http.Request) (*http.Response, *goquery.Document, error) {
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	res, err := w.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, nil, err
	}

	if res.StatusCode == http.StatusTooManyRequests {
		return res, nil, fmt.Errorf(
			"%s is rate limiting requests, please wait a moment and try again",
			w.base.Host)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return res, nil, err
	}

	return res, doc, nil
}

func (w *webSession) get(ctx context.Context, path string) (*http.Response, *goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.url(path), nil)
	if err != nil {
		return nil, nil, err
	}
	return w.do(req)
}

func (w *webSession) postForm(
	ctx context.Context,
	path string,
	form url.Values,
) (*http.Response, *goquery.Document, error) {
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, w.url(path), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return w.do(req)
}

func (w *webSession) csrfToken(ctx context.Context, path string) (string, error) {
	_, doc, err := w.get(ctx, path)
	if err != nil {
		return "", err
	}

	token, ok := doc.Find(`meta[name="csrf-token"]`).Attr("content")
	if !ok || token == "" {
		return "", fmt.Errorf("no CSRF token found on %s", w.url(path))
	}

	return token, nil
}

func flashError(doc *goquery.Document) string {
	if doc == nil {
		return ""
	}
	return strings.TrimSpace(doc.Find(".flash-error, .flash-notice, .error").First().Text())
}

func isLoggedIn(doc *goquery.Document) bool {
	if doc == nil {
		return false
	}
	return doc.Find(`form[action="/logout"]`).Length() > 0 ||
		doc.Find(`a[href="/settings"]`).Length() > 0
}

func (w *webSession) login(ctx context.Context) error {
	token, err := w.csrfToken(ctx, "/login")
	if err != nil {
		return err
	}

	form := url.Values{}
	form.Set("email", w.username)
	form.Set("password", w.password)
	form.Set("authenticity_token", token)

	res, doc, err := w.postForm(ctx, "/login", form)
	if err != nil {
		return err
	}

	if res.Request != nil && strings.HasPrefix(res.Request.URL.Path, "/login/2fa") {
		return ErrTwoFactor
	}
	if isLoggedIn(doc) {
		return nil
	}
	if msg := flashError(doc); msg != "" {
		return fmt.Errorf("%w: %s", ErrBadLogin, msg)
	}

	return ErrBadLogin
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

func (w *webSession) Verify(ctx context.Context) error {
	return w.ensureLogin(ctx)
}

func (w *webSession) PostComment(
	ctx context.Context,
	storyID string,
	parentID string,
	body string,
) (string, error) {
	if err := w.ensureLogin(ctx); err != nil {
		return "", err
	}

	token, err := w.csrfToken(ctx, "/s/"+storyID)
	if err != nil {
		return "", err
	}

	form := url.Values{}
	form.Set("story_id", storyID)
	form.Set("comment", body)
	form.Set("authenticity_token", token)
	if parentID != "" {
		form.Set("parent_comment_short_id", parentID)
	}

	res, doc, err := w.postForm(ctx, "/comments", form)
	if err != nil {
		return "", err
	}

	if doc != nil {
		if id := createdCommentID(doc, parentID); id != "" {
			return id, nil
		}
		if msg := flashError(doc); msg != "" {
			return "", fmt.Errorf("Lobsters did not accept the comment: %s", msg)
		}
		if text := strings.TrimSpace(doc.Text()); res.StatusCode >= 400 && text != "" {
			return "", fmt.Errorf("Lobsters did not accept the comment: %s", firstLine(text))
		}
	}

	return "", fmt.Errorf(
		"Lobsters did not accept the comment (status %d)", res.StatusCode)
}

func (w *webSession) SubmitStory(
	ctx context.Context,
	title string,
	body string,
	isLink bool,
	tag string,
) (string, error) {
	if err := w.ensureLogin(ctx); err != nil {
		return "", err
	}

	token, err := w.csrfToken(ctx, "/stories/new")
	if err != nil {
		return "", err
	}

	form := url.Values{}
	form.Set("authenticity_token", token)
	form.Set("story[title]", title)
	if isLink {
		form.Set("story[url]", strings.TrimSpace(body))
	} else {
		form.Set("story[description]", body)
	}
	if tag != "" {
		form.Add("story[tags][]", tag)
	}

	res, doc, err := w.postForm(ctx, "/stories", form)
	if err != nil {
		return "", err
	}

	if res.Request != nil {
		if m := storyPathRE.FindStringSubmatch(res.Request.URL.Path); m != nil {
			return m[1], nil
		}
	}
	if msg := flashError(doc); msg != "" {
		return "", fmt.Errorf("Lobsters did not accept the story: %s", msg)
	}

	return "", fmt.Errorf(
		"Lobsters did not accept the story (status %d)", res.StatusCode)
}

func createdCommentID(doc *goquery.Document, parentID string) string {
	id := ""
	doc.Find(".comment[data-shortid]").Each(func(_ int, c *goquery.Selection) {
		if shortID, ok := c.Attr("data-shortid"); ok && shortID != "" && shortID != parentID {
			id = shortID
		}
	})
	return id
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
