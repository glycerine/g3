package api

// Source: src/test/memory.cpp grows bit-vector sorts until allocation fails.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestMemorySourceGrowingBitVectorSortsRecoverAfterFailure(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_BV", nil)
	budget := f.m.Budget()
	var refs []ast.ASTRef
	t.Cleanup(func() {
		budget.FailAfter(-1)
		for i := len(refs) - 1; i >= 0; i-- {
			astTestMust(t, f.m.DelRef(&refs[i]))
		}
	})
	budget.FailAfter(30)
	var failure error
	var lastWidth uint32
	for width := uint32(1); width <= 10000; width++ {
		sort, err := f.c.BitVectorSort(width)
		if err != nil {
			failure = err
			break
		}
		ref, err := f.m.NewRef(sort)
		if err != nil {
			failure = err
			break
		}
		refs = append(refs, ref)
		lastWidth = width
	}
	if !errors.Is(failure, arena.ErrInjectedFailure) {
		t.Fatalf("growing bit-vector sorts did not reach an injected failure: width=%d err=%v", lastWidth, failure)
	}
	if len(refs) == 0 {
		t.Fatal("failure occurred before any sorts were created")
	}
	budget.FailAfter(-1)
	for _, index := range []int{0, len(refs) - 1} {
		sort, err := f.m.RefGet(refs[index])
		astTestMust(t, err)
		isBV, err := f.m.IsBitVectorSort(sort)
		astTestMust(t, err)
		if !isBV {
			t.Fatal("earlier sort was damaged", index)
		}
		width, err := f.c.BitVectorSortSize(sort)
		astTestMust(t, err)
		if width != uint32(index+1) {
			t.Fatalf("earlier sort %d has width %d", index, width)
		}
	}
	sort, err := f.c.BitVectorSort(lastWidth + 2)
	astTestMust(t, err)
	isBV, err := f.m.IsBitVectorSort(sort)
	astTestMust(t, err)
	if !isBV {
		t.Fatal("context did not recover after allocation failure")
	}
	width, err := f.c.BitVectorSortSize(sort)
	astTestMust(t, err)
	if width != lastWidth+2 {
		t.Fatalf("recovered sort has width %d, want %d", width, lastWidth+2)
	}
}
