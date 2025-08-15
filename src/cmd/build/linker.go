package build

import (
	"fireball/profiler"
	"fireball/utils"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func linkBinary(profile Profile, target Target, workingPath string, inputs []string, binaryName string) (string, error) {
	defer profiler.Event()()

	windows := strings.Contains(target.Name, "windows")
	name := "ld.lld"

	if windows {
		name = "lld-link"
	}

	cmd := exec.Command(name)
	cmd.Dir = workingPath

	if profile.Lto == Thin && !windows {
		cmd.Args = append(cmd.Args, "--lto=thin")
	}

	if windows {
		cmd.Args = append(cmd.Args, "/out:"+binaryName)
	} else {
		cmd.Args = append(cmd.Args, fmt.Sprintf("-O%d", profile.Opt))
		cmd.Args = append(cmd.Args, "-o", binaryName)
	}

	var output strings.Builder
	cmd.Stdout = &output
	cmd.Stderr = &output

	for _, path := range target.LibPaths {
		prefix := utils.Ternary(windows, "/libpath:", "-L")
		cmd.Args = append(cmd.Args, prefix+path)
	}

	for _, arg := range target.AdditionalLinkArgs {
		cmd.Args = append(cmd.Args, arg)
	}

	for _, path := range target.PreObjectPaths {
		cmd.Args = append(cmd.Args, path)
	}

	for _, lib := range target.Libs {
		prefix := utils.Ternary(windows, "", "-l")
		cmd.Args = append(cmd.Args, prefix+lib)
	}

	for _, input := range inputs {
		relative, err := filepath.Rel(workingPath, input)

		if err == nil {
			cmd.Args = append(cmd.Args, relative)
		} else {
			cmd.Args = append(cmd.Args, input)
		}
	}

	for _, path := range target.PostObjectPaths {
		cmd.Args = append(cmd.Args, path)
	}

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
