package core

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/petermattis/goid"
)

var parallelForCount atomic.Uint32

func ParallelFor[T any](items []T, fun func(int, T) error) error {
	if len(items) == 0 {
		return nil
	}

	defer Scope()()

	batch := parallelForCount.Add(1)
	numWorkers := min(runtime.GOMAXPROCS(-1), len(items))

	var wg sync.WaitGroup
	var currentIdx atomic.Int64

	var errsMutex sync.Mutex
	var errs []error

	for w := range numWorkers {
		workerTid := uint32(w + 1)

		wg.Go(func() {
			gid := goid.Get()

			// Register this worker's lane (PID = batch, TID = worker slot)
			registerWorker(gid, batch, workerTid)
			defer unregisterWorker(gid)

			for {
				i := int(currentIdx.Add(1) - 1)
				if i >= len(items) {
					break
				}

				if err := fun(i, items[i]); err != nil {
					errsMutex.Lock()
					errs = append(errs, err)
					errsMutex.Unlock()
				}

				runtime.Gosched()
			}
		})
	}

	wg.Wait()
	return errors.Join(errs...)
}
