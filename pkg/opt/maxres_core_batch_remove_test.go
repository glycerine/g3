package opt

// Source: src/opt/maxres.cpp remove_soft while collecting disjoint cores.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMaxResRemoveSoftSwapsWithLast(t *testing.T) {
	manager := newWeightedSoftFixture(t).manager
	a := maxResTestBool(t, manager, "batch-remove-a")
	b := maxResTestBool(t, manager, "batch-remove-b")
	c := maxResTestBool(t, manager, "batch-remove-c")
	d := maxResTestBool(t, manager, "batch-remove-d")
	values := []ast.ASTHandle{a, b, c, d, b}
	remaining := removeSoft([]ast.ASTHandle{b, d}, values)
	if len(remaining) != 2 || remaining[0] != a || remaining[1] != c {
		t.Fatal("remaining assumptions", remaining)
	}
}
