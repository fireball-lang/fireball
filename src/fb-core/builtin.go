package fb_core

import (
	"fireball/ast"
	"fireball/types"
)

type Builtins struct {
	StringView *types.Struct
	Option     *types.Struct

	Zeroable *types.Interface

	PanicNode *ast.Func
	PanicType *types.Func

	// Reflection

	Case  *types.Struct
	Field *types.Struct

	Implementation *types.Struct

	TypeInfo *types.Struct
}
