package toolchain

import (
	"bytes"
	"fireball/core"
	"fmt"
	"io"
	"os/exec"
)

func Compile(tc Toolchain, input io.Reader, output string, opt uint8) error {
	defer core.Scope()()

	cmd := exec.Command(tc.Llc, fmt.Sprintf("-O%d", opt), "--filetype=obj", "-o", output)

	cmd.Stdin = input

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, &stderr)
	}
	if stderr.Len() > 0 {
		return fmt.Errorf("failed to compile module: %s", &stderr)
	}

	return nil
}
