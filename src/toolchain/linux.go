package toolchain

import (
	"embed"
	"errors"
	"fireball/abi"
	"fireball/core"
	"os"
	"path/filepath"
)

var linuxLibcPaths = []string{
	"/usr/lib/x86_64-linux-gnu",
	"/lib/x86_64-linux-gnu",
	"/usr/lib64",
	"/lib64",
	"/usr/lib",
	"/lib",
}

var linuxLibcFiles = []string{
	"crt1.o",
	"crti.o",
	"crtn.o",
	"libc.a",
	"libm.a",
}

func getTargetLinuxAmd64() (Target, error) {
	return Target{
		Name:                    "linux-amd64",
		DataLayout:              "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128",
		Triple:                  "x86_64-pc-linux-gnu",
		Arch:                    abi.AMD64,
		CallConv:                abi.SystemV,
		ObjectFileExtension:     ".o",
		ExecutableFileExtension: "",
		AdditionalLinkArgs:      []string{"--eh-frame-hdr"},
	}, nil
}

//go:embed runtime/linux-amd64
var runtimeLinuxAmd64Fs embed.FS

func findLibcLinux() (LibC, error) {
	// Get cache path
	cachePath, err := os.UserCacheDir()
	if err != nil {
		return LibC{}, err
	}

	// Extract runtime
	runtimePath := filepath.Join(cachePath, "fireball", "runtime")

	err = core.ExtractVersionedEmbedFs(runtimePath, "runtime/linux-amd64", runtimeLinuxAmd64Fs)
	if err != nil {
		return LibC{}, err
	}

	// Find LibC
	for _, path := range linuxLibcPaths {
		ok := true

		for _, file := range linuxLibcFiles {
			path := filepath.Join(path, file)

			if _, err := os.Stat(path); err != nil {
				ok = false
				break
			}
		}

		if ok {
			var libc LibC

			libc.LibPaths = []string{runtimePath, path}
			libc.Libs = []string{"clang_rt.builtins-x86_64", "c", "m", ":libgcc_s.so.1"}

			libc.PreObjectPaths = []string{filepath.Join(path, "crt1.o"), filepath.Join(path, "crti.o")}
			libc.PostObjectPaths = []string{filepath.Join(path, "crtn.o")}

			libc.AdditionalLinkArgs = []string{"-dynamic-linker", "/lib64/ld-linux-x86-64.so.2"}

			return libc, nil
		}
	}

	return LibC{}, errors.New("failed to locate libc installation folder")
}
