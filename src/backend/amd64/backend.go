package amd64

import (
	"fireball/backend/obj"
	"fireball/ir"
)

type backend struct {
	module *ir.Module
	file   *obj.File
}

func Generate(module *ir.Module) *obj.File {
	b := backend{
		module: module,
		file:   &obj.File{Arch: obj.AMD64},
	}

	return b.file
}
