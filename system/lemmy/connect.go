package lemmy

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/mrusme/neonmodem/common"
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
	var password string = ""
	var err *common.PasswordError

	for password == "" {
		password, err = common.SetPassword()
		if err != nil {
			fmt.Printf("%v\n", err.Reason)
		}
	}
	credentials["password"] = password

	if sys.config == nil {
		sys.config = make(map[string]interface{})
	}
	sys.config["url"] = sysURL
	sys.config["credentials"] = credentials

	return nil
}
