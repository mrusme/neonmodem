package hyperuplink

import (
	"context"
	"net/http"
	"testing"

	"github.com/mrusme/neonmodem/internal/system"
)

func TestImagesOnTheBoardGetTheKey(t *testing.T) {
	for _, c := range []struct {
		origin string
		token  string
		url    string
		want   string
	}{
		{"https://board.example:3001", "hup_test", "https://board.example:3001/api/v1/attachments/a1", "Bearer hup_test"},
		{"https://board.example:3001", "hup_test", "HTTPS://BOARD.EXAMPLE:3001/x.png", "Bearer hup_test"},
		{"https://board.example", "hup_test", "https://board.example:443/api/v1/attachments/a1", "Bearer hup_test"},
		{"http://localhost:3001/api/v1", "hup_test", "http://localhost:3001/api/v1/attachments/a1", "Bearer hup_test"},
		{"https://board.example:3001", "hup_test", "https://board.example/api/v1/attachments/a1", ""},
		{"https://board.example:3001", "hup_test", "http://board.example:3001/api/v1/attachments/a1", ""},
		{"https://board.example:3001", "hup_test", "https://images.example:3001/a.png", ""},
		{"https://board.example:3001", "", "https://board.example:3001/api/v1/attachments/a1", ""},
	} {
		settings := system.Settings{URL: c.origin}
		if c.token != "" {
			settings.Credentials = map[string]string{system.CredentialToken: c.token}
		}
		sys, err := New(system.Env{Settings: settings})
		if err != nil {
			t.Fatal(err)
		}
		authorizer, ok := sys.(system.MediaAuthorizer)
		if !ok {
			t.Fatal("the Hyperuplink system authorizes no media")
		}

		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, c.url, nil)
		if err != nil {
			t.Fatal(err)
		}
		authorizer.AuthorizeMedia(req)
		if got := req.Header.Get("Authorization"); got != c.want {
			t.Errorf("%s with origin %s and key %q got %q, want %q", c.url, c.origin, c.token, got, c.want)
		}
	}
}
