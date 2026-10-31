package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestExprQEBranchVariablesAndPendingOrder(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	boolean := f.constant("branch-bool", f.boolean)
	bvSort := f.own(m.BitVectorSort(4))
	bv := f.constant("branch-bv", bvSort)
	integer := f.constant("branch-int", f.integer)
	state, err := newExprQEBranchVariables(m)
	fmMust(t, err)
	defer func() { fmMust(t, state.Close()) }()
	for _, v := range []ast.ASTHandle{boolean, bv, integer, integer} {
		fmMust(t, state.add(v))
	}
	for _, v := range []ast.ASTHandle{boolean, bv} {
		b, e := state.branch(v)
		fmMust(t, e)
		if b != v {
			t.Fatal("native branch variable changed")
		}
	}
	b, err := state.branch(integer)
	fmMust(t, err)
	if b == integer {
		t.Fatal("arithmetic branch did not receive a proxy")
	}
	bits, err := m.BitVectorSize(b)
	fmMust(t, err)
	if bits != 20 {
		t.Fatal("proxy width", bits)
	}
	if len(state.entries) != 3 || len(state.pending) != 4 {
		t.Fatal("duplicate registration changed branch mapping")
	}
	truth := f.own(m.MkTrue())
	root, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	fmMust(t, root.initialize(truth))
	fmMust(t, state.consume(root))
	if len(state.pending) != 0 || len(root.variables) != 4 {
		t.Fatal("pending variables were not consumed")
	}
	for i, want := range []ast.ASTHandle{integer, integer, bv, boolean} {
		got, e := m.RefGet(root.variables[i])
		fmMust(t, e)
		if got != want {
			t.Fatal("pending order", i)
		}
	}
	contains, err := state.entries[state.index[integer]].contains.Check(integer)
	fmMust(t, err)
	if !contains {
		t.Fatal("missing occurrence checker")
	}
}
