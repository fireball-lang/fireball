package core

import (
	"bufio"
	"io"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/petermattis/goid"
)

type event struct {
	name string
	ph   byte
	pid  uint32
	tid  uint32
	ts   uint64
	dur  uint64
}

type goId struct {
	pid uint32
	tid uint32
}

type session struct {
	origin time.Time
}

var current atomic.Pointer[session]
var events []event

var norW io.Writer
var bufW *bufio.Writer
var wHasEvents = false

var mutex sync.Mutex

func StartProfiler() {
	mutex.Lock()
	defer mutex.Unlock()

	if current.Load() != nil {
		return
	}

	current.Store(&session{origin: time.Now()})
}

func SetProfilerOutput(w io.Writer) {
	mutex.Lock()
	defer mutex.Unlock()

	norW = w
	bufW = bufio.NewWriterSize(w, 1024*16)

	_, _ = bufW.WriteString("[\n")

	for _, e := range events {
		e.write()
	}
	events = nil
}

func EndProfiler() {
	mutex.Lock()
	defer mutex.Unlock()

	if current.Swap(nil) == nil {
		return
	}

	if bufW != nil {
		_, _ = bufW.WriteString("\n]\n")
		_ = bufW.Flush()

		if c, ok := norW.(io.Closer); ok {
			_ = c.Close()
		}
	}

	events = nil

	norW = nil
	bufW = nil
	wHasEvents = false
}

func Scope() func() {
	s := current.Load()
	if s == nil {
		return func() {}
	}

	startMono := time.Now()

	// Get caller name
	name := "unknown"

	var pc [1]uintptr
	n := runtime.Callers(2, pc[:])

	if n != 0 {
		frames := runtime.CallersFrames(pc[:n])
		frame, _ := frames.Next()

		name = frame.Function

		i := strings.LastIndexByte(name, '/')
		if i != -1 {
			name = name[i+1:]
		}
	}

	// Complete event
	return func() {
		end := time.Since(s.origin)
		begin := startMono.Sub(s.origin)
		recordEvent(s, name, begin, end)
	}
}

func recordEvent(s *session, name string, begin, end time.Duration) {
	goroutineID := goid.Get()

	ts := uint64(begin / time.Microsecond)
	dur := uint64(end/time.Microsecond) - ts

	e := event{
		name: name,
		ph:   'X',
		pid:  0,
		tid:  uint32(goroutineID),
		ts:   ts,
		dur:  dur,
	}

	if id, ok := getWorkerID(goroutineID); ok {
		e.pid = id.pid
		e.tid = id.tid
	}

	mutex.Lock()
	defer mutex.Unlock()

	if current.Load() != s {
		return
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
		_, _ = bufW.WriteString("\t{\"name\":")

		name := strconv.AppendQuote(buffer[:0], e.name)
		_, _ = bufW.Write(name)

		_, _ = bufW.WriteString(",\"ph\":\"")
	} else {
		_, _ = bufW.WriteString("\t{\"ph\":\"")
	}

	_ = bufW.WriteByte(e.ph)

	_, _ = bufW.WriteString("\",\"pid\":")

	pid := strconv.AppendUint(buffer[:0], uint64(e.pid), 10)
	_, _ = bufW.Write(pid)

	_, _ = bufW.WriteString(",\"tid\":")

	tid := strconv.AppendUint(buffer[:0], uint64(e.tid), 10)
	_, _ = bufW.Write(tid)

	_, _ = bufW.WriteString(",\"ts\":")

	ts := strconv.AppendUint(buffer[:0], e.ts, 10)
	_, _ = bufW.Write(ts)

	_, _ = bufW.WriteString(",\"dur\":")

	dur := strconv.AppendUint(buffer[:0], e.dur, 10)
	_, _ = bufW.Write(dur)

	_ = bufW.WriteByte('}')

	wHasEvents = true
}

// go routines

var (
	goIdMutex sync.RWMutex
	goIdMap   = make(map[int64]goId)
)

func registerWorker(gid int64, pid, tid uint32) {
	goIdMutex.Lock()
	goIdMap[gid] = goId{pid: pid, tid: tid}
	goIdMutex.Unlock()
}

func unregisterWorker(gid int64) {
	goIdMutex.Lock()
	delete(goIdMap, gid)
	goIdMutex.Unlock()
}

func getWorkerID(gid int64) (goId, bool) {
	goIdMutex.RLock()
	id, ok := goIdMap[gid]
	goIdMutex.RUnlock()
	return id, ok
}
