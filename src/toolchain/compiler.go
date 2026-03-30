package toolchain

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
)

func Compile(input io.Reader, output string, opt uint8) error {
	cmd := exec.Command("llc", fmt.Sprintf("-O%d", opt), "--filetype=obj", "-o", output)

	cmd.Stdin = input

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, &stderr)
	}

	return nil
}
