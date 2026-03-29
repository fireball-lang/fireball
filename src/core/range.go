package core

type Pos struct {
	Line   uint32
	Column uint32
}

func (p Pos) Shift(columnOffset int) Pos {
	return Pos{
		Line:   p.Line,
		Column: uint32(int64(p.Column) + int64(columnOffset)),
	}
}

type Range struct {
	Start, End Pos
}
