package toolchain

import (
	"errors"

	"golang.org/x/sys/execabs"
)

func Validate() error {
	if _, err := execabs.LookPath("llc"); err != nil {
		return errors.New("failed to find 'llc', you probably don't have LLVM installed")
	}
	if _, err := execabs.LookPath("llvm-as"); err != nil {
		return errors.New("failed to find 'llvm-as', you probably don't have LLVM installed")
	}
	if _, err := execabs.LookPath("ld.lld"); err != nil {
		return errors.New("failed to find 'ld.lld', you probably don't have LLVM installed")
	}

	return nil
}
