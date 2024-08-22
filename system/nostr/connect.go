package nostr

import (
	"fmt"
	"syscall"

	"golang.org/x/term"
)

type UserAPIKey struct {
	Key   string `json:"key"`
	Nonce string `json:"nonce"`
	Push  bool   `json:"push"`
	API   int    `json:"api"`
}

func (sys *System) Connect(sysURL string) error {
	// Request input from user
	var pk string = ""
	for pk == "" {
		fmt.Printf(
			"Please enter your pk (will not echo): ",
		)
		bytepw, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println("")
		if err != nil || len(bytepw) == 0 {
			fmt.Println("Invalid input")
		}
		pk = string(bytepw)
	}

	// Credentials
	credentials := make(map[string]string)
	credentials["pk"] = pk

	if sys.config == nil {
		sys.config = make(map[string]interface{})
	}
	sys.config["url"] = sysURL
	sys.config["credentials"] = credentials

	return nil
}
