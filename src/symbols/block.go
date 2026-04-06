package symbols

type BlockScope struct {
	symbols []Symbol
	blocks  []int
}

func (b *BlockScope) Push() {
	b.blocks = append(b.blocks, len(b.symbols))
}

func (b *BlockScope) Pop() {
	block := b.blocks[len(b.blocks)-1]

	b.blocks = b.blocks[:len(b.blocks)-1]
	b.symbols = b.symbols[:block]
}

func (b *BlockScope) Add(symbol Symbol) bool {
	if len(b.blocks) == 0 {
		panic("symbols.BlockScope.Add() - No block pushed")
	}

	for i := b.blocks[len(b.blocks)-1]; i < len(b.symbols); i++ {
		if b.symbols[i].Name == symbol.Name {
			return false
		}
	}

	b.symbols = append(b.symbols, symbol)
	return true
}

func (b *BlockScope) GetScope(_ string) (Scope, bool) {
	return nil, false
}

func (b *BlockScope) GetSymbol(name string) (Symbol, bool) {
	for i := len(b.symbols) - 1; i >= 0; i-- {
		if b.symbols[i].Name == name {
			return b.symbols[i], true
		}
	}

	return Symbol{}, false
}
