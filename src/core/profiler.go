package core

import (
	"bufio"
	"io"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/petermattis/goid"
)

type event struct {
	name string
	ph   byte
	pid  uint32
	tid  uint32
	ts   uint64
}

type goId struct {
	pid uint32
	tid uint32
}

var start uint64
var events []event

var norW io.Writer
var bufW *bufio.Writer
var wHasEvents = false

var mutex sync.Mutex
var goIds map[int64]goId

func StartProfiler() {
	start = uint64(time.Now().UnixMicro())
}

func SetProfilerOutput(w io.Writer) {
	norW = w
	bufW = bufio.NewWriter(w)

	_, _ = bufW.WriteString("[\n")

	for _, e := range events {
		e.write()
	}
	events = nil
}

func EndProfiler() {
	_, _ = bufW.WriteString("\n]\n")

	_ = bufW.Flush()

	if c, ok := norW.(io.Closer); ok {
		_ = c.Close()
	}

	start = 0
}

func Scope() func() {
	if start == 0 {
		return func() {}
	}

	// Get caller name
	name := ""

	var pc [1]uintptr
	n := runtime.Callers(2, pc[:])

	if n != 0 {
		frames := runtime.CallersFrames(pc[:n])
		frame, _ := frames.Next()

		name = frame.Function

		i := strings.IndexByte(name, '/')
		if i != -1 {
			name = name[i+1:]
		}
	}

	// Begin event
	recordEvent(name, 'B')

	// End event
	return func() {
		recordEvent(name, 'E')
	}
}

func recordEvent(name string, ph byte) {
	goroutineID := goid.Get()

	e := event{
		name: name,
		ph:   ph,
		pid:  0,
		tid:  uint32(goroutineID),
		ts:   uint64(time.Now().UnixMicro()) - start,
	}

	mutex.Lock()
	defer mutex.Unlock()

	if goIds != nil {
		id := goIds[goroutineID]

		e.pid = id.pid
		e.tid = id.tid
	}

	if bufW != nil {
		e.write()
		return
	}

	events = append(events, e)
}

func (e event) write() {
	var buffer [64]byte

	if wHasEvents {
		_, _ = bufW.WriteString(",\n")
	}

	if e.name != "" {
		_, _ = bufW.WriteString("\t{\"name\":\"")
		_, _ = bufW.WriteString(e.name)
		_, _ = bufW.WriteString("\",\"ph\":\"")
	} else {
		_, _ = bufW.WriteString("\t{\"ph\":\"")
	}

	_ = bufW.WriteByte(e.ph)

	_, _ = bufW.WriteString("\",\"pid\":")

	size := len(strconv.AppendInt(buffer[0:0], int64(e.pid), 10))
	_, _ = bufW.Write(buffer[:size])

	_, _ = bufW.WriteString(",\"tid\":")

	size = len(strconv.AppendUint(buffer[0:0], uint64(e.tid), 10))
	_, _ = bufW.Write(buffer[:size])

	_, _ = bufW.WriteString(",\"ts\":")

	size = len(strconv.AppendUint(buffer[0:0], e.ts, 10))
	_, _ = bufW.Write(buffer[:size])

	_ = bufW.WriteByte('}')

	wHasEvents = true
}
