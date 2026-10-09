package images

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func pngSized(t *testing.T, w int, h int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 200, G: 40, B: 40, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func serve(t *testing.T, handler func(w http.ResponseWriter)) (string, *atomic.Int32) {
	t.Helper()

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		handler(w)
	}))
	t.Cleanup(srv.Close)

	return srv.URL, &hits
}

func servePNG(t *testing.T, w int, h int) (string, *atomic.Int32) {
	t.Helper()

	data := pngSized(t, w, h)
	return serve(t, func(rw http.ResponseWriter) {
		rw.Header().Set("Content-Type", "image/png")
		rw.Write(data)
	})
}

func load(r *Renderer, req Request) Result {
	return r.Load(context.Background(), []Request{req})[0]
}

func lineWidths(rendered string) []int {
	var widths []int
	for line := range strings.SplitSeq(rendered, "\n") {
		widths = append(widths, ansi.StringWidth(line))
	}
	return widths
}

func TestLoadRendersAndCachesPerSize(t *testing.T) {
	base, hits := servePNG(t, 8, 8)
	r := New(nil, "")
	req := Request{URL: base + "/picture.png", Width: 40, Height: 10}

	first := load(r, req)
	if first.Err != nil || !strings.Contains(first.Image, "\x1b[") {
		t.Fatalf("the image was not rendered: %+v", first)
	}
	if second := load(r, req); second.Image != first.Image || hits.Load() != 1 {
		t.Errorf("the cache should serve the second load, got %d fetches", hits.Load())
	}

	load(r, Request{URL: req.URL, Width: 40, Height: 3})
	load(r, Request{URL: req.URL, Width: 20, Height: 10})
	if hits.Load() != 3 {
		t.Errorf("another width or height is another fetch, got %d fetches", hits.Load())
	}

	if narrow := load(r, Request{URL: req.URL, Width: minWidth - 1, Height: 10}); !errors.Is(narrow.Err, errTooNarrow) {
		t.Errorf("a width below the minimum gave %+v", narrow)
	}
	if hits.Load() != 3 {
		t.Errorf("a width below the minimum must not fetch, got %d fetches", hits.Load())
	}
}

func TestImagesKeepTheirShape(t *testing.T) {
	wide, _ := servePNG(t, 40, 10)
	small, _ := servePNG(t, 4, 4)
	tall, _ := servePNG(t, 10, 40)
	r := New(nil, "")

	for _, c := range []struct {
		name      string
		req       Request
		width     int
		maxHeight int
	}{
		{"a wide image fills the width", Request{URL: wide + "/w.png", Width: 20, Height: 10}, 20, 10},
		{"a small image keeps its size", Request{URL: small + "/s.png", Width: 20, Height: 10}, 4, 2},
		{"a tall image is limited by the height", Request{URL: tall + "/t.png", Width: 20, Height: 5}, 2, 5},
	} {
		t.Run(c.name, func(t *testing.T) {
			res := load(r, c.req)
			if res.Err != nil {
				t.Fatal(res.Err)
			}
			widths := lineWidths(res.Image)
			if len(widths) > c.maxHeight {
				t.Errorf("%d rows, at most %d expected", len(widths), c.maxHeight)
			}
			for _, w := range widths {
				if w != c.width {
					t.Errorf("rows are %v columns wide, want %d", widths, c.width)
					break
				}
			}
		})
	}
}

func TestFailedLoadsAreNotCached(t *testing.T) {
	missing, missingHits := serve(t, func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusNotFound)
	})
	huge, hugeHits := serve(t, func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(make([]byte, maxBytes+1))
	})
	broken, _ := serve(t, func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("not an image"))
	})

	r := New(nil, "")
	for _, url := range []string{missing + "/gone.png", huge + "/big.jpg", broken + "/text.jpeg"} {
		if res := load(r, Request{URL: url, Width: 40, Height: 10}); res.Err == nil || res.Image != "" {
			t.Errorf("%s gave %+v", url, res)
		}
	}

	load(r, Request{URL: missing + "/gone.png", Width: 40, Height: 10})
	load(r, Request{URL: huge + "/big.jpg", Width: 40, Height: 10})
	if missingHits.Load() != 2 || hugeHits.Load() != 2 {
		t.Errorf("failures must not be cached, got %d and %d fetches", missingHits.Load(), hugeHits.Load())
	}
}

