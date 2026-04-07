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

	AdditionalLinkArgs []string
}

func GetTarget() (Target, error) {
	if runtime.GOOS == "windows" && runtime.GOARCH == "amd64" {
		return getTargetWindowsAmd64()
	}
	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		return getTargetLinuxAmd64()
	}
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		return getTargetDarwinArm64()
	}

	return Target{}, errors.New("fireball doesn't support this platform / architecture")
}
