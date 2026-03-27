package symbols

type BlockScope struct {
	symbols []Symbol
	blocks  []int
}

func (l *BlockScope) Push() {
	l.blocks = append(l.blocks, len(l.symbols))
}

func (l *BlockScope) Pop() {
	block := l.blocks[len(l.blocks)-1]

	l.blocks = l.blocks[:len(l.blocks)-1]
	l.symbols = l.symbols[:block]
}

func (l *BlockScope) Add(symbol Symbol) bool {
	if len(l.blocks) == 0 {
		panic("symbols.BlockScope.Add() - No block pushed")
	}

	for i := l.blocks[len(l.blocks)-1]; i < len(l.symbols); i++ {
		if l.symbols[i].Name == symbol.Name {
			return false
		}
	}

	l.symbols = append(l.symbols, symbol)
	return true
}

func (l *BlockScope) Get(name string) (Symbol, bool) {
	for i := len(l.symbols) - 1; i >= 0; i-- {
		if l.symbols[i].Name == name {
			return l.symbols[i], true
		}
	}

	return Symbol{}, false
}
