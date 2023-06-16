package lemmy

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/99designs/keyring"
	"github.com/mrusme/neonmodem/common"
	"golang.org/x/term"
)

func (sys *System) Connect(sysURL string) error {
	// Request input from user
	scanner := bufio.NewScanner(os.Stdin)
	var username string = ""
	for username == "" {
		fmt.Printf(
			"Please enter your username or email: ",
		)
		scanner.Scan()
		username = strings.ReplaceAll(scanner.Text(), " ", "")
		if username == "" {
			fmt.Println("Invalid input")
		}
	}

	// Request input from user
	var password string = ""
	for password == "" {
		fmt.Printf(
			"Please enter your password (will not echo): ",
		)
		bytepw, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println("")
		if err != nil || len(bytepw) == 0 {
			fmt.Println("Invalid input")
		}
		password = string(bytepw)
	}

	// Credentials
	credentials := make(map[string]string)
	credentials["username"] = username

	ring, _ := keyring.Open(keyring.Config{
		ServiceName: "NeonModem - Lemmy",
	})
	// Attempt to save the password to system keyring
	err := ring.Set(keyring.Item{
		Key:  "password",
		Data: []byte(password),
	})
	// On failure, prompt to continue with insecure password storage or abort
	if err != nil {
		fmt.Println("Unable to save password to a keyring. Would you like to proceed to save the password in clear text in the neonmodem.toml?")
		if resp, _ := common.YesNo(); resp != true {
			fmt.Println("Not adding lemmy account...")
			os.Exit(0)
		} else {
			credentials["password"] = password
		}
	} else {
		credentials["password"] = "password_in_keyring"
	}

	if sys.config == nil {
		sys.config = make(map[string]interface{})
	}
	sys.config["url"] = sysURL
	sys.config["credentials"] = credentials

	return nil
}
