package toolchain

import (
	"errors"
	"fireball/core"
	"fmt"
	"runtime"

	"golang.org/x/sys/execabs"
)

func Validate() error {
	defer core.Scope()()

	if _, err := execabs.LookPath("llc"); err != nil {
		return errors.New("failed to find 'llc', you probably don't have LLVM installed")
	}
	if _, err := execabs.LookPath("llvm-as"); err != nil {
		return errors.New("failed to find 'llvm-as', you probably don't have LLVM installed")
	}

	lld := "ld.lld"
	if runtime.GOOS == "darwin" {
		lld = "ld64.lld"
	}
	if _, err := execabs.LookPath(lld); err != nil {
		return fmt.Errorf("failed to find '%s', you probably don't have LLVM installed", lld)
	}

	return nil
}
