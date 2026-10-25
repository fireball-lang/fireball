package lsp

import (
	"sync"

	"github.com/petermattis/goid"
)

type reentrantRWMutex struct {
	mutex          sync.Mutex
	cond           *sync.Cond
	readers        map[int64]int
	writerID       int64
	writerCount    int
	writerReads    int
	writersWaiting int
}

func newReentrantRWMutex() *reentrantRWMutex {
	m := &reentrantRWMutex{readers: make(map[int64]int)}
	m.cond = sync.NewCond(&m.mutex)
	return m
}

func (m *reentrantRWMutex) Lock() {
	id := goid.Get()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.writerCount > 0 && m.writerID == id {
		m.writerCount++
		return
	}
	if m.readers[id] > 0 {
		panic("lsp.reentrantRWMutex.Lock() - cannot upgrade read lock to write lock")
	}

	m.writersWaiting++
	for m.writerCount > 0 || len(m.readers) > 0 {
		m.cond.Wait()
	}
	m.writersWaiting--

	m.writerID = id
	m.writerCount = 1
}

func (m *reentrantRWMutex) Unlock() {
	id := goid.Get()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.writerCount == 0 || m.writerID != id {
		panic("lsp.reentrantRWMutex.Unlock() - write lock not held by this goroutine")
	}

	m.writerCount--
	if m.writerCount == 0 {
		m.writerID = 0

		if m.writerReads > 0 {
			// Downgrade: transfer writerReads into the standard readers map
			m.readers[id] = m.writerReads
			m.writerReads = 0
		}

		m.cond.Broadcast()
	}
}

func (m *reentrantRWMutex) RLock() {
	id := goid.Get()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Writer taking a nested read lock.
	if m.writerCount > 0 && m.writerID == id {
		m.writerReads++
		return
	}

	// Reentrant read: bypass writer preference to avoid self-deadlock.
	if count := m.readers[id]; count > 0 {
		m.readers[id] = count + 1
		return
	}

	for m.writerCount > 0 || m.writersWaiting > 0 {
		m.cond.Wait()
	}
	m.readers[id] = 1
}

func (m *reentrantRWMutex) RUnlock() {
	id := goid.Get()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.writerCount > 0 && m.writerID == id && m.writerReads > 0 {
		m.writerReads--
		return
	}

	count, ok := m.readers[id]
	if !ok {
		panic("lsp.reentrantRWMutex.RUnlock() - read lock not held by this goroutine")
	}
	if count > 1 {
		m.readers[id] = count - 1
		return
	}

	delete(m.readers, id)

	if len(m.readers) == 0 {
		m.cond.Broadcast()
	}
}

func (m *reentrantRWMutex) RLocker() sync.Locker { return (*rlocker)(m) }

type rlocker reentrantRWMutex

func (r *rlocker) Lock()   { (*reentrantRWMutex)(r).RLock() }
func (r *rlocker) Unlock() { (*reentrantRWMutex)(r).RUnlock() }
