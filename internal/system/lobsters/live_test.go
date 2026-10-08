package lobsters

import (
	"testing"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/systemtest"
)

func TestLive(t *testing.T) {
	url := systemtest.Env("LOBSTERS_URL")
	if url == "" {
		t.Skip("set NEONMODEM_LIVE_LOBSTERS_URL to run the live test")
	}

	settings := system.Settings{URL: url}
	write := false
	if user := systemtest.Env("LOBSTERS_USER"); user != "" {
		settings.Credentials = map[string]string{
			"username": user,
			"password": systemtest.Env("LOBSTERS_PASS"),
		}
		write = true
	}

	sys, err := New(system.Env{Settings: settings})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	systemtest.Exercise(t, sys, systemtest.Options{
		ForumID: systemtest.Env("LOBSTERS_TAG"),
		Write:   write,
	})
}
