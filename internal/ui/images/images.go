package images

import (
	"bytes"
	"context"
	"fmt"
	"image/color"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/eliukblau/pixterm/pkg/ansimage"
	"github.com/mrusme/neonmodem/internal/system/httpx"
)

const (
	maxConcurrent = 4
	maxBytes      = 2 << 20
	fetchTimeout  = 5 * time.Second
	cacheEntries  = 64
	minWidth      = 8
)

var imageURL = regexp.MustCompile(
	`(?m)(http|ftp|https):\/\/([\w_-]+(?:(?:\.[\w_-]+)+))([\w.,@?^=%&:\/~+#-]*[\w@?^=%&\/~+#-])\.(jpg|jpeg|png)`)

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

func cacheKey(url string, width int) string {
	return fmt.Sprintf("%d|%s", width, url)
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

func (r *Renderer) fetch(ctx context.Context, url string) ([]byte, error) {
	select {
	case r.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-r.sem }()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", res.StatusCode)
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

func (r *Renderer) render(ctx context.Context, url string, width int) (string, error) {
	data, err := r.fetch(ctx, url)
	if err != nil {
		return "", err
	}

	pix, err := ansimage.NewScaledFromReader(
		bytes.NewReader(data),
		int(float32(width)*0.75),
		width,
		color.Transparent,
		ansimage.ScaleModeResize,
		ansimage.NoDithering,
	)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("\n\n%s\nSource: %s\n\n", pix.RenderExt(false, false), url), nil
}

func (r *Renderer) RenderInline(ctx context.Context, s string, width int) string {
	if width < minWidth {
		return s
	}

	return imageURL.ReplaceAllStringFunc(s, func(url string) string {
		key := cacheKey(url, width)
		if rendered, ok := r.cached(key); ok {
			return rendered
		}
		if ctx.Err() != nil {
			return url
		}

		rendered, err := r.render(ctx, url, width)
		if err != nil {
			r.logger.Debug("inline image skipped", "url", url, "error", err)
			return url
		}

		r.remember(key, rendered)
		return rendered
	})
}
