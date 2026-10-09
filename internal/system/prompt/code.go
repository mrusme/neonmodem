package prompt

import (
	"errors"
	"strings"
)

const (
	codeDigits = 6
	MaxCodes   = 3

	CodeRejected = "The code was rejected. Codes change every 30 seconds, " +
		"and a device with a wrong clock shows wrong codes."
)

var ErrCodesRejected = errors.New("three codes were rejected; check the clock of " +
	"the device with your authenticator app")

func Code(p Prompter) (string, error) {
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
