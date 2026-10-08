package hackernews

import (
	"testing"
	"time"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/systemtest"
)

// TestLive runs against the real Hacker News. The read path runs whenever
// NEONMODEM_LIVE_HN=1 is set. The write path posts one real Ask HN submission
// and one real comment and therefore needs NEONMODEM_LIVE_HN_WRITE=1 as well as
// NEONMODEM_LIVE_HN_USER and NEONMODEM_LIVE_HN_PASS. NEONMODEM_LIVE_HN_ITEM
// comments on an existing item instead of submitting a new one.
//
//	NEONMODEM_LIVE_HN=1 go test ./internal/system/hackernews/ -run TestLive -v
func TestLive(t *testing.T) {
	if systemtest.Env("HN") == "" {
		t.Skip("set NEONMODEM_LIVE_HN=1 to run the live test")
	}

	settings := system.Settings{}
	write := false
	if systemtest.Env("HN_WRITE") == "1" && systemtest.Env("HN_USER") != "" {
		settings.Credentials = map[string]string{
			"username": systemtest.Env("HN_USER"),
			"password": systemtest.Env("HN_PASS"),
		}
		write = true
	}

	sys, err := New(system.Env{Settings: settings})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	systemtest.Exercise(t, sys, systemtest.Options{
		ForumID:         "ask",
		Write:           write,
		ExistingPostID:  systemtest.Env("HN_ITEM"),
		SkipNestedReply: true,
		WriteDelay:      30 * time.Second,
		ReloadAttempts:  6,
		ReloadDelay:     10 * time.Second,
	})
}
