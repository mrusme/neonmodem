package main

import (
	"embed"

	"github.com/mrusme/gobbs/cmd"
)

//go:embed splashscreen.png
var EMBEDFS embed.FS

func main() {
	cmd.Execute(&EMBEDFS)
}
