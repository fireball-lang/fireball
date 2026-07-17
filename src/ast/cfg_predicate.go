package ast

import (
	"iter"
)

type CfgPredicate interface {
	Node

	isCfgPredicate()
}

// TargetOsCfg

type TargetOsKind uint8

const (
	WindowsOs TargetOsKind = iota
	Linux
	MacOS
)

type TargetOsCfg struct {
	baseNode

	Not  bool
	Kind TargetOsKind
}

func (t *TargetOsCfg) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (t *TargetOsCfg) isCfgPredicate() {}

// TargetFamilyCfg

type TargetFamilyKind uint8

const (
	WindowsFamily TargetFamilyKind = iota
	Unix
)

type TargetFamilyCfg struct {
	baseNode

	Not  bool
	Kind TargetFamilyKind
}

func (t *TargetFamilyCfg) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (t *TargetFamilyCfg) isCfgPredicate() {}

// NotCfg

type NotCfg struct {
	baseNode

	Predicate CfgPredicate
}

func (n *NotCfg) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if !yield(n.Predicate) {
			return
		}
	}
}

func (n *NotCfg) isCfgPredicate() {}

// AllCfg

type AllCfg struct {
	baseNode

	Predicates []CfgPredicate
}

func (a *AllCfg) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, predicate := range a.Predicates {
			if !yield(predicate) {
				return
			}
		}
	}
}

func (a *AllCfg) isCfgPredicate() {}

// AnyCfg

type AnyCfg struct {
	baseNode

	Predicates []CfgPredicate
}

func (a *AnyCfg) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for _, predicate := range a.Predicates {
			if !yield(predicate) {
				return
			}
		}
	}
}

func (a *AnyCfg) isCfgPredicate() {}

// BadCfg

type BadCfg struct {
	baseNode
}

func (b *BadCfg) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {}
}

func (b *BadCfg) isCfgPredicate() {}
