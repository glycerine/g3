package tactic

// Source: src/test/parray.cpp tst5, adapted to the live owning Goal slice.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/clibrand"
)

func TestPersistentArraySourceOwningSnapshot(t *testing.T) {
	m := astTestManager(t)
	original := goalTestNew(t, m, false, false)
	snapshot := goalTestNew(t, m, false, false)
	values := make([]ast.ASTHandle, 100)
	for i := range values {
		values[i] = goalTestAtom(t, m, fmt.Sprintf("source-parray-%d", i))
		astTestMust(t, original.Assert(values[i]))
	}
	astTestMust(t, original.CopyTo(snapshot))
	expected := append([]ast.ASTHandle(nil), values...)
	var rng clibrand.GLibc31
	for i := 0; i < 1000000; i++ {
		index := i % len(values)
		replacement := values[rng.Next()%uint32(len(values))]
		astTestMust(t, original.Update(uint32(index), replacement, ast.ASTHandle{}, ast.ASTDependency{}))
		expected[index] = replacement
	}
	goalTestForms(t, snapshot, values...)
	goalTestForms(t, original, expected...)
}
