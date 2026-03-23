package symbols

type Scope interface {
	Get(name string) (Symbol, bool)
}

// Simple

type SimpleScope []Symbol

func (s SimpleScope) Get(name string) (Symbol, bool) {
	for _, symbol := range s {
		if symbol.Decl.Name() == name {
			return symbol, true
		}
	}

	return Symbol{}, false
}

// Composed

type ComposedScope struct {
	Scopes []Scope
}

func (c *ComposedScope) Get(name string) (Symbol, bool) {
	for _, scope := range c.Scopes {
		if symbol, ok := scope.Get(name); ok {
			return symbol, true
		}
	}

	return Symbol{}, false
}
