package qe

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestExprQETreeBranchesPurityAndLeaves(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	truth := f.own(m.MkTrue())
	x := f.constant("tree-x", f.boolean)
	y := f.constant("tree-y", f.boolean)
	p := f.constant("leaf-p", f.boolean)
	q := f.constant("leaf-q", f.boolean)
	root, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	fmMust(t, root.initialize(truth))
	fmMust(t, root.consumeVariables([]ast.ASTHandle{x, y}))
	first, err := m.RefGet(root.variables[0])
	fmMust(t, err)
	if first != y {
		t.Fatal("variables were not consumed in reverse")
	}
	fmMust(t, root.selectVariable(x, big.NewRat(2, 1)))
	a, err := root.addBranch(f.r.config, big.NewRat(0, 1), truth)
	fmMust(t, err)
	b, err := root.addBranch(f.r.config, big.NewRat(1, 1), truth)
	fmMust(t, err)
	if _, err = root.addBranch(f.r.config, big.NewRat(1, 1), truth); err == nil {
		t.Fatal("duplicate branch accepted")
	}
	if _, err = root.addBranch(f.r.config, big.NewRat(2, 1), truth); err == nil {
		t.Fatal("out-of-range branch accepted")
	}
	fmMust(t, a.initialize(p))
	fmMust(t, b.initialize(q))
	fmMust(t, a.deleteVariable(y))
	fmMust(t, b.deleteVariable(y))
	if a.isPure() || !root.isPure() {
		t.Fatal("purity did not follow node ancestry")
	}
	leaves, err := root.leaves()
	fmMust(t, err)
	defer func() {
		for i := range leaves {
			fmMust(t, m.DelRef(&leaves[i]))
		}
	}()
	if len(leaves) != 2 {
		t.Fatalf("got %d leaves", len(leaves))
	}
	for i, want := range []ast.ASTHandle{q, p} {
		h, e := m.RefGet(leaves[i])
		fmMust(t, e)
		if h != want {
			t.Fatal("leaf order", i)
		}
	}
	unit, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, unit.Close()) }()
	fmMust(t, unit.initialize(truth))
	child, err := unit.addUnit(f.r.config, p)
	fmMust(t, err)
	if !unit.isUnit() || child.parent != unit {
		t.Fatal("unit branch structure")
	}
	if _, err = unit.addUnit(f.r.config, q); err == nil {
		t.Fatal("second unit branch accepted")
	}
}
