package lobsters

import (
	"context"
	"fmt"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/prompt"
)

func (sys *System) Connect(
	ctx context.Context,
	p prompt.Prompter,
	sysURL string,
) (system.Settings, error) {
	settings := system.Settings{URL: sysURL}

	username, err := p.Optional(
		"Please enter your username or email (press Enter for read-only access without an account)",
	)
	if err != nil {
		return settings, err
	}
	if username == "" {
		p.Notice("Connecting without an account; posting and replying will not be available.")
		return settings, nil
	}

	password, err := p.Secret("Please enter your password", "password")
	if err != nil {
		return settings, err
	}

	web, err := newWebSession(sysURL, username, password, sys.proxy, sys.logger)
	if err != nil {
		return settings, err
	}
	if err := web.Verify(ctx); err != nil {
		return settings, fmt.Errorf("could not log in to %s: %w", sysURL, err)
	}

	settings.Credentials = map[string]string{
		"username": username,
		"password": password,
	}

	return settings, nil
}
