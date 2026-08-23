package toolchain

import (
	"errors"
	"fireball/core"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/sys/execabs"
)

type Toolchain struct {
	Llc    string
	LlvmAs string
	Lld    string
}

func Validate() (Toolchain, error) {
	defer core.Scope()()

	var toolchain Toolchain
	var err error

	if toolchain.Llc, err = findLlvmTool("llc"); err != nil {
		return Toolchain{}, err
	}
	if toolchain.LlvmAs, err = findLlvmTool("llvm-as"); err != nil {
		return Toolchain{}, err
	}

	lld := "ld.lld"
	if runtime.GOOS == "darwin" {
		lld = "ld64.lld"
	}
	if toolchain.Lld, err = findLlvmTool(lld); err != nil {
		return Toolchain{}, err
	}

	return toolchain, nil
}

func findLlvmTool(name string) (string, error) {
	// Check distribution 'tools' directory
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}

	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}

	path := filepath.Join(filepath.Dir(exe), "..", "tools", name)
	if runtime.GOOS == "windows" {
		path += ".exe"
	}

	_, err = os.Stat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err == nil {
		return path, nil
	}

	// Check system-wide installing in $PATH$
	if path, err := execabs.LookPath(name); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("failed to find '%s'", name)
}
