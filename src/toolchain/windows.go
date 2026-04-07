package toolchain

import (
	"embed"
	_ "embed"
	"errors"
	"fireball/abi"
	"io"
	"os"
	"path/filepath"
	"slices"
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
		AdditionalLinkArgs:      nil,
	}, nil
}

//go:embed mingw
var mingw embed.FS

func findLibcWindows() (LibC, error) {
	path, err := extractMingw()
	if err != nil {
		return LibC{}, err
	}

	return LibC{
		LibPaths:           []string{path},
		Libs:               []string{"gcc", "gcc_eh", "kernel32", "m", "mingw32", "mingwex", "ucrt"},
		PreObjectPaths:     []string{"crt2.o", "crtbegin.o"},
		PostObjectPaths:    []string{"crtend.o"},
		AdditionalLinkArgs: []string{"-m", "i386pep"},
	}, nil
}

func extractMingw() (string, error) {
	// Get base path
	path, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}

	path = filepath.Join(path, "fireball", "mingw")
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", err
	}

	// Check version
	versionInfoFile, err := os.Open(filepath.Join(path, "version_info.txt"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer versionInfoFile.Close()

	if err == nil {
		found, err := io.ReadAll(versionInfoFile)
		if err != nil {
			return "", err
		}

		expectedFile, err := mingw.Open("mingw/version_info.txt")
		if err != nil {
			return "", err
		}

		expected, err := io.ReadAll(expectedFile)
		if err != nil {
			return "", err
		}

		if slices.Equal(found, expected) {
			return path, nil
		}
	}

	// Copy over mingw libraries
	entries, err := mingw.ReadDir("mingw")
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		from, err := mingw.Open("mingw/" + entry.Name())
		if err != nil {
			return "", err
		}

		to, err := os.Create(filepath.Join(path, entry.Name()))
		if err != nil {
			_ = from.Close()
			return "", err
		}

		_, err = io.Copy(to, from)

		_ = from.Close()
		_ = to.Close()

		if err != nil {
			return "", err
		}
	}

	return path, nil
}
