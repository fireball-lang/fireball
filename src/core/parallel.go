package core

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/petermattis/goid"
)

var parallelForActive atomic.Bool
var parallelForCount atomic.Uint32

func ParallelFor[T any](items []T, fun func(int, T) error) error {
	defer Scope()()

	if !parallelForActive.CompareAndSwap(false, true) {
		panic("core.ParallelFor() - Nested parallel fors not supported")
	}

	defer parallelForActive.Store(false)

	batch := parallelForCount.Add(1)

	var wg sync.WaitGroup

	var errsMutex sync.Mutex
	var errs []error

	numWorkers := runtime.GOMAXPROCS(-1)

	mutex.Lock()
	goIds = make(map[int64]goId)
	mutex.Unlock()

	indexes := make(chan int)

	for w := range numWorkers {
		wg.Go(func() {
			mutex.Lock()
			goIds[goid.Get()] = goId{batch, uint32(w + 1)}
			mutex.Unlock()

			for i := range indexes {
				err := fun(i, items[i])

				if err != nil {
					errsMutex.Lock()
					errs = append(errs, err)
					errsMutex.Unlock()
				}
			}
		})
	}

	for i := range items {
		indexes <- i
	}
	close(indexes)

	wg.Wait()

	mutex.Lock()
	goIds = nil
	mutex.Unlock()

	return errors.Join(errs...)
}
