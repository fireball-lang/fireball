package lsp

import (
	"sync"

	"github.com/petermattis/goid"
)

type reentrantRWMutex struct {
	mutex         sync.Mutex
	cond          *sync.Cond
	readers       map[int64]int
	writer        bool
	writerWaiting bool
}

func newReentrantRWMutex() *reentrantRWMutex {
	m := &reentrantRWMutex{
		readers: make(map[int64]int),
	}

	m.cond = sync.NewCond(&m.mutex)

	return m
}

func (m *reentrantRWMutex) Lock() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.writerWaiting = true
	defer func() { m.writerWaiting = false }()

	for m.writer || len(m.readers) > 0 {
		m.cond.Wait()
	}

	m.writer = true
}

func (m *reentrantRWMutex) Unlock() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.writer {
		panic("lsp.reentrantRWMutex.Unlock() - Lock not held")
	}

	m.writer = false
	m.cond.Broadcast()
}

func (m *reentrantRWMutex) RLock() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	id := goid.Get()

	// Reentrant read acquisition
	if count, ok := m.readers[id]; ok {
		m.readers[id] = count + 1
		return
	}

	for m.writer || m.writerWaiting {
		m.cond.Wait()
	}

	m.readers[id] = 1
}

func (m *reentrantRWMutex) RUnlock() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	id := goid.Get()

	count, ok := m.readers[id]
	if !ok {
		panic("lsp.reentrantRWMutex.RUnlock() - Lock not held")
	}

	if count > 1 {
		m.readers[id] = count - 1
		return
	}

	delete(m.readers, id)
	m.cond.Broadcast()
}

func (m *reentrantRWMutex) RLocker() sync.Locker {
	return (*rlocker)(m)
}

type rlocker reentrantRWMutex

func (r *rlocker) Lock()   { (*reentrantRWMutex)(r).RLock() }
func (r *rlocker) Unlock() { (*reentrantRWMutex)(r).RUnlock() }
