package ir

import (
	"fireball/utils"
	"iter"
)

type linkedListNode[T any] interface {
	next() T
}

func iterLinkedList[T linkedListNode[T]](node T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for !utils.IsNil(node) {
			if !yield(node) {
				return
			}
			node = node.next()
		}
	}
}
