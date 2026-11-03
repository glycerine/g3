package pdr_test

// Source pdr_sym_mux.cpp:93-109 records each newly appended declaration at
// its tuple position, including when one conversion creates several versions.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/pdr"
)

func TestSymbolMuxMultiSlotGrowthPreservesVersionIndices(t *testing.T) {
	command, manager, contexts := pdrContextFixture(t, 0)
	t.Cleanup(func() { pdrMust(t, command.Close()) })
	pdrMust(t, contexts.Close())
	boolean, err := manager.MkBoolSort()
	pdrMust(t, err)
	prefix := pdrFunction(t, manager, "growth", nil, boolean)
	mux, err := pdr.NewSymbolMux(manager)
	pdrMust(t, err)
	t.Cleanup(func() { pdrMust(t, mux.Close()) })
	tuple, err := mux.CreateTuple(prefix, nil, boolean, 1, nil)
	pdrMust(t, err)
	primary := tuple[0]
	var previous []ast.ASTHandle
	for _, target := range []uint32{4, 8} {
		last, err := mux.Convert(primary, 0, target)
		pdrMust(t, err)
		versions := make([]ast.ASTHandle, target+1)
		for index := uint32(0); index <= target; index++ {
			version, found, err := mux.TryByPrefix(prefix, index)
			pdrMust(t, err)
			if !found {
				t.Fatal("missing grown version", index)
			}
			versions[index] = version
			actual, found, err := mux.TryIndex(version)
			pdrMust(t, err)
			if !found || actual != index {
				t.Fatal("grown declaration has wrong source index", index, actual, found)
			}
			name, err := manager.GetASTName(version)
			pdrMust(t, err)
			text, present, err := manager.Symbols.BareString(name)
			pdrMust(t, err)
			want := "growth_n"
			if index != 0 {
				want = fmt.Sprintf("growth_%d", index-1)
			}
			if !present || text != want {
				t.Fatal("grown version name", index, text, want)
			}
			back, err := mux.Convert(version, index, 0)
			pdrMust(t, err)
			if back != primary {
				t.Fatal("grown version does not round trip", index)
			}
			if index < uint32(len(previous)) && version != previous[index] {
				t.Fatal("growth replaced an existing version", index)
			}
		}
		if versions[target] != last {
			t.Fatal("growth returned a different target version")
		}
		previous = versions
	}
	if _, err = mux.Convert(previous[4], 3, 0); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("conversion accepted a mismatched source index", err)
	}
}
