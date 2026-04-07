package toolchain

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func Link(inputs []string, output string, opt uint8, target Target, libc *LibC) error {
	lld := "ld.lld"
	if strings.Contains(target.Name, "darwin") {
		lld = "ld64.lld"
	}

	cmd := exec.Command(lld, "-o", output)

	//cmd.Args = append(cmd.Args, "--lto=thin")
	cmd.Args = append(cmd.Args, fmt.Sprintf("--lto-O%d", opt))
	cmd.Args = append(cmd.Args, fmt.Sprintf("--lto-CGO%d", opt))

	cmd.Args = append(cmd.Args, target.AdditionalLinkArgs...)

	if libc != nil {
		cmd.Args = append(cmd.Args, libc.AdditionalLinkArgs...)

		cmd.Args = append(cmd.Args, libc.PreObjectPaths...)
	}

	cmd.Args = append(cmd.Args, inputs...)

	if libc != nil {
		cmd.Args = append(cmd.Args, libc.PostObjectPaths...)

		for _, path := range libc.LibPaths {
			cmd.Args = append(cmd.Args, "-L", path)
		}

		for _, lib := range libc.Libs {
			cmd.Args = append(cmd.Args, "-l"+lib)
		}
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, &stderr)
	}

	return nil
}
