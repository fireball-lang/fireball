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

// Combined

type CombinedScope struct {
	Scopes []Scope
}

func (c *CombinedScope) GetScope(name string) (Scope, bool) {
	for i := len(c.Scopes) - 1; i >= 0; i-- {
		if scope, ok := c.Scopes[i].GetScope(name); ok {
			return scope, true
		}
	}

	return nil, false
}

func (c *CombinedScope) GetSymbol(name string) (Symbol, bool) {
	for i := len(c.Scopes) - 1; i >= 0; i-- {
		if symbol, ok := c.Scopes[i].GetSymbol(name); ok {
			return symbol, true
		}
	}

	return Symbol{}, false
}
