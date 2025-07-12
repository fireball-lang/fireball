package analyzer

import (
	"fireball/ast"
	"fireball/lexer"
	"fireball/utils"
	"strconv"
	"strings"
)

func combinePathsWithoutLastSegments[T1, T2 ast.PathLike](path1 T1, path2 T2) ast.StringPath {
	path := ast.StringPath{Segments: make([]string, path1.SegmentCount()-1+path2.SegmentCount()-1)}

	for i := 0; i < path1.SegmentCount()-1; i++ {
		path.Segments[i] = path1.SegmentAt(i)
	}

	for i := 0; i < path2.SegmentCount()-1; i++ {
		path.Segments[path1.SegmentCount()-1+i] = path2.SegmentAt(i)
	}

	return path
}

func getSymbolLookup(ctx Context, scope Scope, path *ast.Path) SymbolLookup {
	if path.SegmentCount() <= 1 {
		return scope
	}

	module := scope.GetModule(path.SegmentAt(0))
	if utils.IsNil(module) {
		return nil
	}

	absPath := combinePathsWithoutLastSegments(module.AbsolutePath(), path)
	return ctx.GetAbsoluteModule(absPath)
}

func getDeclPath[T ast.Decl](decl T) *ast.Path {
	modulePath := ast.Root(decl).ModulePath()

	path := &ast.Path{Segments: make([]*ast.Leaf, len(modulePath.Segments)+1)}
	copy(path.Segments, modulePath.Segments)
	path.Segments[len(path.Segments)-1] = &ast.Leaf{Token: lexer.Token{Kind: lexer.Identifier, Text: decl.Name()}}

	return path
}

func addError[T utils.DiagnosticConsumer](diagnostics T, node ast.Node, message string) {
	if !utils.IsNil(diagnostics) {
		diagnostics.Add(utils.Diagnostic{
			Kind:    utils.Error,
			Message: message,
			Range:   node.Range(),
		})
	}
}

func parseInteger(token lexer.Token) (utils.Integer, string) {
	switch token.Kind {
	case lexer.Integer:
		if strings.ContainsAny(token.Text, "uU") {
			return parseUint(token.Text, 10, "Invalid unsigned integer.")
		}

		v, err := strconv.ParseInt(token.Text, 10, 64)

		if err != nil {
			return utils.Integer{}, "Invalid signed integer."
		}

		return utils.Signed(v), ""

	case lexer.Hexadecimal:
		return parseUint(token.Text, 16, "Invalid hexadecimal integer.")

	case lexer.Binary:
		return parseUint(token.Text, 2, "Invalid binary integer.")

	default:
		panic("analyzer.parseInteger() - Invalid token kind")
	}
}

func parseUint(str string, base int, errorMsg string) (utils.Integer, string) {
	if base == 10 {
		str = str[:len(str)-1]
	} else {
		str = str[2:]
	}

	v, err := strconv.ParseUint(str, base, 64)

	if err != nil {
		return utils.Integer{}, errorMsg
	}

	return utils.Unsigned(false, v), ""
}

func GetDeclFromDeclType[T ast.Decl](type_ ast.Type) (T, bool) {
	if type_, ok := type_.(*ast.DeclType); ok {
		if decl, ok := type_.Decl.(T); ok {
			return decl, true
		}
	}

	var empty T
	return empty, false
}

func structContainsType(decl *ast.Struct, lookingFor *ast.Struct) bool {
	for _, field := range decl.Fields {
		if d, ok := field.Type.(*ast.DeclType); ok {
			if s, ok := d.Decl.(*ast.Struct); ok {
				if s == lookingFor {
					return true
				}

				if structContainsType(s, lookingFor) {
					return true
				}
			}
		}
	}

	return false
}
