package build

import (
	"fmt"
	"os/exec"
	"strings"
)

type compileParams struct {
	profile Profile
	target  Target
}

func compile(params compileParams, irPath string) (string, error) {
	objPath := strings.TrimSuffix(irPath, ".ll") + params.target.ObjectFileExtension

	cmd := exec.Command("llc", fmt.Sprintf("-O%d", params.profile.Opt), "--filetype=obj", "-o", objPath, irPath)

	var output strings.Builder
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Run(); err != nil {
		if output.Len() > 0 {
			return "", fmt.Errorf("failed to compile IR: %w - %s", err, output.String())
		}

		return "", fmt.Errorf("failed to compile IR: %w", err)
	}

	if output.Len() > 0 {
		return "", fmt.Errorf("failed to compile IR: %s", output.String())
	}

	return objPath, nil
}
