package ast

import (
	"fireball/utils"
	"strings"
)

// PathLike

type PathLike interface {
	SegmentCount() int
	SegmentAt(index int) string
}

func PathEquals[T1, T2 PathLike](p1 T1, p2 T2) bool {
	if utils.IsNil(p1) || utils.IsNil(p2) || p1.SegmentCount() != p2.SegmentCount() {
		return false
	}

	for i := 0; i < p1.SegmentCount(); i++ {
		if p1.SegmentAt(i) != p2.SegmentAt(i) {
			return false
		}
	}

	return true
}

func PathWriteString[T PathLike](sb *strings.Builder, path T) {
	for i := 0; i < path.SegmentCount(); i++ {
		if i > 0 {
			sb.WriteRune(':')
		}

		sb.WriteString(path.SegmentAt(i))
	}
}

func PathString[T PathLike](path T) string {
	var sb strings.Builder
	PathWriteString(&sb, path)

	return sb.String()
}

// StringPath

type StringPath struct {
	Segments []string
}

func (s StringPath) SegmentCount() int {
	return len(s.Segments)
}

func (s StringPath) SegmentAt(index int) string {
	return s.Segments[index]
}
