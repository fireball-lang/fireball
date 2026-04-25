package lsp

import (
	"bytes"
	"fireball/project"
	"io"
	"slices"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/owenrumney/go-lsp/lsp"
)

type Source struct {
	lines [][]byte
}

func NewSource(prev project.Source) *Source {
	reader := prev.Get()

	//goland:noinspection GoUnhandledErrorResult
	defer reader.Close()

	b, err := io.ReadAll(reader)
	if err != nil {
		panic(err.Error())
	}

	return &Source{
		lines: bytes.SplitAfter(b, []byte{'\n'}),
	}
}

func (l *Source) Apply(change lsp.TextDocumentContentChangeEvent) {
	// Full
	if change.Range == nil {
		l.lines = bytes.SplitAfter([]byte(change.Text), []byte{'\n'})
		return
	}

	// Incremental
	startLine := change.Range.Start.Line
	endLine := change.Range.End.Line

	startByte := utf16OffsetToBytes(l.lines[startLine], change.Range.Start.Character)
	endByte := utf16OffsetToBytes(l.lines[endLine], change.Range.End.Character)

	prefix := l.lines[startLine][:startByte]
	suffix := l.lines[endLine][endByte:]

	newParts := bytes.SplitAfter([]byte(change.Text), []byte{'\n'})
	newParts[0] = append(append([]byte(nil), prefix...), newParts[0]...)
	newParts[len(newParts)-1] = append(newParts[len(newParts)-1], suffix...)

	l.lines = slices.Replace(l.lines, startLine, endLine+1, newParts...)
}

func utf16OffsetToBytes(line []byte, utf16Offset int) int {
	byteOffset := 0
	utf16Count := 0

	for byteOffset < len(line) && utf16Count < utf16Offset {
		r, size := utf8.DecodeRune(line[byteOffset:])
		utf16Count += utf16.RuneLen(r)
		byteOffset += size
	}

	return byteOffset
}

func (l *Source) Get() io.ReadCloser {
	return &lineReader{lines: l.lines}
}

type lineReader struct {
	lines [][]byte

	lineI int
	charI int
}

func (l *lineReader) Read(p []byte) (int, error) {
	if l.lineI >= len(l.lines) {
		return 0, io.EOF
	}

	line := l.lines[l.lineI][l.charI:]
	n := copy(p, line)

	l.charI += n

	if len(line)-n == 0 {
		l.lineI++
		l.charI = 0
	}

	return n, nil
}

func (l *lineReader) Close() error {
	return nil
}
