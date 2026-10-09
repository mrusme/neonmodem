package hyperuplink

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mrusme/neonmodem/internal/system"
	"github.com/mrusme/neonmodem/internal/system/hyperuplink/api"
	"github.com/mrusme/neonmodem/internal/system/prompt"
)

const (
	keyNameMax = 64

	signInWithPassword = 0
)

var (
	errWrongCredentials = errors.New("the username or password is wrong")
	errTooManySignIns   = errors.New(
		"too many sign-ins from this address; Hyperuplink allows ten per minute")
)

func (sys *System) Connect(
	ctx context.Context,
	p prompt.Prompter,
	sysURL string,
) (system.Settings, error) {
	origin, trimmed := apiOrigin(sysURL)
	settings := system.Settings{URL: origin}
	if trimmed {
		p.Notice(fmt.Sprintf("Using %s; neonmodem adds %s itself.", origin, api.Prefix))
	}

	probe, err := newClient(origin, "", sys.proxy, sys.logger, 0)
	if err != nil {
		return settings, err
	}
	board, err := sys.discover(ctx, probe, origin)
	if err != nil {
		return settings, err
	}

	notice := fmt.Sprintf("Connecting to %q (Hyperuplink %s).", board.Name, board.Version)
	if board.Version == "" {
		notice = fmt.Sprintf("Connecting to %q (Hyperuplink).", board.Name)
	}
	if !board.Guests {
		notice += " The board doesn't serve guests, so an account is needed."
	}
	p.Notice(notice)

	username, err := p.Credential(ctx, prompt.Field{
		Name:      "username",
		Question:  "Please enter your username",
		NoAccount: board.Guests,
	})
	if err != nil {
		return settings, err
	}
	if username.NoAccount {
		p.Notice("Connecting without an account; posting and replying will not be available.")
		return settings, nil
	}

	choice, err := p.Choose("How do you want to sign in?", []string{
		"With your password; the board issues an API key for neonmodem",
		"With an API key issued on the board",
	})
	if err != nil {
		return settings, err
	}

	var token prompt.Answer
	if choice == signInWithPassword {
		token, username, err = sys.signIn(ctx, p, probe, origin, username)
	} else {
		token, username, err = sys.verifyKey(ctx, p, origin, username)
	}
	if err != nil {
		return settings, err
	}

	settings.SetCredential(system.CredentialUsername, username)
	settings.SetCredential(system.CredentialToken, token)

	return settings, nil
}

func (sys *System) discover(ctx context.Context, client *api.Client, origin string) (*api.Discovery, error) {
	bounded, cancel := system.Bound(ctx, sys.readTimeout)
	defer cancel()

	board, err := client.Discover(bounded)
	err = system.Timeout(ctx, sys.readTimeout, err)
	if err != nil {
		if errors.Is(err, api.ErrNotAnAPI) {
			return nil, fmt.Errorf(
				"%s doesn't look quite right: %w. Please make sure you're using the "+
					"address of the Hyperuplink API: its own port, 3001 by default, or "+
					"the address a reverse proxy forwards /api/ to",
				origin, err,
			)
		}
		return nil, fmt.Errorf("could not reach the Hyperuplink API at %s: %w", origin, err)
	}

	if board.API == "" {
		return nil, fmt.Errorf("%s is not a Hyperuplink API", origin)
	}
	if board.API != api.Version {
		return nil, fmt.Errorf("%s serves API version %s; this version of neonmodem speaks %s",
			origin, board.API, api.Version)
	}

	return board, nil
}

