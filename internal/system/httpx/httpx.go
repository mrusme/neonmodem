package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

const (
	DefaultUserAgent = "Neon Modem Overdrive"
	DefaultTimeout   = 10 * time.Second

	maxBodyBytes = 16 << 20
)

type Options struct {
	Proxy     string
	Timeout   time.Duration
	Retries   int
	UserAgent string
	Jar       http.CookieJar
	Logger    *slog.Logger
}

func NewHTTPClient(o Options) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if o.Proxy != "" {
		proxyURL, err := url.Parse(o.Proxy)
		if err != nil {
			if o.Logger != nil {
				o.Logger.Error("ignoring invalid proxy", "proxy", o.Proxy, "error", err)
			}
		} else {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	userAgent := o.UserAgent
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}
	timeout := o.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	var rt http.RoundTripper = &headerTransport{next: transport, userAgent: userAgent}

	if o.Retries > 0 {
		rc := retryablehttp.NewClient()
		rc.RetryMax = o.Retries
		rc.Logger = nil
		rc.HTTPClient = &http.Client{Transport: rt, Timeout: timeout}
		return &http.Client{
			Transport: &retryablehttp.RoundTripper{Client: rc},
			Jar:       o.Jar,
		}
	}

	return &http.Client{Transport: rt, Timeout: timeout, Jar: o.Jar}
}

type headerTransport struct {
	next      http.RoundTripper
	userAgent string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req = req.Clone(req.Context())
		req.Header.Set("User-Agent", t.userAgent)
	}
	return t.next.RoundTrip(req)
}

var ErrNotJSON = errors.New("the server did not respond with JSON")

type Error struct {
	Method     string
	URL        string
	StatusCode int
	Body       []byte
	Message    string
	Err        error
}

func (e *Error) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s", e.Method, e.URL)
	if e.StatusCode != 0 {
		fmt.Fprintf(&b, " returned status %d", e.StatusCode)
	}
	switch {
	case e.Message != "":
		b.WriteString(": " + e.Message)
	case e.Err != nil:
		b.WriteString(": " + e.Err.Error())
	}
	return b.String()
}

func (e *Error) Unwrap() error {
	return e.Err
}

func StatusOf(err error) int {
	if e, ok := errors.AsType[*Error](err); ok {
		return e.StatusCode
	}
	return 0
}

type Client struct {
	HTTP        *http.Client
	Base        *url.URL
	Headers     http.Header
	DecodeError func(status int, body []byte) string
}

func NewClient(httpClient *http.Client, base string) (*Client, error) {
	u, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("%q is not an absolute URL", base)
	}

	return &Client{
		HTTP:    httpClient,
		Base:    u,
		Headers: http.Header{},
	}, nil
}

func (c *Client) URL(path string, query url.Values) string {
	u := c.Base.JoinPath(path)
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}
	return u.String()
}

func (c *Client) NewRequest(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	body any,
) (*http.Request, error) {
	var payload io.Reader
	if body != nil {
		buf := new(bytes.Buffer)
		if err := json.NewEncoder(buf).Encode(body); err != nil {
			return nil, err
		}
		payload = buf
	}

	req, err := http.NewRequestWithContext(ctx, method, c.URL(path, query), payload)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, values := range c.Headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	return req, nil
}

func (c *Client) Do(req *http.Request, out any) error {
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, maxBodyBytes))
	if err != nil {
		return err
	}

	e := &Error{
		Method:     req.Method,
		URL:        req.URL.String(),
		StatusCode: res.StatusCode,
		Body:       body,
	}

	if res.StatusCode < http.StatusOK || res.StatusCode > 299 {
		if isJSON(res) && c.DecodeError != nil {
			e.Message = c.DecodeError(res.StatusCode, body)
		}
		if e.Message == "" {
			if isJSON(res) {
				e.Message = strings.TrimSpace(string(head(body, 300)))
			} else {
				e.Err = ErrNotJSON
			}
		}
		return e
	}

	if out == nil || res.StatusCode == http.StatusNoContent || len(body) == 0 {
		return nil
	}

	if !isJSON(res) {
		e.Err = ErrNotJSON
		return e
	}

	if err := json.Unmarshal(body, out); err != nil {
		e.Err = fmt.Errorf("malformed response: %w", err)
		return e
	}

	return nil
}

func (c *Client) Get(ctx context.Context, path string, query url.Values, out any) error {
	req, err := c.NewRequest(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return err
	}
	return c.Do(req, out)
}

func (c *Client) Post(ctx context.Context, path string, body any, out any) error {
	req, err := c.NewRequest(ctx, http.MethodPost, path, nil, body)
	if err != nil {
		return err
	}
	return c.Do(req, out)
}

func (c *Client) Delete(ctx context.Context, path string) error {
	req, err := c.NewRequest(ctx, http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}
	return c.Do(req, nil)
}

func isJSON(res *http.Response) bool {
	mediatype, _, err := mime.ParseMediaType(res.Header.Get("Content-Type"))
	if err != nil {
		return false
	}
	return mediatype == "application/json" || strings.HasSuffix(mediatype, "+json")
}

func head(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[:n]
}
