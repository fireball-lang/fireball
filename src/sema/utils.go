package sema

import "fireball/types"

func getOptionInnerType(typ types.Type) types.Type {
	if s, ok := typ.(*types.Struct); ok && s.Name == "core::Option" && len(s.Substitutions) == 1 {
		return s.Substitutions[0].Type
	}

	return nil
}
