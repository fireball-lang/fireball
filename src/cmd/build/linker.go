package build

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func linkBinary(profile Profile, workingPath string, inputs []string, binaryName string) (string, error) {
	cmd := exec.Command("ld.lld", fmt.Sprintf("-O%d", profile.Opt), "-o", binaryName)
	cmd.Dir = workingPath

	var output strings.Builder
	cmd.Stdout = &output
	cmd.Stderr = &output

	cmd.Args = append(cmd.Args, "-L/usr/lib")

	cmd.Args = append(cmd.Args, "-dynamic-linker")
	cmd.Args = append(cmd.Args, "/lib64/ld-linux-x86-64.so.2")

	cmd.Args = append(cmd.Args, "/usr/lib/crt1.o")
	cmd.Args = append(cmd.Args, "/usr/lib/crti.o")

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

	cmd.Args = append(cmd.Args, "/usr/lib/crtn.o")

	if err := cmd.Run(); err != nil {
		return "", err
	}

	if output.Len() > 0 {
		return "", fmt.Errorf("failed to link binary: %s", output.String())
	}

	return filepath.Join(workingPath, binaryName), nil
}
