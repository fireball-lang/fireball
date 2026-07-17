package cfg

import (
	"fireball/ast"
	"runtime"
)

type Env struct {
	TargetOs     ast.TargetOsKind
	TargetFamily ast.TargetFamilyKind
}

func GetHost() Env {
	var env Env

	// TargetOs
	switch runtime.GOOS {
	case "windows":
		env.TargetOs = ast.WindowsOs
	case "linux":
		env.TargetOs = ast.Linux
	case "darwin":
		env.TargetOs = ast.MacOS

	default:
		panic("cfg.GetHost() - Unsupported OS")
	}

	// TargetFamily
	switch env.TargetOs {
	case ast.WindowsOs:
		env.TargetFamily = ast.WindowsFamily
	case ast.Linux:
		env.TargetFamily = ast.Unix
	case ast.MacOS:
		env.TargetFamily = ast.Unix
	}

	return env
}
