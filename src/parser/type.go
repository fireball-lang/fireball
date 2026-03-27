package parser

import (
	"fireball/ast"
	"fireball/lexer"
	"fireball/types"
)

func (p *parser) parseType() (ast.Type, int) {
	switch p.current.Kind {
	case lexer.Identifier:
		return p.parseIdentifierType()
	case lexer.LeftBracket:
		return p.parseArrayType()
	case lexer.Star:
		return p.parsePointerType()

	default:
		b := &ast.BadType{}
		b.Range_ = p.current.Range
		return b, p.error("expected type")
	}
}

func (p *parser) parseIdentifierType() (ast.Type, int) {
	switch p.current.Text {
	case "void":
		return p.primitiveType(types.Void)
	case "bool":
		return p.primitiveType(types.Bool)

	case "u8":
		return p.primitiveType(types.U8)
	case "u16":
		return p.primitiveType(types.U16)
	case "u32":
		return p.primitiveType(types.U32)
	case "u64":
		return p.primitiveType(types.U64)

	case "i8":
		return p.primitiveType(types.I8)
	case "i16":
		return p.primitiveType(types.I16)
	case "i32":
		return p.primitiveType(types.I32)
	case "i64":
		return p.primitiveType(types.I64)

	case "f32":
		return p.primitiveType(types.F32)
	case "f64":
		return p.primitiveType(types.F64)

	default:
		i := &ast.IdentifierType{}
		i.Token = p.advance()
		return i, -1
	}
}

func (p *parser) primitiveType(kind types.PrimitiveKind) (*ast.PrimitiveType, int) {
	t := &ast.PrimitiveType{}
	t.Range_ = p.advance().Range
	t.Kind = kind

	return t, -1
}

func (p *parser) parseArrayType() (a *ast.ArrayType, recoverId int) {
	a = &ast.ArrayType{}
	a.Range_.Start = p.current.Range.Start
	defer func() {
		a.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// '['
	if recoverId = p.expect(lexer.LeftBracket, "expected '[' before array size"); recoverId >= 0 {
		return
	}

	// Size
	if recoverId = p.expectFunc(lexer.IsInteger, "expected array size"); recoverId >= 0 {
		return
	}
	a.Size = p.previous

	// ']'
	if recoverId = p.expect(lexer.RightBracket, "expected ']' after array size"); recoverId >= 0 {
		return
	}

	// Type
	if a.Type, recoverId = p.parseType(); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parsePointerType() (t *ast.PointerType, recoverId int) {
	t = &ast.PointerType{}
	t.Range_.Start = p.current.Range.Start
	defer func() {
		t.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// '*'
	if recoverId = p.expect(lexer.Star, "expected '*' before pointee type"); recoverId >= 0 {
		return
	}

	// Pointee
	if t.Pointee, recoverId = p.parseType(); recoverId >= 0 {
		return
	}

	return
}
