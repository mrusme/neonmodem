package lemmy

import (
	"context"
	"fmt"
	"strings"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/httpx"
	"github.com/mrusme/neonmodem/internal/system/prompt"
	"go.elara.ws/go-lemmy"
)

func (sys *System) Connect(
	ctx context.Context,
	p prompt.Prompter,
	sysURL string,
) (system.Settings, error) {
	settings := system.Settings{URL: sysURL, Options: map[string]string{}}

	username, err := p.Optional(
		"Please enter your username or email (press Enter for read-only access without an account)",
	)
	if err != nil {
		return settings, err
	}

	if username != "" {
		password, err := p.Secret("Please enter your password", "password")
		if err != nil {
			return settings, err
		}

		client, err := lemmy.NewWithClient(sysURL, httpx.NewHTTPClient(httpx.Options{}))
		if err != nil {
			return settings, err
		}
		if err := client.ClientLogin(ctx, lemmy.Login{
			UsernameOrEmail: username,
			Password:        password,
		}); err != nil {
			return settings, fmt.Errorf("could not log in to %s: %w", sysURL, err)
		}

		settings.Credentials = map[string]string{
			"username": username,
			"password": password,
		}
	} else {
		p.Notice("Connecting without an account; posting and replying will not be available.")
	}

	listing, err := p.Optional(
		"Which feed should be listed, subscribed, local or all (press Enter for subscribed)",
	)
	if err != nil {
		return settings, err
	}
	listing = strings.ToLower(strings.TrimSpace(listing))
	switch listing {
	case "":
		listing = ListingSubscribed
	case ListingSubscribed, ListingLocal, ListingAll:
	default:
		return settings, fmt.Errorf(
			"%q is not a valid feed; use subscribed, local or all", listing)
	}
	if listing == ListingSubscribed && username == "" {
		p.Notice("The subscribed feed needs an account; the local feed will be listed instead.")
	}
	settings.Options[OptionListing] = listing

	return settings, nil
}
