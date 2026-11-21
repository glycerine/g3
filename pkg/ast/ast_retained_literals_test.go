package ast

import (
	"errors"
	"testing"
)

func TestRetainedLiteralBufferOwnershipAndSigns(t *testing.T) {
	m, other := astTestManager(t), astTestManager(t)
	makeVar := func(owner *ASTManager) ASTHandle {
		t.Helper()
		sort, err := owner.MkBoolSort()
		astTestMust(t, err)
		x, err := owner.MkVar(301, sort)
		astTestMust(t, err)
		return x
	}
	x, foreign := makeVar(m), makeVar(other)
	astTestPin(t, other, foreign)
	if x.slot != foreign.slot || x.generation != foreign.generation {
		t.Fatal("fixture needs identical local references in different managers")
	}
	b, err := m.NewRetainedLiteralBuffer(66)
	astTestMust(t, err)
	if err = b.Retain(2, foreign, true); !errors.Is(err, ErrASTHandle) {
		t.Fatal("accepted a foreign atom", err)
	}
	astTestMust(t, b.Retain(0, x, true))
	astTestMust(t, b.Retain(63, x, false))
	astTestMust(t, b.Retain(64, x, true))
	if err = b.Retain(0, x, false); !errors.Is(err, ErrASTDomain) {
		t.Fatal("overwrote an occupied atom", err)
	}
	check := func(i int, want ASTHandle, negative bool) {
		t.Helper()
		h, sign, err := b.At(i)
		if err != nil || h != want || sign != negative {
			t.Fatalf("atom %d = %v, %v, %v", i, h, sign, err)
		}
	}
	b.Swap(0, 65)
	b.Swap(63, 64)
	b.Swap(63, 63)
	check(0, ASTHandle{}, false)
	check(63, x, true)
	check(64, x, false)
	check(65, x, true)
	// A callback can refuse to mutate the manager before consuming an owner.
	m.traceExecuting = true
	err = b.Release(63, 3)
	m.traceExecuting = false
	if !errors.Is(err, ErrASTPending) {
		t.Fatal("release during a trace callback", err)
	}
	check(63, x, true)
	astTestMust(t, b.Release(63, 2))
	check(63, ASTHandle{}, true)
	check(64, ASTHandle{}, false)
	n, err := m.GetRefCount(x)
	if err != nil || n != 1 {
		t.Fatal("release crossed its requested range", n, err)
	}
	astTestMust(t, b.Release(63, 3))
	if m.IsLiveHandle(x) {
		t.Fatal("last alias kept the atom alive")
	}
	next := makeVar(m)
	if next.slot != x.slot || next.generation == x.generation {
		t.Fatal("fixture did not recycle the atom slot", x, next)
	}
	if err = b.Retain(65, x, false); !errors.Is(err, ErrASTHandle) {
		t.Fatal("accepted a stale generation", err)
	}
	check(65, ASTHandle{}, true)
	astTestMust(t, b.Retain(65, next, false))
	check(65, next, false)
	astTestMust(t, b.Release(0, b.Len()))
}

func TestRetainedLiteralBufferRanges(t *testing.T) {
	m := astTestManager(t)
	if _, err := m.NewRetainedLiteralBuffer(-1); !errors.Is(err, ErrASTDomain) {
		t.Fatal("negative size", err)
	}
	if _, err := (*ASTManager)(nil).NewRetainedLiteralBuffer(1); !errors.Is(err, ErrASTHandle) {
		t.Fatal("nil manager", err)
	}
	b, err := m.NewRetainedLiteralBuffer(0)
	astTestMust(t, err)
	astTestMust(t, b.Release(0, 0))
	for _, r := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
		if err := b.Release(r[0], r[1]); !errors.Is(err, ErrASTDomain) {
			t.Fatal("accepted invalid range", r, err)
		}
	}
}
