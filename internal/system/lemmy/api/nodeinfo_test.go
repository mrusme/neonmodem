package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mrusme/neonmodem/internal/system/httpx"
)

func software(name string, version string) string {
	return fmt.Sprintf(`{"version":"2.1","software":{"name":%q,"version":%q},"protocols":["activitypub"]}`, name, version)
}

func TestVersionFollowsTheMajorVersion(t *testing.T) {
	for version, want := range map[string]Version{
		"0.19.19-9-gc55dd700c":          V3,
		"0.19.21-beta.3":                V3,
		"0.19.21-dotzip.2":              V3,
		"0.19.21":                       V3,
		"0.19.18-2-g28d90e6ca-modified": V3,
		"0.19.21-beta.3-25-gb5838a6d1":  V3,
		"0.19.15":                       V3,
		"1.0.0-beta.2":                  V4,
		"1.0.0-nightly-2026-10-26":      V4,
		"2.0.0":                         V4,
	} {
		got, err := apiVersion("lemmy", version)
		if err != nil || got != want {
			t.Errorf("%s: got %d and %v, want %d", version, got, err, want)
		}
	}
}

func TestVersionReadsTheNodeinfo(t *testing.T) {
	for _, fl := range flavors {
		f, sysURL := newFakeServer(t, fl.version, map[string][]reply{})
		c := newTestClient(t, sysURL, "")

		for range 3 {
			v, err := c.Version(context.Background())
			if err != nil || v != fl.version {
				t.Fatalf("%s: got %d and %v", fl.name, v, err)
			}
		}
		if n := f.nodeinfoRequests(); n != 1 {
			t.Errorf("%s: read the nodeinfo %d times", fl.name, n)
		}
	}
}

func TestVersionRejectsWhatIsNotLemmy(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"other software", http.StatusOK, software("piefed", "1.2.3"),
			"the site runs piefed 1.2.3, not Lemmy"},
		{"no software", http.StatusOK, `{"version":"2.1"}`,
			"the site's nodeinfo doesn't name its software"},
		{"no number", http.StatusOK, software("lemmy", "nightly"),
			`can't tell which API Lemmy "nightly" serves`},
		{"no document", http.StatusNotFound, ``,
			"reading the software version: GET "},
		{"web page", http.StatusOK, `<html>welcome</html>`,
			"reading the software version: GET "},
	} {
		f, sysURL := newFakeServer(t, 0, map[string][]reply{routeNodeinfo: {{tc.status, tc.body}}})
		c := newTestClient(t, sysURL, "")

		_, err := c.Posts(context.Background(), PostQuery{Listing: ListingAll, Limit: 50})
		if err == nil || !strings.HasPrefix(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want %q", tc.name, err, tc.want)
		}
		if requests := f.all(); len(requests) != 0 {
			t.Errorf("%s: made requests %v", tc.name, requests)
		}
	}

	_, sysURL := newFakeServer(t, 0, map[string][]reply{routeNodeinfo: ok(`<html>welcome</html>`)})
	_, err := newTestClient(t, sysURL, "").Version(context.Background())
	if !errors.Is(err, httpx.ErrNotJSON) {
		t.Errorf("a web page gave %v, want ErrNotJSON", err)
	}
}

func TestConcurrentFirstCallsReadTheVersionOnce(t *testing.T) {
	f, sysURL := newFakeServer(t, V4, map[string][]reply{routeV4Posts: ok(fixture(t, "v4/posts.json"))})
	c := newTestClient(t, sysURL, "")

	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Go(func() {
			_, errs[i] = c.Posts(context.Background(), PostQuery{Listing: ListingAll, Limit: 50})
		})
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("call %d: %v", i, err)
		}
	}
	if n := f.nodeinfoRequests(); n != 1 {
		t.Errorf("read the nodeinfo %d times", n)
	}
	if n := f.count(routeV4Posts); n != len(errs) {
		t.Errorf("made %d post requests", n)
	}
}

func TestAFailedReadIsNotKept(t *testing.T) {
	f, sysURL := newFakeServer(t, 0, map[string][]reply{routeNodeinfo: {
		{http.StatusServiceUnavailable, `<html>maintenance</html>`},
		{http.StatusOK, fixture(t, "v3/nodeinfo.json")},
	}})
	c := newTestClient(t, sysURL, "")

	if _, err := c.Version(context.Background()); err == nil {
		t.Fatal("the failed read was accepted")
	}
	v, err := c.Version(context.Background())
	if err != nil || v != V3 {
		t.Fatalf("the second call got %d and %v", v, err)
	}
	if n := f.nodeinfoRequests(); n != 2 {
		t.Errorf("read the nodeinfo %d times", n)
	}
}

func TestAWaitingCallerEndsWithItsDeadline(t *testing.T) {
	release := make(chan struct{})
	reading := make(chan struct{})
	var once sync.Once
	info := fixture(t, "v4/nodeinfo.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(reading) })
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, info)
	}))
	t.Cleanup(srv.Close)
	c := newTestClient(t, srv.URL, "")

	first := make(chan error, 1)
	go func() {
		_, err := c.Version(context.Background())
		first <- err
	}()
	<-reading

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.Version(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("the waiting caller got %v", err)
	}
	if waited := time.Since(start); waited > time.Second {
		t.Errorf("the waiting caller returned after %s", waited)
	}

	close(release)
	if err := <-first; err != nil {
		t.Errorf("the first caller got %v", err)
	}
	if v, err := c.Version(context.Background()); err != nil || v != V4 {
		t.Errorf("got %d and %v after the read", v, err)
	}
}
