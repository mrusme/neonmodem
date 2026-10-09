package hyperuplink

import (
	"net/url"
	"strings"

	"github.com/mrusme/neonmodem/internal/system/hyperuplink/api"
)

func apiOrigin(raw string) (string, bool) {
	origin := strings.TrimRight(strings.TrimSpace(raw), "/")
	for _, suffix := range []string{api.Prefix, "/api"} {
		if base, found := strings.CutSuffix(origin, suffix); found {
			return strings.TrimRight(base, "/"), true
		}
	}
	return origin, false
}

func sameOrigin(u *url.URL, origin string) bool {
	o, err := url.Parse(origin)
	if err != nil || u == nil {
		return false
	}
	return strings.EqualFold(u.Scheme, o.Scheme) &&
		strings.EqualFold(u.Hostname(), o.Hostname()) &&
		port(u) == port(o)
}

func port(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	switch strings.ToLower(u.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	}
	return ""
}
