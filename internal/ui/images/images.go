package images

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/color"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/eliukblau/pixterm/pkg/ansimage"
	"github.com/mrusme/neonmodem/internal/system/httpx"
)

const (
	maxConcurrent = 4
	maxBytes      = 10 << 20
	fetchTimeout  = 15 * time.Second
	cacheEntries  = 64
	minWidth      = 8
)

var (
	errTooNarrow = errors.New("the window is too narrow for images")
	errEmpty     = errors.New("the image rendered empty")
)

type Request struct {
	URL       string
	Width     int
	Height    int
	Authorize func(*http.Request)
}

type Result struct {
	Image string
	Err   error
}

type Renderer struct {
	logger *slog.Logger
	client *http.Client
	sem    chan struct{}

	mu    sync.Mutex
	cache map[string]string
	order []string
}

func New(logger *slog.Logger, proxy string) *Renderer {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	return &Renderer{
		logger: logger,
		client: httpx.NewHTTPClient(httpx.Options{Proxy: proxy, Timeout: fetchTimeout, Logger: logger}),
		sem:    make(chan struct{}, maxConcurrent),
		cache:  map[string]string{},
	}
}

func cacheKey(req Request) string {
	return fmt.Sprintf("%d|%d|%s", req.Width, req.Height, req.URL)
}

func (r *Renderer) cached(key string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.cache[key]
	return v, ok
}

func (r *Renderer) remember(key string, rendered string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.cache[key]; !exists {
		r.order = append(r.order, key)
	}
	r.cache[key] = rendered
	for len(r.order) > cacheEntries {
		delete(r.cache, r.order[0])
		r.order = r.order[1:]
	}
}

func (r *Renderer) Load(ctx context.Context, reqs []Request) []Result {
	results := make([]Result, len(reqs))
	groups := map[string][]int{}
	var keys []string
	for i, req := range reqs {
		key := cacheKey(req)
		if _, ok := groups[key]; !ok {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], i)
	}

	var wg sync.WaitGroup
	for _, key := range keys {
		indexes := groups[key]
		wg.Go(func() {
			result := r.load(ctx, key, reqs[indexes[0]])
			for _, i := range indexes {
				results[i] = result
			}
		})
	}
	wg.Wait()

	return results
}

func (r *Renderer) load(ctx context.Context, key string, req Request) Result {
	if req.Width < minWidth {
		return Result{Err: errTooNarrow}
	}
	if rendered, ok := r.cached(key); ok {
		return Result{Image: rendered}
	}

	select {
	case r.sem <- struct{}{}:
	case <-ctx.Done():
		return Result{Err: ctx.Err()}
	}
	defer func() { <-r.sem }()

	rendered, err := r.render(ctx, req)
	if err != nil {
		r.logger.Debug("image skipped", "url", req.URL, "error", err)
		return Result{Err: err}
	}

	r.remember(key, rendered)
	return Result{Image: rendered}
}

func (r *Renderer) fetch(ctx context.Context, req Request) ([]byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, req.URL, nil)
	if err != nil {
		return nil, err
	}
	if req.Authorize != nil {
		req.Authorize(httpReq)
	}

	res, err := r.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", res.StatusCode)
	}
	if contentType := res.Header.Get("Content-Type"); !imageType(contentType) {
		return nil, fmt.Errorf("type %s", contentType)
	}

	data, err := io.ReadAll(io.LimitReader(res.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBytes {
		return nil, fmt.Errorf("larger than %d bytes", maxBytes)
	}

	return data, nil
}

func imageType(contentType string) bool {
	if contentType == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return strings.HasPrefix(mediaType, "image/") || mediaType == "application/octet-stream"
}

func (r *Renderer) render(ctx context.Context, req Request) (string, error) {
	data, err := r.fetch(ctx, req)
	if err != nil {
		return "", err
	}

	pix, err := ansimage.NewScaledFromReader(
		bytes.NewReader(data),
		2*max(req.Height, 1),
		req.Width,
		color.Transparent,
		ansimage.ScaleModeFit,
		ansimage.NoDithering,
	)
	if err != nil {
		return "", err
	}

	rendered := strings.Trim(pix.RenderExt(false, false), "\n")
	if rendered == "" {
		return "", errEmpty
	}
	return rendered, nil
}
