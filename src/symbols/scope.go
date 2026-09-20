package symbols

import (
	"cmp"
	"slices"
)

type Domain uint8

const (
	Type Domain = 1 << iota
	Variable
	Function
)

type Scope interface {
	GetScope(name string) (Scope, bool)
	GetSymbol(domain Domain, name string) (Symbol, bool)
}

// Symbol

type SymbolScope []Symbol

func (s SymbolScope) GetScope(_ string) (Scope, bool) {
	return nil, false
}

func (s SymbolScope) GetSymbol(domain Domain, name string) (Symbol, bool) {
	for _, symbol := range s {
		if symbol.Kind.IsInDomain(domain) && symbol.Name == name {
			return symbol, true
		}
	}

	return Symbol{}, false
}

// Binary

type BinaryScope []Symbol

func NewBinaryScope(symbols []Symbol) BinaryScope {
	symbols = slices.Clone(symbols)

	slices.SortFunc(symbols, func(a, b Symbol) int {
		if c := cmp.Compare(a.Name, b.Name); c != 0 {
			return c
		}

		return cmp.Compare(a.Kind.Domain(), b.Kind.Domain())
	})

	return symbols
}

func (b BinaryScope) GetScope(_ string) (Scope, bool) {
	return nil, false
}

func (b BinaryScope) GetSymbol(domain Domain, name string) (Symbol, bool) {
	index, ok := slices.BinarySearchFunc(b, name, func(s Symbol, target string) int {
		return cmp.Compare(s.Name, target)
	})

	if !ok {
		return Symbol{}, false
	}

	for i := index; i < len(b) && b[i].Name == name; i++ {
		if b[i].Kind.IsInDomain(domain) {
			return b[i], true
		}
	}

	return Symbol{}, false
}

// Basic

type BasicScope struct {
	scopes  map[string]Scope
	symbols []Symbol
}

func NewBasicScope() *BasicScope {
	return &BasicScope{
		scopes:  make(map[string]Scope),
		symbols: nil,
	}
}

func (b *BasicScope) AddScope(name string, scope Scope) bool {
	if _, ok := b.scopes[name]; ok {
		return false
	}

	b.scopes[name] = scope
	return true
}

func (b *BasicScope) AddSymbol(symbol Symbol) {
	b.symbols = append(b.symbols, symbol)
}

func (b *BasicScope) GetScope(name string) (Scope, bool) {
	scope, ok := b.scopes[name]
	return scope, ok
}

func (b *BasicScope) GetSymbol(domain Domain, name string) (Symbol, bool) {
	for _, symbol := range b.symbols {
		if symbol.Kind.IsInDomain(domain) && symbol.Name == name {
			return symbol, true
		}
	}

	return Symbol{}, false
}

// ScopeStack

type ScopeStack struct {
	scopes []Scope
}

func (s *ScopeStack) Push(scope Scope) {
	s.scopes = append(s.scopes, scope)
}

func (s *ScopeStack) Pop() {
	s.scopes = s.scopes[:len(s.scopes)-1]
}

func (s *ScopeStack) ValidateEmpty() {
	if len(s.scopes) != 0 {
		panic("symbols.ScopeStack.ValidateEmpty() - Scope stack is not empty, missing Pop() call")
	}
}

func (s *ScopeStack) GetScope(name string) (Scope, bool) {
	for i := len(s.scopes) - 1; i >= 0; i-- {
		if scope, ok := s.scopes[i].GetScope(name); ok {
			return scope, true
		}
	}

	return nil, false
}

func (s *ScopeStack) GetSymbol(domain Domain, name string) (Symbol, bool) {
	for i := len(s.scopes) - 1; i >= 0; i-- {
		if symbol, ok := s.scopes[i].GetSymbol(domain, name); ok {
			return symbol, true
		}
	}

	return Symbol{}, false
}
