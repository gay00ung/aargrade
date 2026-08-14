//go:build windows

package migration

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	migrationMoveFileReplaceExisting = 0x1
	migrationMoveFileWriteThrough    = 0x8
)

var migrationMoveFileExW = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")

func replaceMigrationFile(source, destination string) error {
	sourcePointer, err := syscall.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	destinationPointer, err := syscall.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	result, _, callErr := migrationMoveFileExW.Call(
		uintptr(unsafe.Pointer(sourcePointer)),
		uintptr(unsafe.Pointer(destinationPointer)),
		migrationMoveFileReplaceExisting|migrationMoveFileWriteThrough,
	)
	if result == 0 {
		if callErr != nil && callErr != syscall.Errno(0) {
			return callErr
		}
		return fmt.Errorf("MoveFileExW failed")
	}
	return nil
}
