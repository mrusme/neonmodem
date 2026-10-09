package hyperuplink

import (
	"context"
	"io"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/prompt"
	"github.com/mrusme/neonmodem/internal/system/systemtest"
)

func TestLive(t *testing.T) {
	url := systemtest.Env("HUP_URL")
	if url == "" {
		t.Skip("set NEONMODEM_LIVE_HUP_URL to run the live test")
	}

	settings := system.Settings{URL: url}
	write := true
	switch {
	case systemtest.Env("HUP_TOKEN") != "":
		settings.Credentials = map[string]string{system.CredentialToken: systemtest.Env("HUP_TOKEN")}
	case systemtest.Env("HUP_USER") != "":
		settings = liveConnect(t, url, systemtest.Env("HUP_USER"), systemtest.Env("HUP_PASS"))
	default:
		write = false
	}

	sys, err := New(system.Env{Settings: settings})
	if err != nil {
		t.Fatal(err)
	}

	systemtest.Exercise(t, sys, systemtest.Options{
		ForumID: systemtest.Env("HUP_FORUM"),
		Write:   write,
	})
}

func liveConnect(t *testing.T, url string, user string, password string) system.Settings {
	t.Helper()

	sys, err := New(system.Env{})
	if err != nil {
		t.Fatal(err)
	}

	input := "1\n" + user + "\n1\n2\nprint password\n1\n"
	term := prompt.NewTerminal(strings.NewReader(input), io.Discard,
		systemtest.Commands{"print password": password})
	settings, err := sys.Connect(context.Background(), term, url)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	keys := slices.Sorted(maps.Keys(settings.Credentials))
	if !slices.Equal(keys, []string{system.CredentialToken, system.CredentialUsername}) ||
		settings.Credential(system.CredentialToken) == "" {
		t.Fatalf("connect stored the credentials %v", keys)
	}
	return settings
}