func (sys *System) signIn(
	ctx context.Context,
	p prompt.Prompter,
	client *api.Client,
	origin string,
	username prompt.Answer,
) (prompt.Answer, prompt.Answer, error) {
	password, err := p.Credential(ctx, prompt.Field{
		Name:     "password",
		Question: "Please enter your password",
		Secret:   true,
	})
	if err != nil {
		return prompt.Answer{}, username, err
	}

	in := &api.SignInInput{
		Username: username.Value,
		Password: password.Value,
		Name:     keyName(),
	}
	resp, err := sys.exchange(ctx, client, in)
	if codeOf(err) == api.CodeOTPRequired {
		p.Notice("Your account uses two-factor authentication.")
		resp, err = sys.exchangeWithCode(ctx, client, p, in)
	}
	if err != nil {
		return prompt.Answer{}, username,
			fmt.Errorf("could not sign in to %s: %w", origin, signInError(err))
	}
	p.Notice("Signed in. Your password won't be stored, only the API key the board issued.")

	token, err := p.Generated(ctx, prompt.Field{Name: "API key", Secret: true}, resp.Token)
	if err != nil {
		return prompt.Answer{}, username, err
	}

	if username.Command == "" && resp.User.Username != "" {
		username.Value = resp.User.Username
	}

	return token, username, nil
}

func (sys *System) exchange(
	ctx context.Context,
	client *api.Client,
	in *api.SignInInput,
) (*api.SignInResponse, error) {
	bounded, cancel := system.Bound(ctx, sys.readTimeout)
	defer cancel()

	resp, err := client.SignIn(bounded, in)
	return resp, system.Timeout(ctx, sys.readTimeout, err)
}

func (sys *System) exchangeWithCode(
	ctx context.Context,
	client *api.Client,
	p prompt.Prompter,
	in *api.SignInInput,
) (*api.SignInResponse, error) {
	for rejected := 0; ; {
		code, err := prompt.Code(p)
		if err != nil {
			return nil, err
		}

		in.OTPCode = code
		resp, err := sys.exchange(ctx, client, in)
		if codeOf(err) != api.CodeOTPCodeWrong {
			return resp, err
		}

		rejected++
		if rejected == prompt.MaxCodes {
			return nil, prompt.ErrCodesRejected
		}
		p.Notice(prompt.CodeRejected)
	}
}

func (sys *System) verifyKey(
	ctx context.Context,
	p prompt.Prompter,
	origin string,
	username prompt.Answer,
) (prompt.Answer, prompt.Answer, error) {
	token, err := p.Credential(ctx, prompt.Field{
		Name:     "API key",
		Question: "Please enter your API key",
		Secret:   true,
	})
	if err != nil {
		return token, username, err
	}
	if token.Command == "" {
		token.Value = strings.TrimSpace(token.Value)
	}
	if token.Value == "" {
		return token, username, errors.New("no API key was entered")
	}

	client, err := newClient(origin, token.Value, sys.proxy, sys.logger, 0)
	if err != nil {
		return token, username, err
	}

	bounded, cancel := system.Bound(ctx, sys.readTimeout)
	defer cancel()
	session, err := client.Whoami(bounded)
	err = system.Timeout(ctx, sys.readTimeout, err)
	if err != nil {
		return token, username, fmt.Errorf("could not authenticate against %s: %w", origin, err)
	}
	if !session.Authenticated || session.User == nil {
		return token, username, fmt.Errorf("%s didn't accept the API key", origin)
	}

	owner := session.User.Username
	if owner != username.Value {
		if username.Command != "" {
			return token, username, fmt.Errorf(
				"the API key belongs to '%s', but the username command printed '%s'",
				owner, username.Value,
			)
		}
		p.Notice(fmt.Sprintf(
			"Note: this API key belongs to '%s', not '%s'; using '%s'.",
			owner, username.Value, owner,
		))
		username.Value = owner
	}

	return token, username, nil
}

func codeOf(err error) string {
	problem, ok := api.ProblemOf(err)
	if !ok {
		return ""
	}
	return problem.Code
}

func signInError(err error) error {
	switch codeOf(err) {
	case api.CodeUsernamePasswordWrong:
		return errWrongCredentials
	case api.CodeRateLimited:
		return errTooManySignIns
	}
	return err
}

func keyName() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		return "neonmodem"
	}

	name := []rune("neonmodem on " + host)
	if len(name) > keyNameMax {
		name = name[:keyNameMax]
	}
	return string(name)
}
