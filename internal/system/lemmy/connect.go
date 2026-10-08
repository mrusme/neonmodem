package lemmy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/prompt"
	"go.elara.ws/go-lemmy"
)

const (
	codeDigits = 6
	maxCodes   = 3
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
		jwt, err := logIn(ctx, sys.readTimeout, client, p, lemmy.Login{
			UsernameOrEmail: username.Value,
			Password:        password.Value,
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

func logIn(
	ctx context.Context,
	timeout time.Duration,
	client *lemmy.Client,
	p prompt.Prompter,
	login lemmy.Login,
) (string, error) {
	resp, err := login1(ctx, timeout, client, login)
	if errorString(err) == "missing_totp_token" {
		p.Notice("Your account uses two-factor authentication.")
		resp, err = logInWithCode(ctx, timeout, client, p, login)
	}
	if errorString(err) == "rate_limit_error" {
		return "", errTooManyLogins
	}
	if err != nil {
		return "", err
	}

	jwt, ok := resp.JWT.Value()
	if !ok || jwt == "" {
		return "", lemmy.ErrNoToken
	}
	return jwt, nil
}

func login1(
	ctx context.Context,
	timeout time.Duration,
	client *lemmy.Client,
	login lemmy.Login,
) (*lemmy.LoginResponse, error) {
	bounded, cancel := system.Bound(ctx, timeout)
	defer cancel()

	resp, err := client.Login(bounded, login)
	return resp, system.Timeout(ctx, timeout, err)
}

func logInWithCode(
	ctx context.Context,
	timeout time.Duration,
	client *lemmy.Client,
	p prompt.Prompter,
	login lemmy.Login,
) (*lemmy.LoginResponse, error) {
	for rejected := 0; ; {
		code, err := readCode(p)
		if err != nil {
			return nil, err
		}

		login.TOTP2FAToken = lemmy.NewOptional(code)
		resp, err := login1(ctx, timeout, client, login)
		if errorString(err) != "incorrect_totp_token" {
			return resp, err
		}

		rejected++
		if rejected == maxCodes {
			return nil, errors.New("three codes were rejected; check the clock of " +
				"the device with your authenticator app")
		}
		p.Notice("The code was rejected. Codes change every 30 seconds, " +
			"and a device with a wrong clock shows wrong codes.")
	}
}

func readCode(p prompt.Prompter) (string, error) {
	for {
		answer, err := p.Line(
			"Please enter the current 2FA code from your authenticator app", "2FA code")
		if err != nil {
			return "", err
		}

		code := strings.Join(strings.Fields(answer), "")
		if isCode(code) {
			return code, nil
		}
		p.Notice("A 2FA code has six digits.")
	}
}

func isCode(code string) bool {
	if len(code) != codeDigits {
		return false
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
