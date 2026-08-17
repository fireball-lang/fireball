package project

import (
	"fireball/ast"
	"fireball/fb-core"
	"fireball/symbols"
	"fireball/types"
)

func GetBuiltins(projMap map[string]*Project) fb_core.Builtins {
	var builtins fb_core.Builtins

	proj := projMap["core"]
	if proj == nil {
		panic("project.GetBuiltins() - Failed to get 'core' project")
	}

	symbol, ok := proj.Module.GetSymbol(symbols.Type, "StringView")
	if !ok {
		panic("project.GetBuiltins() - Failed to get 'core::StringView' type")
	}
	builtins.StringView = symbol.Type.(*types.Struct)

	symbol, ok = proj.Module.GetSymbol(symbols.Type, "Zeroable")
	if !ok {
		panic("project.GetBuiltins() - Failed to get 'core::Zeroable' type")
	}
	builtins.Zeroable = symbol.Type.(*types.Interface)

	symbol, ok = proj.Module.GetSymbol(symbols.Function, "panic")
	if !ok {
		panic("project.GetBuiltins() - Failed to get 'core::panic' function")
	}
	builtins.PanicNode = symbol.Node.(*ast.Func)
	builtins.PanicType = symbol.Type.(*types.Func)

	// Reflection

	symbol, ok = proj.Module.GetSymbol(symbols.Type, "Case")
	if !ok {
		panic("project.GetBuiltins() - Failed to get 'core::Case' type")
	}
	builtins.Case = symbol.Type.(*types.Struct)

	symbol, ok = proj.Module.GetSymbol(symbols.Type, "Field")
	if !ok {
		panic("project.GetBuiltins() - Failed to get 'core::Field' type")
	}
	builtins.Field = symbol.Type.(*types.Struct)

	symbol, ok = proj.Module.GetSymbol(symbols.Type, "Implementation")
	if !ok {
		panic("project.GetBuiltins() - Failed to get 'core::Implementation' type")
	}
	builtins.Implementation = symbol.Type.(*types.Struct)

	symbol, ok = proj.Module.GetSymbol(symbols.Type, "TypeInfo")
	if !ok {
		panic("project.GetBuiltins() - Failed to get 'core::TypeInfo' type")
	}
	builtins.TypeInfo = symbol.Type.(*types.Struct)

	return builtins
}
