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
		Triple:                  "x86_64-pc-windows-msvc",
		Arch:                    abi.AMD64,
		CallConv:                abi.Win64,
		ObjectFileExtension:     ".obj",
		ExecutableFileExtension: ".exe",
		AdditionalLinkArgs:      []string{"-m", "i386pep"},
	}, nil
}

//go:embed mingw
var mingwFs embed.FS

func findLibcWindows() (LibC, error) {
	// Get destination mingw path
	path, err := os.UserCacheDir()
	if err != nil {
		return LibC{}, err
	}

	path = filepath.Join(path, "fireball", "mingw")

	// Extract mingw
	err = core.ExtractVersionedEmbedFs(path, "mingw", mingwFs)
	if err != nil {
		return LibC{}, err
	}

	// Return
	return LibC{
		LibPaths:           []string{path},
		Libs:               []string{"gcc", "gcc_eh", "kernel32", "m", "mingw32", "mingwex", "ucrt"},
		PreObjectPaths:     []string{"crt2.o", "crtbegin.o"},
		PostObjectPaths:    []string{"crtend.o"},
		AdditionalLinkArgs: nil,
	}, nil
}
