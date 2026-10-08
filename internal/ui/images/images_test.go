package images

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := range 4 {
		for x := range 4 {
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

func TestRenderedImagesAreCachedPerURLAndWidth(t *testing.T) {
	data := pngBytes(t)
	base, hits := serve(t, func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(data)
	})

	r := New(nil, "")
	url := base + "/picture.png"
	text := "look: " + url + " done"

	first := r.RenderInline(context.Background(), text, 40)
	if first == text || !strings.Contains(first, "Source: "+url) {
		t.Fatalf("the image was not rendered: %q", first)
	}
	if second := r.RenderInline(context.Background(), text, 40); second != first {
		t.Error("the second render differs from the first")
	}
	if hits.Load() != 1 {
		t.Errorf("the cache should serve the second render, got %d fetches", hits.Load())
	}

	if out := r.RenderInline(context.Background(), text, 20); out == first {
		t.Error("another width renders differently")
	}
	if hits.Load() != 2 {
		t.Errorf("another width is another fetch, got %d fetches", hits.Load())
	}

	if out := r.RenderInline(context.Background(), text, minWidth-1); out != text {
		t.Error("a width below the minimum leaves the text alone")
	}
	if hits.Load() != 2 {
		t.Errorf("a width below the minimum must not fetch, got %d fetches", hits.Load())
	}
}

func TestFailedFetchesLeaveTheURLAndAreNotCached(t *testing.T) {
	missing, missingHits := serve(t, func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusNotFound)
	})
	huge, hugeHits := serve(t, func(w http.ResponseWriter) {
		w.Write(make([]byte, maxBytes+1))
	})
	broken, _ := serve(t, func(w http.ResponseWriter) {
		w.Write([]byte("not an image"))
	})

	r := New(nil, "")
	for _, url := range []string{missing + "/gone.png", huge + "/big.jpg", broken + "/text.jpeg"} {
		if out := r.RenderInline(context.Background(), url, 40); out != url {
			t.Errorf("a failed fetch of %s must leave the URL, got %q", url, out)
		}
	}

	r.RenderInline(context.Background(), missing+"/gone.png", 40)
	r.RenderInline(context.Background(), huge+"/big.jpg", 40)
	if missingHits.Load() != 2 || hugeHits.Load() != 2 {
		t.Errorf("failures must not be cached, got %d and %d fetches",
			missingHits.Load(), hugeHits.Load())
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
	url := srv.URL + "/slow.png"

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if out := r.RenderInline(ctx, url, 40); out != url {
		t.Errorf("a cancelled download must leave the URL, got %q", out)
	}
	if waited := time.Since(start); waited > 3*time.Second {
		t.Errorf("the render waited %s past the cancellation", waited)
	}

	done, cancelDone := context.WithCancel(context.Background())
	cancelDone()
	if out := r.RenderInline(done, url, 40); out != url {
		t.Errorf("a cancelled load must leave the URL, got %q", out)
	}
	if hits.Load() != 1 {
		t.Errorf("a load cancelled before the render must not fetch, got %d fetches", hits.Load())
	}
}

func TestDownloadsGoThroughTheConfiguredProxy(t *testing.T) {
	data := pngBytes(t)
	var hosts []string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		hosts = append(hosts, req.Host)
		w.Header().Set("Content-Type", "image/png")
		w.Write(data)
	}))
	t.Cleanup(proxy.Close)

	r := New(nil, proxy.URL)
	url := "http://images.invalid/picture.png"
	if out := r.RenderInline(context.Background(), url, 40); !strings.Contains(out, "Source: "+url) {
		t.Fatalf("the image was not rendered through the proxy: %q", out)
	}
	if len(hosts) != 1 || hosts[0] != "images.invalid" {
		t.Errorf("the proxy saw the requests %q, expected one for images.invalid", hosts)
	}
}
