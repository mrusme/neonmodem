package common

import (
	"fmt"
	"syscall"

	"github.com/99designs/keyring"
	"github.com/manifoldco/promptui"
	"golang.org/x/term"
)

type PasswordError struct {
	Reason string
}

func (e *PasswordError) Error() string {
	return fmt.Sprintf("reason: %v", e.Reason)
}

func YesNo() (bool, error) {
	prompt := promptui.Select{
		Label: "Select[Yes/No]",
		Items: []string{"Yes", "No"},
	}

	_, result, err := prompt.Run()
	if err != nil {
		return false, err
	}

	return result == "Yes", nil

}

func SetPassword() (string, *PasswordError) {
	// Prompt for password input
	fmt.Println("Please enter your password (will not echo): ")
	bytepw, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println("")
	if err != nil || len(bytepw) == 0 {
		return "", &PasswordError{Reason: "Invalid input"}
	}

	// Open system keyring object
	ring, _ := keyring.Open(keyring.Config{
		ServiceName: "NeonModem - Lemmy",
	})

	// Attempt to save the password to system keyring
	// If we can't, ask if should save it in clear text
	err = ring.Set(keyring.Item{
		Key:  "password",
		Data: bytepw,
	})
	if err != nil {
		fmt.Println("Unable to save password to a keyring. Would you like to proceed to save the password in clear text in the neonmodem.toml?")
		if resp, _ := YesNo(); resp != true {
			fmt.Println("Not adding lemmy account...")
			return "", &PasswordError{Reason: "Keyring unavailable"}
		} else {
			return string(bytepw), nil
		}
	}

	return "password_in_keyring", nil
}
