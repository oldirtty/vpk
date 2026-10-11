package main

import (
	"os"

	"github.com/oldirtty/vpk/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
