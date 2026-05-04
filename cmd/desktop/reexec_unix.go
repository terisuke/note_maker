//go:build !windows

package main

import (
	"fmt"
	"os"
	"syscall"
)

func reexecWithCurrentEnv() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable for desktop bootstrap: %w", err)
	}
	return syscall.Exec(executable, os.Args, os.Environ())
}
