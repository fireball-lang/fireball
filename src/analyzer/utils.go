package analyzer

import (
	"fireball/ast"
	"fireball/lexer"
	"fireball/utils"
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
