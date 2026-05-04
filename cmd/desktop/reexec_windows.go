//go:build windows

package main

import (
	"fmt"
	"os"
)

func reexecWithCurrentEnv() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable for desktop bootstrap: %w", err)
	}
	process, err := os.StartProcess(executable, os.Args, &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		Env:   os.Environ(),
	})
	if err != nil {
		return fmt.Errorf("restart desktop process with bootstrapped environment: %w", err)
	}
	_ = process.Release()
	os.Exit(0)
	return nil
}
