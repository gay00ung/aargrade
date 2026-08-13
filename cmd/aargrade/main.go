package main

import (
	"os"
	"runtime/debug"

	"github.com/gay00ung/aargrade/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, resolvedVersion()))
}

func resolvedVersion() string {
	moduleVersion := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		moduleVersion = info.Main.Version
	}
	return selectVersion(version, moduleVersion)
}

func selectVersion(linkedVersion, moduleVersion string) string {
	if linkedVersion != "" && linkedVersion != "dev" {
		return linkedVersion
	}
	if moduleVersion != "" && moduleVersion != "(devel)" {
		return moduleVersion
	}
	if linkedVersion != "" {
		return linkedVersion
	}
	return "dev"
}
