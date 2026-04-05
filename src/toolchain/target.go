package toolchain

import (
	"errors"
	"fireball/abi"
	"runtime"
)

type Target struct {
	Name string

	DataLayout string
	Triple     string

	Arch     abi.Arch
	CallConv abi.CallConv

	ObjectFileExtension     string
	ExecutableFileExtension string
}

func GetTarget() (Target, error) {
	if runtime.GOOS == "windows" && runtime.GOARCH == "amd64" {
		return getTargetWindowsAmd64(), nil
	}
	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		return getTargetLinuxAmd64(), nil
	}

	return Target{}, errors.New("fireball doesn't support this platform / architecture")
}
