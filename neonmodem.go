package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/mrusme/neonmodem/cmd"
)

//go:embed splashscreen.png
var EMBEDFS embed.FS

func main() {
	if err := cmd.Execute(&EMBEDFS); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
