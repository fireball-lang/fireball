package codegen

import "fireball/ir"

type symbolEntry struct {
	name  string
	value ir.Value
}

type symbolScope struct {
	symbols []symbolEntry
	blocks  []int
}

func (s *symbolScope) Push() {
	s.blocks = append(s.blocks, len(s.symbols))
}

func (s *symbolScope) Pop() {
	block := s.blocks[len(s.blocks)-1]

	s.blocks = s.blocks[:len(s.blocks)-1]
	s.symbols = s.symbols[:block]
}

func (s *symbolScope) Add(name string, value ir.Value) bool {
	if len(s.blocks) == 0 {
		panic("codegen.symbolScope.Add() - No block pushed")
	}

	for i := s.blocks[len(s.blocks)-1]; i < len(s.symbols); i++ {
		if s.symbols[i].name == name {
			return false
		}
	}

	s.symbols = append(s.symbols, symbolEntry{name, value})
	return true
}

func (s *symbolScope) Get(name string) ir.Value {
	for i := len(s.symbols) - 1; i >= 0; i-- {
		if s.symbols[i].name == name {
			return s.symbols[i].value
		}
	}

	panic("codegen.symbolScope.Get() - Entry '" + name + "' not found")
}
