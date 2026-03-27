package symbols

type Scope interface {
	Get(name string) (Symbol, bool)
}

// Simple

type SimpleScope []Symbol

func (s SimpleScope) Get(name string) (Symbol, bool) {
	for _, symbol := range s {
		if symbol.Name == name {
			return symbol, true
		}
	}

	return Symbol{}, false
}

// Stacked

type StackedScope struct {
	Scopes []Scope
}

func (c *StackedScope) Get(name string) (Symbol, bool) {
	for i := len(c.Scopes) - 1; i >= 0; i-- {
		if symbol, ok := c.Scopes[i].Get(name); ok {
			return symbol, true
		}
	}

	return Symbol{}, false
}
