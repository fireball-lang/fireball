package symbols

type Scope interface {
	GetScope(name string) (Scope, bool)
	GetSymbol(name string) (Symbol, bool)
}

// Symbol

type SymbolScope []Symbol

func (s SymbolScope) GetScope(_ string) (Scope, bool) {
	return nil, false
}

func (s SymbolScope) GetSymbol(name string) (Symbol, bool) {
	for _, symbol := range s {
		if symbol.Name == name {
			return symbol, true
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

func (b *BasicScope) GetSymbol(name string) (Symbol, bool) {
	for _, symbol := range b.symbols {
		if symbol.Name == name {
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

func (s *ScopeStack) GetSymbol(name string) (Symbol, bool) {
	for i := len(s.scopes) - 1; i >= 0; i-- {
		if symbol, ok := s.scopes[i].GetSymbol(name); ok {
			return symbol, true
		}
	}

	return Symbol{}, false
}
