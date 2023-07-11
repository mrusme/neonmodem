package lemmy

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/crypto/bcrypt"
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

		hashed, _ := bcrypt.GenerateFromPassword([]byte(bytepw), 8)

		password = string(hashed)
	}

	// Credentials
	credentials := make(map[string]string)
	credentials["username"] = username
	credentials["password"] = password

	if sys.config == nil {
		sys.config = make(map[string]interface{})
	}
	sys.config["url"] = sysURL
	sys.config["credentials"] = credentials

	return nil
}
