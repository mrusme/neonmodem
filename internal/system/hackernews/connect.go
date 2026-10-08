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
	return sys.connect(ctx, p, api.SiteURL)
}

func (sys *System) connect(
	ctx context.Context,
	p prompt.Prompter,
	siteURL string,
) (system.Settings, error) {
	settings := system.Settings{URL: siteURL}

	username, err := p.Credential(ctx, prompt.Field{
		Name:      "Hacker News username",
		Question:  "Please enter your Hacker News username",
		NoAccount: true,
	})
	if err != nil {
		return settings, err
	}
	if username.NoAccount {
		p.Notice("Connecting without an account; posting and replying will not be available.")
		return settings, nil
	}

	password, err := p.Credential(ctx, prompt.Field{
		Name:     "password",
		Question: "Please enter your password",
		Secret:   true,
	})
	if err != nil {
		return settings, err
	}

	web, err := newWebSession(siteURL, username.Value, password.Value, sys.proxy, sys.logger)
	if err != nil {
		return settings, err
	}
	bounded, cancel := system.Bound(ctx, sys.readTimeout)
	defer cancel()
	if err := system.Timeout(ctx, sys.readTimeout, web.Verify(bounded)); err != nil {
		return settings, fmt.Errorf("could not log in to Hacker News: %w", err)
	}

	settings.SetCredential(system.CredentialUsername, username)
	settings.SetCredential(system.CredentialPassword, password)

	return settings, nil
}
