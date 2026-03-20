package core

type Ring[T any] struct {
	buffer []T

	writeIndex int
	readIndex  int
}

func NewRing[T any](size int) Ring[T] {
	return Ring[T]{
		buffer:     make([]T, size+1),
		writeIndex: 0,
		readIndex:  0,
	}
}

func (r *Ring[T]) Size() int {
	return (r.writeIndex + len(r.buffer) - r.readIndex) % len(r.buffer)
}

func (r *Ring[T]) Remaining() int {
	return (len(r.buffer) - 1) - ((r.writeIndex + len(r.buffer) - r.readIndex) % len(r.buffer))
}

func (r *Ring[T]) Add(item T) {
	nextWriteIndex := (r.writeIndex + 1) % len(r.buffer)

	if nextWriteIndex != r.readIndex {
		r.buffer[r.writeIndex] = item
		r.writeIndex = nextWriteIndex

		return
	}

	panic("Ring is full")
}

func (r *Ring[T]) Peek(offset int) T {
	if offset < 0 || offset >= r.Size() {
		panic("Ring peek offset out of bounds")
	}

	index := (r.readIndex + offset) % len(r.buffer)
	return r.buffer[index]
}

func (r *Ring[T]) TryGet() (T, bool) {
	var empty T

	if r.readIndex != r.writeIndex {
		item := r.buffer[r.readIndex]
		r.buffer[r.readIndex] = empty

		r.readIndex = (r.readIndex + 1) % len(r.buffer)

		return item, true
	}

	return empty, false
}
