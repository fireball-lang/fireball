package core

type Pos struct {
	Line   uint32
	Column uint32
}

type Range struct {
	Start, End Pos
}
