package sema

import (
	"fireball/ast"
	"fireball/core"
	"fireball/symbols"
	"fireball/types"
	"fmt"
	"slices"
	"strings"
)

type common struct {
	scopes symbols.ScopeStack

	path        string
	fileModPath []string

	instantiations *types.InstantiationCache
	typeEnv        *TypeEnvironment

	checkVisibility      bool
	checkTypeConstraints bool
	selfType             types.Type

	nodeTypes   map[ast.Node]types.Type
	diagnostics []core.Diagnostic
}

func setupCommon(file *ast.File, fileSymbols []symbols.Symbol, root symbols.Scope, instantiations *types.InstantiationCache, typeEnv *TypeEnvironment, nodeTypes map[ast.Node]types.Type, path string) common {
	fileModPath := make([]string, 0, len(file.Mod.Path))
	for _, entry := range file.Mod.Path {
		fileModPath = append(fileModPath, entry.Token.Text)
	}

	c := common{
		path:                 path,
		fileModPath:          fileModPath,
		instantiations:       instantiations,
		typeEnv:              typeEnv,
		checkVisibility:      true,
		checkTypeConstraints: false,
		nodeTypes:            nodeTypes,
	}

	c.scopes.Push(root)
	c.scopes.Push(c.GetImportsScope(root, file))
	c.scopes.Push(symbols.SymbolScope(fileSymbols))

	return c
}

func (c *common) GetImportsScope(root symbols.Scope, file *ast.File) symbols.Scope {
	scope := symbols.NewBasicScope()

	for _, i := range file.Imports {
		// Get import scope
		importScope, ok := getScope(root, i.Path)
		if !ok {
			range_ := i.Range()
			if len(i.Path) > 0 {
				range_ = ast.SliceRange(i.Path)
			}

			sb := strings.Builder{}

			for j, entry := range i.Path {
				if j > 0 {
					sb.WriteString("::")
				}
				sb.WriteString(entry.Token.Text)
			}

			c.ErrorRange(range_, "module '%s' cannot be found", sb.String())
			continue
		}

		// Scope import
		if len(i.Symbols) == 0 {
			var name string
			var errNode ast.Node

			if i.Alias == nil {
				name = i.Path[len(i.Path)-1].Token.Text
				errNode = i.Path[len(i.Path)-1]
			} else {
				name = i.Alias.Token.Text
				errNode = i.Alias
			}

			if !scope.AddScope(name, importScope) {
				c.Error(errNode, "module alias with the name '%s' already exists", name)
			}

			continue
		}

		// Symbols import
		importModPath := make([]string, len(i.Path))
		for j, entry := range i.Path {
			importModPath[j] = entry.Token.Text
		}

		for _, name := range i.Symbols {
			symbol, ok := importScope.GetSymbol(symbols.Type|symbols.Variable|symbols.Function, name.Token.Text)
			if !ok {
				c.Error(name, "symbol '%s' cannot be found", name.Token.Text)
				continue
			}

			if c.checkVisibility && !symbol.Public && !slices.Equal(importModPath, c.fileModPath) {
				c.Error(name, "symbol '%s' is private", name.Token.Text)
			}

			scope.AddSymbol(symbol)

			c.nodeTypes[name] = symbol.Type
		}
	}

	return scope
}

