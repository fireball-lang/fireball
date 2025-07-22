package build

import (
	"fmt"
	"os/exec"
	"strings"
)

func compileToBitcode(irPath string) (string, error) {
	bcPath := strings.TrimSuffix(irPath, ".ll") + ".bc"

	cmd := exec.Command("llvm-as", irPath, "-o", bcPath)

	var output strings.Builder
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Run(); err != nil {
		return "", err
	}

	if output.Len() > 0 {
		return "", fmt.Errorf("failed to compile IR to BC: %s", output.String())
	}

	return bcPath, nil
}
