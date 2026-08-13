//go:build !windows

package host

import "os"

func replaceFile(source, destination string) error {
	return os.Rename(source, destination)
}
