package parser

import (
	"fireball/ast"
	"fireball/core"
	"fireball/lexer"
	"fireball/types"
)

func (p *parser) canStartType() bool {
	switch p.current.Kind {
	case lexer.Mut, lexer.Identifier, lexer.LeftBracket, lexer.Ampersand, lexer.Star, lexer.Func, lexer.QuestionMark:
		return true
	default:
		return false
	}
}

func (p *parser) parseType() (ast.Type, int) {
	switch p.current.Kind {
	case lexer.Mut:
		if p.next.Kind == lexer.Ampersand {
			return p.parseReferenceType()
		}
		if p.next.Kind == lexer.Star {
			return p.parsePointerType()
		}
		if p.next.Kind == lexer.LeftBracket {
			return p.parseSliceType()
		}
		return p.parseIdentifierType()
	case lexer.Identifier:
		return p.parseIdentifierType()
	case lexer.LeftBracket:
		if p.next.Kind == lexer.RightBracket {
			return p.parseSliceType()
		}
		return p.parseArrayType()
	case lexer.Ampersand:
		return p.parseReferenceType()
	case lexer.Star:
		return p.parsePointerType()
	case lexer.Func:
		return p.parseFuncType()
	case lexer.QuestionMark:
		return p.parseOptionType()

	default:
		b := &ast.BadType{}
		b.Range_ = p.current.Range
		return b, p.error("expected type")
	}
}

func (p *parser) parseIdentifierType() (ast.Type, int) {
	mutable := false
	var mutableRange core.Range

	if p.current.Kind == lexer.Mut {
		mutableRange = p.advance().Range
		mutable = true
	}

	if p.current.Kind != lexer.Identifier {
		return p.badType(), p.error("expected an identifier")
	}

	if p.current.Text == "Self" {
		if mutable {
			p.reportError(mutableRange, "'Self' type cannot be mutable")
		}

		return p.selfType()
	}

	var kind types.PrimitiveKind
	ok := true

	switch p.current.Text {
	case "void":
		kind = types.Void
	case "bool":
		kind = types.Bool

	case "u8":
		kind = types.U8
	case "u16":
		kind = types.U16
	case "u32":
		kind = types.U32
	case "u64":
		kind = types.U64

	case "i8":
		kind = types.I8
	case "i16":
		kind = types.I16
	case "i32":
		kind = types.I32
	case "i64":
		kind = types.I64

	case "f32":
		kind = types.F32
	case "f64":
		kind = types.F64

	default:
		ok = false
	}

	if ok {
		if mutable {
			p.reportError(mutableRange, "primitive type cannot be mutable")
		}

		return p.primitiveType(kind)
	}

	return p.parseNonPrimitiveIdentifierType(mutable)
}

func (p *parser) selfType() (*ast.SelfType, int) {
	t := &ast.SelfType{}
	t.Range_ = p.advance().Range
	return t, -1
}

func (p *parser) parseNonPrimitiveIdentifierType(mutable bool) (i *ast.IdentifierType, recoverId int) {
	i = &ast.IdentifierType{}
	i.Range_.Start = p.current.Range.Start
	i.Mutable = mutable
	defer func() {
		i.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// Path
	if i.Path, recoverId = p.parseIdentifierPath(false); recoverId >= 0 {
		return
	}

	return
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

		if core.IsNil(a.Type) {
			a.Type = p.badType()
		}
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

func (p *parser) parseReferenceType() (t *ast.ReferenceType, recoverId int) {
	t = &ast.ReferenceType{}
	t.Range_.Start = p.current.Range.Start
	defer func() {
		t.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// 'mut'
	if p.current.Kind == lexer.Mut {
		p.advance()
		t.Mutable = true
	}

	// '&'
	if recoverId = p.expect(lexer.Ampersand, "expected '&' before pointee type"); recoverId >= 0 {
		return
	}

	// Pointee
	if t.Pointee, recoverId = p.parseType(); recoverId >= 0 {
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

	// 'mut'
	if p.current.Kind == lexer.Mut {
		p.advance()
		t.Mutable = true
	}

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

func (p *parser) parseFuncType() (f *ast.FuncType, recoverId int) {
	f = &ast.FuncType{}
	f.Range_.Start = p.current.Range.Start
	defer func() {
		f.Range_.End = p.previous.Range.End
	}()

	recoverId = -1

	// 'func'
	if recoverId = p.expect(lexer.Func, "expected 'func'"); recoverId >= 0 {
		return
	}

	// Signature
	if _, f.Params, f.VarArgs, f.Returns, recoverId = p.parseFuncSignature(false, false); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseOptionType() (o *ast.OptionType, recoverId int) {
	o = &ast.OptionType{}
	o.Range_.Start = p.current.Range.Start
	defer func() {
		o.Range_.End = p.previous.Range.End

		if core.IsNil(o.Type) {
			o.Type = p.badType()
		}
	}()

	recoverId = -1

	// '?'
	if recoverId = p.expect(lexer.QuestionMark, "expected '?'"); recoverId >= 0 {
		return
	}

	// Type
	if o.Type, recoverId = p.parseType(); recoverId >= 0 {
		return
	}

	return
}

func (p *parser) parseSliceType() (s *ast.SliceType, recoverId int) {
	s = &ast.SliceType{}
	s.Range_.Start = p.current.Range.Start
	defer func() {
		s.Range_.End = p.previous.Range.End

		if core.IsNil(s.Type) {
			s.Type = p.badType()
		}
	}()

	recoverId = -1

	// 'mut'
	if p.current.Kind == lexer.Mut {
		p.advance()
		s.Mutable = true
	}

	// '['
	if recoverId = p.expect(lexer.LeftBracket, "expected '['"); recoverId >= 0 {
		return
	}

	// ']'
	if recoverId = p.expect(lexer.RightBracket, "expected ']'"); recoverId >= 0 {
		return
	}

	// Type
	if s.Type, recoverId = p.parseType(); recoverId >= 0 {
		return
	}

	return
}
