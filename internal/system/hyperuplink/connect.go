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

	username, err := p.Line("Please enter your username", "username")
	if err != nil {
		return settings, err
	}

	secret, err := p.Secret("Please enter your API token", "API token")
	if err != nil {
		return settings, err
	}

	token := strings.TrimSpace(secret)
	if token == "" {
		return settings, errors.New("no API token was entered")
	}

	client, err := newClient(sysURL, token, sys.proxy, sys.logger)
	if err != nil {
		return settings, err
	}

	session, err := client.Session.Whoami(ctx)
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

	if session.User.Username != username {
		p.Notice(fmt.Sprintf(
			"Note: this token belongs to '%s', not '%s'; using '%s'.",
			session.User.Username, username, session.User.Username,
		))
		username = session.User.Username
	}

	settings.Credentials = map[string]string{
		"username": username,
		"token":    token,
	}

	return settings, nil
}
