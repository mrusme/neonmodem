package hyperuplink

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/hyperuplink/api"
	"github.com/mrusme/neonmodem/internal/system/prompt"
)

func (sys *System) Connect(
	ctx context.Context,
	p prompt.Prompter,
	sysURL string,
) (system.Settings, error) {
	settings := system.Settings{URL: sysURL}

	username, err := p.Credential(ctx, prompt.Field{
		Name:     "username",
		Question: "Please enter your username",
	})
	if err != nil {
		return settings, err
	}

	token, err := p.Credential(ctx, prompt.Field{
		Name:     "API token",
		Question: "Please enter your API token",
		Secret:   true,
	})
	if err != nil {
		return settings, err
	}
	if token.Command == "" {
		token.Value = strings.TrimSpace(token.Value)
	}
	if token.Value == "" {
		return settings, errors.New("no API token was entered")
	}

	client, err := newClient(sysURL, token.Value, sys.proxy, sys.logger)
	if err != nil {
		return settings, err
	}

	bounded, cancel := system.Bound(ctx, sys.readTimeout)
	defer cancel()
	session, err := client.Whoami(bounded)
	err = system.Timeout(ctx, sys.readTimeout, err)
	if err != nil {
		if errors.Is(err, api.ErrNotAnAPI) {
			return settings, fmt.Errorf(
				"%s doesn't look quite right: %w. Please make sure you're "+
					"using the address of the Hyperuplink API, which listens on a "+
					"different port than the web interface, port 3001 by default",
				sysURL, err,
			)
		}
		return settings, fmt.Errorf("could not authenticate against %s: %w", sysURL, err)
	}

	if session.User.Username != username.Value {
		if username.Command != "" {
			return settings, fmt.Errorf(
				"the API token belongs to '%s', but the username command printed '%s'",
				session.User.Username, username.Value,
			)
		}
		p.Notice(fmt.Sprintf(
			"Note: this token belongs to '%s', not '%s'; using '%s'.",
			session.User.Username, username.Value, session.User.Username,
		))
		username.Value = session.User.Username
	}

	settings.SetCredential(system.CredentialUsername, username)
	settings.SetCredential(system.CredentialToken, token)

	return settings, nil
}
