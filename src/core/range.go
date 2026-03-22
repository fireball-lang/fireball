package core

type Pos struct {
	Line   uint16
	Column uint16
}

type Range struct {
	Start, End Pos
}
