package lemmy

import (
	"context"
	"io"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/prompt"
	"github.com/mrusme/neonmodem/internal/system/systemtest"
)

func TestLive(t *testing.T) {
	url := systemtest.Env("LEMMY_URL")
	if url == "" {
		t.Skip("set NEONMODEM_LIVE_LEMMY_URL to run the live test")
	}

	settings := system.Settings{
		URL:     url,
		Options: map[string]string{OptionListing: systemtest.Env("LEMMY_LISTING")},
	}
	write := true
	switch {
	case systemtest.Env("LEMMY_TOKEN") != "":
		settings.Credentials = map[string]string{
			system.CredentialToken: systemtest.Env("LEMMY_TOKEN"),
		}
		if user := systemtest.Env("LEMMY_USER"); user != "" {
			settings.Credentials[system.CredentialUsername] = user
		}
	case systemtest.Env("LEMMY_USER") != "":
		settings = liveConnect(t, url, systemtest.Env("LEMMY_USER"), systemtest.Env("LEMMY_PASS"), "")
	default:
		write = false
	}

	exercise(t, settings, write)
}

func TestLiveTwoFactor(t *testing.T) {
	url := systemtest.Env("LEMMY_URL")
	user := systemtest.Env("LEMMY_2FA_USER")
	if url == "" || user == "" {
		t.Skip("set NEONMODEM_LIVE_LEMMY_URL and NEONMODEM_LIVE_LEMMY_2FA_USER to run the live test with 2FA")
	}

	settings := liveConnect(t, url, user,
		systemtest.Env("LEMMY_2FA_PASS"), systemtest.Env("LEMMY_2FA_TOTP"))
	exercise(t, settings, true)
}

func liveConnect(t *testing.T, url string, user string, password string, secret string) system.Settings {
	t.Helper()

	sys, err := New(system.Env{})
	if err != nil {
		t.Fatal(err)
	}

	input := "1\n" + user + "\n2\nprint password\n"
	if secret != "" {
		code, err := systemtest.TOTP(secret, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		input += code + "\n"
	}
	input += "1\n" + systemtest.Env("LEMMY_LISTING") + "\n"

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

func exercise(t *testing.T, settings system.Settings, write bool) {
	t.Helper()

	sys, err := New(system.Env{Settings: settings})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	systemtest.Exercise(t, sys, systemtest.Options{Write: write})
}
