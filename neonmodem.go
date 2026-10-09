package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/mrusme/neonmodem/cmd"
	"github.com/mrusme/neonmodem/internal/system/text"
)

//go:embed splashscreen.png
var EMBEDFS embed.FS

func main() {
	if err := cmd.Execute(&EMBEDFS); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", text.PrintableLines(err.Error()))
		os.Exit(1)
	}
}
