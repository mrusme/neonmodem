package images

import (
	"bytes"
	"fmt"
	"image/color"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/eliukblau/pixterm/pkg/ansimage"
)

const (
	maxConcurrent = 4
	maxBytes      = 2 << 20
	fetchTimeout  = 5 * time.Second
	cacheEntries  = 64
)

var imageURL = regexp.MustCompile(
	`(?m)(http|ftp|https):\/\/([\w_-]+(?:(?:\.[\w_-]+)+))([\w.,@?^=%&:\/~+#-]*[\w@?^=%&\/~+#-])\.(jpg|jpeg|png)`)

var (
	client = &http.Client{Timeout: fetchTimeout}
	sem    = make(chan struct{}, maxConcurrent)

	cacheMu sync.Mutex
	cache   = map[string]string{}
	order   []string
)

func cacheKey(url string, width int) string {
	return fmt.Sprintf("%d|%s", width, url)
}

func cached(key string) (string, bool) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	v, ok := cache[key]
	return v, ok
}

func remember(key string, rendered string) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if _, exists := cache[key]; !exists {
		order = append(order, key)
	}
	cache[key] = rendered
	for len(order) > cacheEntries {
		delete(cache, order[0])
		order = order[1:]
	}
}

func fetch(url string) ([]byte, error) {
	sem <- struct{}{}
	defer func() { <-sem }()

	res, err := client.Get(url)
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

func render(url string, width int) (string, error) {
	data, err := fetch(url)
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

func RenderInline(logger *slog.Logger, s string, width int) string {
	if width < 8 {
		return s
	}

	return imageURL.ReplaceAllStringFunc(s, func(url string) string {
		key := cacheKey(url, width)
		if rendered, ok := cached(key); ok {
			return rendered
		}

		rendered, err := render(url, width)
		if err != nil {
			logger.Debug("inline image skipped", "url", url, "error", err)
			return url
		}

		remember(key, rendered)
		return rendered
	})
}
