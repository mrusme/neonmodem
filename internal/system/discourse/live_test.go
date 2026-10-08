package discourse

import (
	"testing"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/systemtest"
)

func TestLive(t *testing.T) {
	url := systemtest.Env("DISCOURSE_URL")
	if url == "" {
		t.Skip("set NEONMODEM_LIVE_DISCOURSE_URL to run the live test")
	}

	settings := system.Settings{URL: url}
	write := false
	if key := systemtest.Env("DISCOURSE_KEY"); key != "" {
		settings.Credentials = map[string]string{
			"key":       key,
			"client_id": systemtest.Env("DISCOURSE_CLIENT_ID"),
			"username":  systemtest.Env("DISCOURSE_USER"),
		}
		write = true
	}

	sys, err := New(system.Env{Settings: settings})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	systemtest.Exercise(t, sys, systemtest.Options{
		ForumID: systemtest.Env("DISCOURSE_CATEGORY"),
		Write:   write,
	})
}