func (c *common) GetSymbol(domain symbols.Domain, entries []*ast.IdentifierEntry) (symbols.Symbol, bool) {
	if len(entries) == 0 {
		return symbols.Symbol{}, false
	}

	var scope symbols.Scope = &c.scopes
	var subs []types.Substitution

	crossedModuleBoundary := false

	// Resolve scope
	for i := 0; i < len(entries)-1; i++ {
		entry := entries[i]
		name := entry.Name.Token.Text

		// Self:: inside impl / interface blocks
		if name == "Self" && c.selfType != nil {
			c.nodeTypes[entry] = c.selfType

			if typeScope := c.typeEnv.GetTypeScope(c.selfType); typeScope != nil {
				if len(entry.TypeArgs) != 0 {
					c.ErrorRange(ast.SliceRange(entry.TypeArgs), "'Self' type cannot have type arguments")
				}

				scope = typeScope
				continue
			}

			// Invalid type
			c.ErrorRange(ast.SliceRange(entries[:i+1]), "type '%s' cannot have any symbols", c.selfType)
			return symbols.Symbol{}, false
		}

		// Module
		if child, ok := scope.GetScope(name); ok {
			if len(entry.TypeArgs) != 0 {
				c.ErrorRange(ast.SliceRange(entry.TypeArgs), "module cannot have type arguments")
			}

			crossedModuleBoundary = true
			scope = child
			continue
		}

		// Type
		if symbol, ok := scope.GetSymbol(symbols.Type, name); ok {
			c.nodeTypes[entry] = symbol.Type

			// Type scope
			if typeScope := c.typeEnv.GetTypeScope(symbol.Type); !core.IsNil(typeScope) {
				// Check if this type belongs to a different module
				if c.checkVisibility {
					var modulePath []string

					switch {
					case symbol.Kind == symbols.Struct:
						modulePath = symbol.Type.(*types.Struct).ModulePath
					case symbol.Kind == symbols.Enum:
						modulePath = symbol.Type.(*types.Enum).ModulePath
					case symbol.Kind == symbols.Interface:
						modulePath = symbol.Type.(*types.Interface).ModulePath
					}

					if len(modulePath) > 0 && !slices.Equal(modulePath, c.fileModPath) {
						crossedModuleBoundary = true
					}
				}

				// Check type args
				subs = c.CheckTypeArgsForSymbolEntry(symbol, entry, subs)

				scope = typeScope
				continue
			}

			// Invalid type
			c.ErrorRange(ast.SliceRange(entries[:i+1]), "type '%s' cannot have any symbols", symbol.Type)
			return symbols.Symbol{}, false
		}

		// Unknown
		sb := strings.Builder{}

		for j, entry := range entries[:i+1] {
			if j > 0 {
				sb.WriteString("::")
			}
			sb.WriteString(entry.Name.Token.Text)
		}

		c.ErrorRange(ast.SliceRange(entries), "module or type '%s' cannot be found", sb.String())
		return symbols.Symbol{}, false
	}

	// Resolve symbol
	entry := entries[len(entries)-1]
	symbol, ok := scope.GetSymbol(domain, entry.Name.Token.Text)

	if !ok {
		c.Error(entry, "symbol '%s' cannot be found", entry.Name.Token.Text)
		return symbols.Symbol{}, false
	}

	// Check symbol visibility
	if crossedModuleBoundary && c.checkVisibility && !symbol.Public {
		c.Error(entry, "symbol '%s' is private", entry.Name.Token.Text)
	}

	// Check type args and substitute
	subs = c.CheckTypeArgsForSymbolEntry(symbol, entry, subs)

	if len(subs) > 0 {
		symbol.Type = c.instantiations.Get(symbol.Type, subs)
	}

	// Assign type and return
	c.nodeTypes[entry] = symbol.Type

	return symbol, ok
}

func (c *common) CheckTypeArgsForSymbolEntry(symbol symbols.Symbol, entry *ast.IdentifierEntry, subs []types.Substitution) []types.Substitution {
	switch symbol.Kind {
	case symbols.Struct:
		return c.CheckTypeArgs(symbol.Type, symbol.Type.(*types.Struct).TypeParams, entry, subs)
	case symbols.Interface:
		return c.CheckTypeArgs(symbol.Type, symbol.Type.(*types.Interface).TypeParams, entry, subs)
	case symbols.Func:
		return c.CheckTypeArgs(symbol.Type, symbol.Type.(*types.Func).TypeParams, entry, subs)

	default:
		if len(entry.TypeArgs) > 0 {
			c.ErrorRange(ast.SliceRange(entry.TypeArgs), "type '%s' cannot have type arguments", symbol.Type)
		}

		return subs
	}
}

