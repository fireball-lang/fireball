package build

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var libcPaths = []string{
	"/usr/lib/x86_64-linux-gnu",
	"/lib/x86_64-linux-gnu",
	"/usr/lib64",
	"/lib64",
	"/usr/lib",
	"/lib",
}

var libcFiles = []string{
	"crt1.o",
	"crti.o",
	"crtn.o",
	"libc.a",
	"libm.a",
}

func linkBinary(profile Profile, workingPath string, inputs []string, binaryName string) (string, error) {
	libcFolder, err := findLibcFolder()
	if err != nil {
		return "", err
	}

	cmd := exec.Command("ld.lld", fmt.Sprintf("-O%d", profile.Opt), "-o", binaryName)
	cmd.Dir = workingPath

	var output strings.Builder
	cmd.Stdout = &output
	cmd.Stderr = &output

	cmd.Args = append(cmd.Args, "-L"+libcFolder)

	cmd.Args = append(cmd.Args, "-dynamic-linker")
	cmd.Args = append(cmd.Args, "/lib64/ld-linux-x86-64.so.2")

	cmd.Args = append(cmd.Args, filepath.Join(libcFolder, "crt1.o"))
	cmd.Args = append(cmd.Args, filepath.Join(libcFolder, "crti.o"))

	cmd.Args = append(cmd.Args, "-lc")
	cmd.Args = append(cmd.Args, "-lm")

	for _, input := range inputs {
		relative, err := filepath.Rel(workingPath, input)

		if err == nil {
			cmd.Args = append(cmd.Args, relative)
		} else {
			cmd.Args = append(cmd.Args, input)
		}
	}

	cmd.Args = append(cmd.Args, filepath.Join(libcFolder, "crtn.o"))

	if err := cmd.Run(); err != nil {
		if output.Len() > 0 {
			return "", fmt.Errorf("failed to link binary: %w - %s", err, output.String())
		}

		return "", fmt.Errorf("failed to link binary: %w", err)
	}

	if output.Len() > 0 {
		return "", fmt.Errorf("failed to link binary: %s", output.String())
	}

	return filepath.Join(workingPath, binaryName), nil
}

func findLibcFolder() (string, error) {
	for _, path := range libcPaths {
		ok := true

		for _, file := range libcFiles {
			path := filepath.Join(path, file)

			if _, err := os.Stat(path); err != nil {
				ok = false
				break
			}
		}

		if ok {
			return path, nil
		}
	}

	return "", errors.New("failed to locate libc installation folder")
}
