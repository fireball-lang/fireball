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

func (r Range) Contains(pos Pos) bool {
	if pos.Line < r.Start.Line || pos.Line > r.End.Line {
		return false
	}

	if pos.Line == r.Start.Line && pos.Column < r.Start.Column {
		return false
	}
	if pos.Line == r.End.Line && pos.Column > r.End.Column {
		return false
	}

	return true
}