func TestOnlyImageTypesAreRendered(t *testing.T) {
	data := pngSized(t, 4, 4)
	for _, c := range []struct {
		contentType string
		ok          bool
	}{
		{"image/png", true},
		{"image/webp; charset=binary", true},
		{"application/octet-stream", true},
		{"", true},
		{"text/html; charset=utf-8", false},
		{"video/mp4", false},
	} {
		base, _ := serve(t, func(w http.ResponseWriter) {
			w.Header()["Content-Type"] = []string{c.contentType}
			w.Write(data)
		})
		res := load(New(nil, ""), Request{URL: base + "/file", Width: 20, Height: 10})
		if (res.Err == nil) != c.ok {
			t.Errorf("Content-Type %q gave %+v", c.contentType, res)
		}
	}
}

func TestLoadFetchesFourAtATime(t *testing.T) {
	data := pngSized(t, 4, 4)
	var mu sync.Mutex
	inFlight, peak := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		inFlight++
		peak = max(peak, inFlight)
		mu.Unlock()

		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		inFlight--
		mu.Unlock()
		w.Header().Set("Content-Type", "image/png")
		w.Write(data)
	}))
	t.Cleanup(srv.Close)

	var reqs []Request
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		reqs = append(reqs, Request{URL: srv.URL + "/" + name + ".png", Width: 20, Height: 10})
	}
	reqs = append(reqs, reqs[0])

	start := time.Now()
	results := New(nil, "").Load(context.Background(), reqs)
	took := time.Since(start)

	for i, res := range results {
		if res.Err != nil {
			t.Errorf("request %d failed: %v", i, res.Err)
		}
	}
	if results[8].Image != results[0].Image {
		t.Error("a repeated request gets the same image")
	}
	if peak > maxConcurrent || peak < 2 {
		t.Errorf("%d downloads ran at once, expected between 2 and %d", peak, maxConcurrent)
	}
	if took >= 800*time.Millisecond {
		t.Errorf("eight downloads of 100 ms took %s, they didn't run in parallel", took)
	}
}

func TestAuthorizeSeesEveryRequest(t *testing.T) {
	data := pngSized(t, 4, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer hup_test" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(data)
	}))
	t.Cleanup(srv.Close)

	r := New(nil, "")
	if res := load(r, Request{URL: srv.URL + "/a", Width: 20, Height: 10}); res.Err == nil {
		t.Error("the request without credentials should have failed")
	}
	res := load(r, Request{URL: srv.URL + "/a", Width: 20, Height: 10, Authorize: func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer hup_test")
	}})
	if res.Err != nil {
		t.Errorf("the authorized request failed: %v", res.Err)
	}
}

func TestACancelledLoadStopsTheDownload(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		hits.Add(1)
		<-req.Context().Done()
	}))
	t.Cleanup(srv.Close)

	r := New(nil, "")
	req := Request{URL: srv.URL + "/slow.png", Width: 40, Height: 10}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if res := r.Load(ctx, []Request{req})[0]; res.Err == nil {
		t.Error("a cancelled download must fail")
	}
	if waited := time.Since(start); waited > 3*time.Second {
		t.Errorf("the load waited %s past the cancellation", waited)
	}

	done, cancelDone := context.WithCancel(context.Background())
	cancelDone()
	if res := r.Load(done, []Request{req})[0]; !errors.Is(res.Err, context.Canceled) {
		t.Errorf("a load cancelled before it started gave %+v", res)
	}
	if hits.Load() != 1 {
		t.Errorf("a load cancelled before it started must not fetch, got %d fetches", hits.Load())
	}
}

func TestDownloadsGoThroughTheConfiguredProxy(t *testing.T) {
	data := pngSized(t, 4, 4)
	var hosts []string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		hosts = append(hosts, req.Host)
		w.Header().Set("Content-Type", "image/png")
		w.Write(data)
	}))
	t.Cleanup(proxy.Close)

	r := New(nil, proxy.URL)
	if res := load(r, Request{URL: "http://images.invalid/picture.png", Width: 40, Height: 10}); res.Err != nil {
		t.Fatalf("the image was not rendered through the proxy: %v", res.Err)
	}
	if len(hosts) != 1 || hosts[0] != "images.invalid" {
		t.Errorf("the proxy saw the requests %q, expected one for images.invalid", hosts)
	}
}
