package hackernews

import (
	"context"
	"fmt"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/hackernews/api"
	"github.com/mrusme/neonmodem/internal/system/prompt"
)

func (sys *System) Connect(
	ctx context.Context,
	p prompt.Prompter,
	sysURL string,
) (system.Settings, error) {
	settings := system.Settings{URL: api.SiteURL}

	username, err := p.Optional(
		"Please enter your Hacker News username (press Enter for read-only access without an account)",
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

	web := newWebSession(api.SiteURL, username, password, sys.proxy, sys.logger)
	if err := web.Verify(ctx); err != nil {
		return settings, fmt.Errorf("could not log in to Hacker News: %w", err)
	}

	settings.Credentials = map[string]string{
		"username": username,
		"password": password,
	}

	return settings, nil
}
