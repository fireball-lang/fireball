package toolchain

import (
	"errors"
	"fireball/core"
	"runtime"
)

type LibC struct {
	LibPaths []string
	Libs     []string

	PreObjectPaths  []string
	PostObjectPaths []string

	AdditionalLinkArgs []string
}

func FindLibC() (LibC, error) {
	defer core.Scope()()

	if runtime.GOOS == "windows" {
		return findLibcWindows()
	}
	if runtime.GOOS == "linux" {
		return findLibcLinux()
	}
	if runtime.GOOS == "darwin" {
		return findLibcDarwin()
	}

	return LibC{}, errors.New("fireball doesn't support this platform / architecture")
}
