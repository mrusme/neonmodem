package lemmy

import (
	"context"
	"fmt"
	"strings"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/prompt"
	"go.elara.ws/go-lemmy"
)

func (sys *System) Connect(
	ctx context.Context,
	p prompt.Prompter,
	sysURL string,
) (system.Settings, error) {
	settings := system.Settings{URL: sysURL, Options: map[string]string{}}

	username, err := p.Credential(ctx, prompt.Field{
		Name:      "username or email",
		Question:  "Please enter your username or email",
		NoAccount: true,
	})
	if err != nil {
		return settings, err
	}

	if username.NoAccount {
		p.Notice("Connecting without an account; posting and replying will not be available.")
	} else {
		password, err := p.Credential(ctx, prompt.Field{
			Name:     "password",
			Question: "Please enter your password",
			Secret:   true,
		})
		if err != nil {
			return settings, err
		}

		client, err := newClient(sysURL, sys.proxy, sys.logger)
		if err != nil {
			return settings, err
		}
		if err := client.ClientLogin(ctx, lemmy.Login{
			UsernameOrEmail: username.Value,
			Password:        password.Value,
		}); err != nil {
			return settings, fmt.Errorf("could not log in to %s: %w", sysURL, err)
		}

		settings.SetCredential(system.CredentialUsername, username)
		settings.SetCredential(system.CredentialPassword, password)
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
	if listing == ListingSubscribed && username.NoAccount {
		p.Notice("The subscribed feed needs an account; the local feed will be listed instead.")
	}
	settings.Options[OptionListing] = listing

	return settings, nil
}
