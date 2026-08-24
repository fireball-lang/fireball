package toolchain

import (
	"embed"
	_ "embed"
	"fireball/abi"
	"fireball/core"
	"os"
	"path/filepath"
)

func getTargetWindowsAmd64() (Target, error) {
	return Target{
		Name:                    "windows-amd64",
		DataLayout:              "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128",
		Triple:                  "x86_64-pc-windows-gnu",
		Arch:                    abi.AMD64,
		CallConv:                abi.Win64,
		ObjectFileExtension:     ".obj",
		ExecutableFileExtension: ".exe",
		AdditionalLinkArgs:      []string{"-m", "i386pep"},
	}, nil
}

//go:embed mingw
var mingwFs embed.FS

//go:embed runtime/windows-amd64
var runtimeWindowsAmd64Fs embed.FS

func findLibcWindows() (LibC, error) {
	// Get cache path
	cachePath, err := os.UserCacheDir()
	if err != nil {
		return LibC{}, err
	}

	// Extract runtime
	runtimePath := filepath.Join(cachePath, "fireball", "runtime")

	err = core.ExtractVersionedEmbedFs(runtimePath, "runtime/windows-amd64", runtimeWindowsAmd64Fs)
	if err != nil {
		return LibC{}, err
	}

	// Extract mingw
	mingwPath := filepath.Join(cachePath, "fireball", "mingw")

	err = core.ExtractVersionedEmbedFs(mingwPath, "mingw", mingwFs)
	if err != nil {
		return LibC{}, err
	}

	// Return
	return LibC{
		LibPaths:           []string{runtimePath, mingwPath},
		Libs:               []string{"clang_rt.builtins-x86_64", "unwind", "kernel32", "m", "mingw32", "mingwex", "ucrt", "ws2_32", "secur32"},
		PreObjectPaths:     []string{"crt2.o", "crtbegin.o"},
		PostObjectPaths:    []string{"crtend.o"},
		AdditionalLinkArgs: nil,
	}, nil
}
