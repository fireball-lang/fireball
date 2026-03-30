package toolchain

import (
	"bytes"
	"fmt"
	"os/exec"
)

func Link(inputs []string, output string, opt uint8, libc *LibC) error {
	cmd := exec.Command("ld.lld", fmt.Sprintf("-O%d", opt), "-o", output)

	//cmd.Args = append(cmd.Args, "--lto=thin")
	cmd.Args = append(cmd.Args, fmt.Sprintf("--lto-O%d", opt))
	cmd.Args = append(cmd.Args, fmt.Sprintf("--lto-CGO%d", opt))

	if libc != nil {
		for _, path := range libc.LibPaths {
			cmd.Args = append(cmd.Args, "-L", path)
		}

		for _, lib := range libc.Libs {
			cmd.Args = append(cmd.Args, "-l", lib)
		}

		cmd.Args = append(cmd.Args, libc.AdditionalLinkArgs...)

		cmd.Args = append(cmd.Args, libc.PreObjectPaths...)
	}

	cmd.Args = append(cmd.Args, inputs...)

	if libc != nil {
		cmd.Args = append(cmd.Args, libc.PostObjectPaths...)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, &stderr)
	}

	return nil
}
