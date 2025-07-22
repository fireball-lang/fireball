package build

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
}

func getTarget() (Target, error) {
	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		return Target{
			Name:       "linux-amd64",
			DataLayout: "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128",
			Triple:     "x86_64-unknown-linux",
			Arch:       abi.AMD64,
			CallConv:   abi.SystemV,
		}, nil
	}

	return Target{}, errors.New("fireball doesn't support this platform / architecture")
}
