package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// smtcore_02-F7: core_hashtable::reset keeps the capacity unless more than
// three quarters of a table above 16 slots were free, in which case it halves.
func TestSeqObjHashtableResetKeepsOrHalvesCapacity(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkSort(symbol.Numerical(3100), ast.ASTSortInfo{})
	astTestMust(t, err)
	constants := make([]ast.ASTHandle, 20)
	for i := range constants {
		constants[i], err = m.MkConst(symbol.Numerical(uint32(3101+i)), sort)
		astTestMust(t, err)
	}
	h := newSeqObjHashtable(m)
	for _, c := range constants {
		astTestMust(t, h.insert(c))
	}
	capacity := len(h.table)
	astTestMust(t, h.remove(constants[0]))
	h.reset()
	if len(h.table) != capacity || h.size != 0 || h.numDeleted != 0 || len(h.entries()) != 0 {
		t.Fatal("reset of a dense table must keep its capacity", capacity, len(h.table), h.size, h.numDeleted)
	}
	h.table = make([]seqObjEntry, 64)
	for _, c := range constants[:5] {
		astTestMust(t, h.insert(c))
	}
	h.reset()
	if len(h.table) != 32 || h.size != 0 {
		t.Fatal("reset of a sparse table must halve it", len(h.table))
	}
	h.reset()
	if len(h.table) != 32 {
		t.Fatal("reset of an empty table is a no-op", len(h.table))
	}
}
