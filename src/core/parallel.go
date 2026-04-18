package core

import (
	"errors"
	"runtime"
	"sync"

	"github.com/petermattis/goid"
)

var parallelForCount uint32

func ParallelFor[T any](items []T, fun func(int, T) error) error {
	defer Scope()()

	if goIds != nil {
		panic("core.ParallelFor() - Nested parallel fors not supported")
	}

	parallelForCount++

	var wg sync.WaitGroup

	var errsMutex sync.Mutex
	var errs []error

	tickets := make(chan uint32, runtime.GOMAXPROCS(-1))
	goIds = make(map[int64]goId)

	for i := range runtime.GOMAXPROCS(-1) {
		tickets <- uint32(i + 1)
	}

	for i, item := range items {
		ticket := <-tickets

		wg.Go(func() {
			defer func() { tickets <- ticket }()

			mutex.Lock()
			goIds[goid.Get()] = goId{parallelForCount, ticket}
			mutex.Unlock()

			err := fun(i, item)

			if err != nil {
				errsMutex.Lock()
				errs = append(errs, err)
				errsMutex.Unlock()
			}
		})
	}

	wg.Wait()

	goIds = nil

	return errors.Join(errs...)
}
