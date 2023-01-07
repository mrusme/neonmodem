package main

import (
	"embed"

	"github.com/mrusme/neonmodem/cmd"
)

//go:embed splashscreen.png
var EMBEDFS embed.FS

func main() {
	cmd.Execute(&EMBEDFS)
}
