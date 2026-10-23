// Command kiln is a git-trailer driven problem tracker and path rebuilder.
package main

import (
	"os"

	"github.com/jackiehank/kiln/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