func (c *common) CheckTypeArgs(typ types.Type, typeParams []*types.Param, entry *ast.IdentifierEntry, subs []types.Substitution) []types.Substitution {
	// Check count
	if len(typeParams) != len(entry.TypeArgs) {
		range_ := entry.Range()
		if len(entry.TypeArgs) > 0 {
			range_ = ast.SliceRange(entry.TypeArgs)
		}

		c.ErrorRange(range_, "type '%s' expects %d type parameters, got %d", typ, len(typeParams), len(entry.TypeArgs))
		return subs
	}

	if len(typeParams) == 0 {
		return subs
	}

	// Build substitutions
	base := len(subs)

	for i, param := range typeParams {
		arg := c.ResolveAndAnalyzeType(entry.TypeArgs[i])

		subs = append(subs, types.Substitution{
			Param: param,
			Type:  arg,
		})
	}

	// Check constraints
	if c.checkTypeConstraints {
		for i, param := range typeParams {
			for _, constraint := range param.Constraints {
				if in, ok := c.instantiations.Substitute(constraint, subs).(*types.Interface); ok {
					c.CheckConstraint(subs[base+i].Type, in, entry.TypeArgs[i])
				}
			}
		}
	}

	return subs
}

func (c *common) CheckConstraint(typ types.Type, constraint *types.Interface, node ast.Node) bool {
	if typ == types.Invalid {
		return false
	}

	// Type param
	if p, ok := typ.(*types.Param); ok {
		if len(p.Constraints) == 0 {
			c.Error(node, "type parameter '%s' has no constraint, expected '%s'", p.Name, constraint)
			return false
		}

		// Satisfied if any of the param's constraints matches
		for _, c := range p.Constraints {
			if c.AsImmutable() == constraint.AsImmutable() {
				return true
			}
		}

		c.Error(node, "type parameter '%s' does not satisfy '%s'", p.Name, constraint)
		return false
	}

	// Pointer
	checkTyp := typ

	if ptr, ok := typ.(*types.Pointer); ok {
		if constraint.Mutable && !ptr.Mutable {
			c.Error(node, "type '%s' does not satisfy constraint '%s': pointer must be mutable ('mut %s')", typ, constraint, typ)
			return false
		}

		checkTyp = ptr.Pointee
	}

	// Resolve canonical generic template of the constraint
	constraintCanon := constraint.AsImmutable()
	constraintTemplate := constraintCanon
	if constraintCanon.Generic != nil {
		constraintTemplate = constraintCanon.Generic
	}

	// Check interface conformances
	for _, conf := range c.typeEnv.GetConformances(checkTyp) {
		conf = conf.AsImmutable()
		confTemplate := conf
		if conf.Generic != nil {
			confTemplate = conf.Generic
		}

		if constraintTemplate == confTemplate {
			if constraint.Generic == nil || conf == constraintCanon {
				return true
			}
		}
	}

	c.Error(node, "type '%s' does not satisfy constraint '%s'", typ, constraint)
	return false
}

func getScope(scope symbols.Scope, path []*ast.Leaf) (symbols.Scope, bool) {
	for _, entry := range path {
		var ok bool
		scope, ok = scope.GetScope(entry.Token.Text)

		if !ok {
			return nil, false
		}
	}

	return scope, true
}

func (c *common) Error(node ast.Node, format string, args ...any) ExprInfo {
	return c.ErrorRange(node.Range(), format, args...)
}

func (c *common) Warning(node ast.Node, format string, args ...any) {
	c.WarningRange(node.Range(), format, args...)
}

func (c *common) ErrorRange(range_ core.Range, format string, args ...any) ExprInfo {
	c.diagnostics = append(c.diagnostics, core.Diagnostic{
		Kind:    core.Error,
		Path:    c.path,
		Range:   range_,
		Message: fmt.Sprintf(format, args...),
	})

	return ExprInfo{Type: types.Invalid}
}

func (c *common) WarningRange(range_ core.Range, format string, args ...any) {
	c.diagnostics = append(c.diagnostics, core.Diagnostic{
		Kind:    core.Warning,
		Path:    c.path,
		Range:   range_,
		Message: fmt.Sprintf(format, args...),
	})
}
