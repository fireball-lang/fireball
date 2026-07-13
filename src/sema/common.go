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

	instantiations types.InstantiationCache
	typeEnv        *TypeEnvironment

	checkVisibility      bool
	checkTypeConstraints bool
	selfType             types.Type

	nodeTypes   map[ast.Node]types.Type
	diagnostics []core.Diagnostic
}

func setupCommon(file *ast.File, fileSymbols []symbols.Symbol, root symbols.Scope, instantiations types.InstantiationCache, typeEnv *TypeEnvironment, nodeTypes map[ast.Node]types.Type, path string) common {
	fileModPath := make([]string, 0, len(file.Mod.Path.Entries))
	for _, entry := range file.Mod.Path.Entries {
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
		importScope, ok := getScope(root, i.Path.Entries)
		if !ok {
			c.Error(i.Path, "module '%s' cannot be found", i.Path)
			continue
		}

		// Scope import
		if len(i.Symbols) == 0 {
			var name string
			var errNode ast.Node

			if i.Alias == nil {
				name = i.Path.LastName()
				errNode = i.Path.Entries[len(i.Path.Entries)-1]
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
		importModPath := make([]string, len(i.Path.Entries))
		for j, entry := range i.Path.Entries {
			importModPath[j] = entry.Token.Text
		}

		for _, name := range i.Symbols {
			symbol, ok := importScope.GetSymbol(name.Token.Text)
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

func (c *common) GetSymbol(path *ast.IdentifierPath) (symbols.Symbol, bool) {
	if len(path.Entries) == 0 {
		return symbols.Symbol{}, false
	}

	var scope symbols.Scope = &c.scopes
	crossedModuleBoundary := false

	for i := 0; i < len(path.Entries)-1; i++ {
		entry := path.Entries[i].Token.Text

		// Handle Self:: inside impl / interface blocks.
		if entry == "Self" && c.selfType != nil {
			c.nodeTypes[path.Entries[i]] = c.selfType

			if typeScope := c.typeEnv.GetTypeScope(c.selfType); typeScope != nil {
				scope = typeScope
				continue
			}

			c.Error(path, "method '%s' cannot be found on type 'Self'", path.Entries[i+1].Token.Text)
			return symbols.Symbol{}, false
		}

		// Try module scope navigation first.
		if child, ok := scope.GetScope(entry); ok {
			crossedModuleBoundary = true
			scope = child
			continue
		}

		// Try type scope (for Struct::staticMethod, Enum::case or constrained type param).
		if symbol, ok := scope.GetSymbol(entry); ok {
			c.nodeTypes[path.Entries[i]] = symbol.Type

			if typeScope := c.typeEnv.GetTypeScope(symbol.Type); typeScope != nil {
				// Check if this type belongs to a different module.
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

				scope = typeScope
				continue
			}

			// Symbol found but has no registered static methods.
			c.Error(path, "member '%s' cannot be found on type '%s'", path.Entries[i+1].Token.Text, symbol.Type)
			return symbols.Symbol{}, false
		}

		// Neither a module nor a type — build the full path for the error message.
		sb := strings.Builder{}

		for j, leaf := range path.Entries[:i+1] {
			if j > 0 {
				sb.WriteString("::")
			}
			sb.WriteString(leaf.Token.Text)
		}

		c.Error(path, "module or type '%s' cannot be found", &sb)
		return symbols.Symbol{}, false
	}

	// Look up the final segment in whatever scope we navigated to.
	entry := path.Entries[len(path.Entries)-1]

	symbol, ok := scope.GetSymbol(entry.Token.Text)
	if ok {
		if crossedModuleBoundary && c.checkVisibility && !symbol.Public {
			c.Error(entry, "symbol '%s' is private", entry.Token.Text)
		}

		c.nodeTypes[entry] = symbol.Type
	} else {
		c.Error(entry, "symbol '%s' cannot be found", entry.Token.Text)
	}

	return symbol, ok
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

func (c *common) Error(node ast.Node, format string, args ...any) ExprInfo {
	return c.ErrorRange(node.Range(), format, args...)
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
