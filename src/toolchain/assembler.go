package toolchain

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
)

func Assemble(input io.Reader, output string) error {
	cmd := exec.Command("llvm-as", "-o", output)

	cmd.Stdin = input

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, &stderr)
	}

	return nil
}
