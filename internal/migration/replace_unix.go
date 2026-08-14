//go:build !windows

package migration

import "os"

func replaceMigrationFile(source, destination string) error {
	return os.Rename(source, destination)
}
