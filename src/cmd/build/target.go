package build

import (
	"errors"
	"fireball/abi"
	"os"
	"path/filepath"
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

	LibPaths []string
	Libs     []string

	PreObjectPaths  []string
	PostObjectPaths []string

	AdditionalLinkArgs []string
}

func getTarget() (Target, error) {
	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		target := Target{
			Name:                    "linux-amd64",
			DataLayout:              "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128",
			Triple:                  "x86_64-pc-linux-gnu",
			Arch:                    abi.AMD64,
			CallConv:                abi.SystemV,
			ObjectFileExtension:     ".o",
			ExecutableFileExtension: "",
		}

		return target, findLibcLinux(&target)
	}

	if runtime.GOOS == "windows" && runtime.GOARCH == "amd64" {
		target := Target{
			Name:                    "windows-amd64",
			DataLayout:              "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128",
			Triple:                  "x86_64-pc-windows-msvc",
			Arch:                    abi.AMD64,
			CallConv:                abi.Win64,
			ObjectFileExtension:     ".obj",
			ExecutableFileExtension: ".exe",
		}

		return target, findLibcWindows(&target)
	}

	return Target{}, errors.New("fireball doesn't support this platform / architecture")
}

// Linux

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

func findLibcLinux(target *Target) error {
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
			target.LibPaths = []string{path}
			target.Libs = []string{"c", "m"}

			target.PreObjectPaths = []string{filepath.Join(path, "crt1.o"), filepath.Join(path, "crti.o")}
			target.PostObjectPaths = []string{filepath.Join(path, "crtn.o")}

			target.AdditionalLinkArgs = []string{"-dynamic-linker", "/lib64/ld-linux-x86-64.so.2"}

			return nil
		}
	}

	return errors.New("failed to locate libc installation folder")
}

// Windows

func findLibcWindows(target *Target) error {
	// Development kit
	kitBase := "C:\\Program Files (x86)\\Windows Kits\\10\\Lib"

	entries, err := os.ReadDir(kitBase)
	if err != nil {
		return errors.New("failed to find a valid windows development kit")
	}

	ucrtPath := filepath.Join(kitBase, entries[0].Name(), "ucrt", "x64")
	_, err = os.Stat(ucrtPath)
	if err != nil {
		return errors.New("failed to find a valid windows development kit, ucrt")
	}

	umPath := filepath.Join(kitBase, entries[0].Name(), "um", "x64")
	_, err = os.Stat(umPath)
	if err != nil {
		return errors.New("failed to find a valid windows development kit, um")
	}

	// Build tools
	toolsBase := "C:\\Program Files (x86)\\Microsoft Visual Studio\\2022\\BuildTools\\VC\\Tools\\MSVC"

	entries, err = os.ReadDir(toolsBase)
	if err != nil {
		return errors.New("failed to find a MSVC installation")
	}

	msvcPath := filepath.Join(toolsBase, entries[0].Name(), "lib", "x64")
	_, err = os.Stat(msvcPath)
	if err != nil {
		return errors.New("failed to find a MSVC installation, lib")
	}

	// Setup target
	target.LibPaths = []string{ucrtPath, umPath, msvcPath}
	target.Libs = []string{"libucrt.lib", "libcmt.lib"}

	target.AdditionalLinkArgs = []string{"/machine:x64", "/subsystem:console"}

	return nil
}
