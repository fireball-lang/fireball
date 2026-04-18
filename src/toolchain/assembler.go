package toolchain

import (
	"bytes"
	"fireball/core"
	"fmt"
	"io"
	"os/exec"
)

func Assemble(input io.Reader, output string) error {
	defer core.Scope()()

	cmd := exec.Command("llvm-as", "-o", output)

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
