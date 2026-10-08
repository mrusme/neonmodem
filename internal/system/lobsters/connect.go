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

	web, err := newWebSession(sysURL, username.Value, password.Value, sys.proxy, sys.logger)
	if err != nil {
		return settings, err
	}
	bounded, cancel := system.Bound(ctx, sys.readTimeout)
	defer cancel()
	if err := system.Timeout(ctx, sys.readTimeout, web.Verify(bounded)); err != nil {
		return settings, fmt.Errorf("could not log in to %s: %w", sysURL, err)
	}

	settings.SetCredential(system.CredentialUsername, username)
	settings.SetCredential(system.CredentialPassword, password)

	return settings, nil
}
