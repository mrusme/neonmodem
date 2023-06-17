package lemmy

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

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

	// Credentials
	credentials := make(map[string]string)
	credentials["username"] = username

	// New password code
	for {
		fmt.Println("Please enter your password (will not echo): ")
		bytepw, err := term.ReadPassword(int(syscall.Stdin))
		password, err := common.SetPassword(bytepw)
		if err == nil {
			credentials["password"] = password
			break
		}
	}

	if sys.config == nil {
		sys.config = make(map[string]interface{})
	}
	sys.config["url"] = sysURL
	sys.config["credentials"] = credentials

	return nil
}
