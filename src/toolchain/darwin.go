package toolchain

import (
	"bytes"
	"fireball/abi"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func getTargetDarwinArm64() (Target, error) {
	// Get SDK version
	cmd := exec.Command("xcrun", "--show-sdk-version")

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return Target{}, fmt.Errorf("failed to find the SDK version: %s", &stderr)
	}

	version := strings.TrimSpace(stdout.String())

	// Return target
	return Target{
		Name:                    "darwin-arm64",
		DataLayout:              "e-m:o-i64:64-i128:128-n32:64-S128",
		Triple:                  "arm64-apple-darwin",
		Arch:                    abi.ARM64,
		CallConv:                abi.AAPCS64,
		ObjectFileExtension:     ".o",
		ExecutableFileExtension: "",
		AdditionalLinkArgs: []string{
			"-platform_version", "macos", "11.0", version,
			"-arch", "arm64",
		},
	}, nil
}

func findLibcDarwin() (LibC, error) {
	// Get SDK path
	cmd := exec.Command("xcrun", "--show-sdk-path")

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return LibC{}, fmt.Errorf("failed to find the SDK: %s", &stderr)
	}

	path := strings.TrimSpace(stdout.String())

	if _, err := os.Stat(path); err != nil {
		return LibC{}, fmt.Errorf("invalid SDK path: %s", path)
	}

	// Return LibC
	return LibC{
		LibPaths:        []string{filepath.Join(path, "usr", "lib")},
		Libs:            []string{"System"},
		PreObjectPaths:  nil,
		PostObjectPaths: nil,
		AdditionalLinkArgs: []string{
			"-syslibroot", path,
		},
	}, nil
}
