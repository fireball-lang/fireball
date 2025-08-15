package profiler

import (
	"bufio"
	"io"
	"log"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var writer *bufio.Writer
var sb strings.Builder
var hasEvent bool

func Start(w io.Writer) (func() error, error) {
	writer = bufio.NewWriter(w)
	hasEvent = false

	_, _ = writer.WriteString("[\n")

	return func() error {
		_, _ = writer.WriteString("\n]\n")

		err := writer.Flush()
		writer = nil

		return err
	}, nil
}

func EventNamed(name string) func() {
	if writer == nil {
		return func() {}
	}

	writeEvent(name, 'B')

	return func() {
		writeEvent(name, 'E')
	}
}

func Event() func() {
	if writer == nil {
		return func() {}
	}

	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		log.Fatalln("profiler.Event() - Failed to get caller information")
	}

	f := runtime.FuncForPC(pc)
	name := path.Base(f.Name())

	writeEvent(name, 'B')

	return func() {
		writeEvent(name, 'E')
	}
}

func writeEvent(name string, ph rune) {
	var buffer [64]byte
	ts := strconv.AppendInt(buffer[0:0], time.Now().UnixMicro(), 10)

	if hasEvent {
		sb.WriteString(",\n")
	}

	sb.WriteString("\t{ \"name\": \"")
	sb.WriteString(name)
	sb.WriteString("\", ")

	sb.WriteString("\"ph\": \"")
	sb.WriteRune(ph)
	sb.WriteString("\", ")

	sb.WriteString("\"pid\": 0, ")

	sb.WriteString("\"tid\": 0,")

	sb.WriteString("\"ts\": ")
	sb.Write(ts)

	sb.WriteString(" }")

	_, _ = writer.WriteString(sb.String())
	sb.Reset()

	hasEvent = true
}
