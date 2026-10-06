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

	web := newWebSession(api.SiteURL, username.Value, password.Value, sys.proxy, sys.logger)
	if err := web.Verify(ctx); err != nil {
		return settings, fmt.Errorf("could not log in to Hacker News: %w", err)
	}

	settings.SetCredential(system.CredentialUsername, username)
	settings.SetCredential(system.CredentialPassword, password)

	return settings, nil
}
