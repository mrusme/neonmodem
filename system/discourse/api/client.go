package api

// https://docs.discourse.org

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

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
	proxy string,
	credentials map[string]string,
	logger Logger,
) ClientConfig {
	var httpClient *http.Client = nil
	var httpTransport *http.Transport = nil

	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			logger.Error(err)
		} else {
			logger.Debugf("setting up http proxy transport: %s\n", proxyURL.String())
			httpTransport = &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			}
		}
	}

	httpClient = &http.Client{
		Transport: httpTransport,
		Timeout:   time.Second * 10,
	}

	return ClientConfig{
		Endpoint:    endpoint,
		Credentials: credentials,
		HTTPClient:  httpClient,
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

	if req, err = http.NewRequest(
		method,
		parsedURL.String(),
		buffer,
	); err != nil {
		return nil, err
	}

	req.Header.Add("User-Agent", "Neon Modem Overdrive")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Api-Client-Id", c.credentials["client_id"])
	req.Header.Add("User-Api-Key", c.credentials["key"])

	return req, nil
}

type ErrorBody struct {
	Errors []string `json:"errors"`
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

	if res.StatusCode < http.StatusOK ||
		res.StatusCode > http.StatusNoContent {
		var errbody ErrorBody
		if err := json.Unmarshal(body, &errbody); err == nil {
			return &RequestError{
				Err: errors.New(strings.Join(errbody.Errors, "\n")),
			}
		}

		return &RequestError{
			Err: errors.New(string(body)),
		}
	}

	return nil
}
