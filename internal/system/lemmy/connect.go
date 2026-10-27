package lemmy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/lemmy/api"
	"github.com/mrusme/neonmodem/internal/system/prompt"
)

var errTooManyLogins = errors.New(
	"too many logins from this IP address; please try again later, since Lemmy " +
		"limits logins per hour")

func (sys *System) Connect(
	ctx context.Context,
	p prompt.Prompter,
	sysURL string,
) (system.Settings, error) {
	settings := system.Settings{URL: sysURL, Options: map[string]string{}}

	client, err := newClient(sysURL, "", sys.proxy, sys.logger, 0)
	if err != nil {
		return settings, err
	}
	if err := readVersion(ctx, sys.readTimeout, client); err != nil {
		return settings, fmt.Errorf("could not connect to %s: %w", sysURL, err)
	}

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

		jwt, err := logIn(ctx, sys.readTimeout, client, p, api.Login{
			Username: username.Value,
			Password: password.Value,
		})
		if err != nil {
			return settings, fmt.Errorf("could not log in to %s: %w", sysURL, err)
		}
		p.Notice("Logged in. Your password won't be stored, only the session token Lemmy issued.")

		token, err := p.Generated(ctx, prompt.Field{Name: "session token", Secret: true}, jwt)
		if err != nil {
			return settings, err
		}

		settings.SetCredential(system.CredentialUsername, username)
		settings.SetCredential(system.CredentialToken, token)
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

func readVersion(ctx context.Context, timeout time.Duration, client *api.Client) error {
	bounded, cancel := system.Bound(ctx, timeout)
	defer cancel()

	_, err := client.Version(bounded)
	return system.Timeout(ctx, timeout, err)
}

func logIn(
	ctx context.Context,
	timeout time.Duration,
	client *api.Client,
	p prompt.Prompter,
	login api.Login,
) (string, error) {
	jwt, err := login1(ctx, timeout, client, login)
	if errors.Is(err, api.ErrCodeRequired) {
		p.Notice("Your account uses two-factor authentication.")
		jwt, err = logInWithCode(ctx, timeout, client, p, login)
	}
	if errors.Is(err, api.ErrRateLimited) {
		return "", errTooManyLogins
	}
	return jwt, err
}

func login1(
	ctx context.Context,
	timeout time.Duration,
	client *api.Client,
	login api.Login,
) (string, error) {
	bounded, cancel := system.Bound(ctx, timeout)
	defer cancel()

	jwt, err := client.Login(bounded, login)
	return jwt, system.Timeout(ctx, timeout, err)
}

func logInWithCode(
	ctx context.Context,
	timeout time.Duration,
	client *api.Client,
	p prompt.Prompter,
	login api.Login,
) (string, error) {
	for rejected := 0; ; {
		code, err := prompt.Code(p)
		if err != nil {
			return "", err
		}

		login.Code = code
		jwt, err := login1(ctx, timeout, client, login)
		if !errors.Is(err, api.ErrCodeIncorrect) {
			return jwt, err
		}

		rejected++
		if rejected == prompt.MaxCodes {
			return "", prompt.ErrCodesRejected
		}
		p.Notice(prompt.CodeRejected)
	}
}
