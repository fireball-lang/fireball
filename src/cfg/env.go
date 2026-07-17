package cfg

import (
	"fireball/ast"
	"runtime"
)

type Env struct {
	TargetOs     ast.TargetOsKind
	TargetFamily ast.TargetFamilyKind
}

func (e *Env) ComputeDerived() {
	// TargetFamily
	switch e.TargetOs {
	case ast.WindowsOs:
		e.TargetFamily = ast.WindowsFamily
	case ast.Linux:
		e.TargetFamily = ast.Unix
	case ast.MacOS:
		e.TargetFamily = ast.Unix
	}
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

	// derived
	env.ComputeDerived()

	return env
}
