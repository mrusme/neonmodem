package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/hashicorp/go-retryablehttp"
)

type Response struct {
	Post PostModel `json:"post,omitempty"`
}

type RequestError struct {
	Err error
}

func (re *RequestError) Error() string {
	return re.Err.Error()
}

type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})

	Info(args ...interface{})
	Infof(format string, args ...interface{})

	Warn(args ...interface{})
	Warnf(format string, args ...interface{})

	Error(args ...interface{})
	Errorf(format string, args ...interface{})

	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
}

type StdLogger struct {
	L Logger
}

func NewStdLogger(l Logger) retryablehttp.Logger {
	return &StdLogger{
		L: l,
	}
}

func (l *StdLogger) Printf(message string, v ...interface{}) {
	l.L.Debug(message, v)
}

type ClientConfig struct {
	Endpoint    string
	Credentials map[string]string
	HTTPClient  *http.Client
	Logger      Logger
}

type Client struct {
	httpClient  *retryablehttp.Client
	endpoint    *url.URL
	credentials map[string]string
	logger      Logger

	Posts      PostsService
	Topics     TopicsService
	Categories CategoriesService
}

func NewDefaultClientConfig(
	endpoint string,
	credentials map[string]string,
	logger Logger,
) ClientConfig {
	return ClientConfig{
		Endpoint:    endpoint,
		Credentials: credentials,
		HTTPClient:  http.DefaultClient,
		Logger:      logger,
	}
}

func NewClient(cc *ClientConfig) *Client {
	c := new(Client)
	c.logger = cc.Logger
	c.httpClient = retryablehttp.NewClient()
	c.httpClient.RetryMax = 3
	if c.logger != nil {
		c.httpClient.Logger = NewStdLogger(c.logger)
	}
	c.httpClient.HTTPClient = cc.HTTPClient
	c.endpoint, _ = url.Parse(cc.Endpoint)
	c.credentials = cc.Credentials

	c.Posts = &PostServiceHandler{client: c}
	c.Topics = &TopicServiceHandler{client: c}
	c.Categories = &CategoryServiceHandler{client: c}

	return c
}

func (c *Client) NewRequest(
	ctx context.Context,
	method string,
	location string,
	body interface{},
) (*http.Request, error) {
	var parsedURL *url.URL
	var req *http.Request
	var err error

	if parsedURL, err = c.endpoint.Parse(location); err != nil {
		return nil, err
	}

	buffer := new(bytes.Buffer)
	if body != nil {
		if err = json.NewEncoder(buffer).Encode(body); err != nil {
			return nil, err
		}
	}

	c.logger.Debug(buffer.String())

	if req, err = http.NewRequest(
		method,
		parsedURL.String(),
		buffer,
	); err != nil {
		return nil, err
	}

	req.Header.Add("User-Agent", "gobbs")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Api-Client-Id", c.credentials["client_id"])
	req.Header.Add("User-Api-Key", c.credentials["key"])

	return req, nil
}

func (c *Client) Do(
	ctx context.Context,
	req *http.Request,
	content interface{},
) error {
	var rreq *retryablehttp.Request
	var res *http.Response
	var body []byte
	var err error

	if rreq, err = retryablehttp.FromRequest(req); err != nil {
		return err
	}

	rreq = rreq.WithContext(ctx)
	if res, err = c.httpClient.Do(rreq); err != nil {
		return err
	}
	defer res.Body.Close()

	if body, err = ioutil.ReadAll(res.Body); err != nil {
		return err
	}

	if content != nil {
		if err = json.Unmarshal(body, content); err != nil {
			return err
		}
	}

	c.logger.Debug(res)
	c.logger.Debug(string(body))

	if res.StatusCode < http.StatusOK ||
		res.StatusCode > http.StatusNoContent {
		return &RequestError{
			Err: errors.New("Non-2xx status code"),
		}
	}

	return nil
}
