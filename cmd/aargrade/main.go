package main

import (
	"os"

	"github.com/gay00ung/aargrade/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, version))
}
