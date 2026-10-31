package qe

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestExprQETreeConstraintUsesSelectedAncestry(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	k := qsatKernelOwner(t, f)
	solver, err := k.solver()
	fmMust(t, err)
	truth := f.own(m.MkTrue())
	p := f.constant("constraint-p", f.boolean)
	q := f.constant("constraint-q", f.boolean)
	root, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	fmMust(t, root.initialize(truth))
	fmMust(t, root.consumeVariables([]ast.ASTHandle{p}))
	fmMust(t, root.selectVariable(p, big.NewRat(2, 1)))
	child, err := root.addBranch(f.r.config, big.NewRat(0, 1), p)
	fmMust(t, err)
	notQ := f.own(m.MkNot(q))
	notP := f.own(m.MkNot(p))
	fmMust(t, solver.Push())
	fmMust(t, assertExprQETreeConstraint(child, solver, true, q))
	fmMust(t, solver.Push())
	fmMust(t, solver.AssertFormula(p))
	fmMust(t, solver.AssertFormula(notQ))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("current assignment did not guard clause")
	}
	fmMust(t, solver.Pop(1))
	fmMust(t, solver.Push())
	fmMust(t, solver.AssertFormula(notP))
	fmMust(t, solver.AssertFormula(notQ))
	status, err = solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.True {
		t.Fatal("constraint affected another branch")
	}
	fmMust(t, solver.Pop(1))
	fmMust(t, solver.Pop(1))
	fmMust(t, solver.Push())
	fmMust(t, assertExprQETreeConstraint(child, solver, false, q))
	fmMust(t, solver.AssertFormula(notQ))
	status, err = solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("parent-only constraint did not propagate")
	}
	fmMust(t, solver.Pop(1))
}
