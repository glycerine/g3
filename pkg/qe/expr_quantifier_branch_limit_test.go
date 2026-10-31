package qe

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestExprQEBranchLimitRestrictsArithmeticProxy(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	k := qsatKernelOwner(t, f)
	solver, err := k.solver()
	fmMust(t, err)
	variable := f.constant("limit-x", f.integer)
	truth := f.own(m.MkTrue())
	vars, err := newExprQEBranchVariables(m)
	fmMust(t, err)
	defer func() { fmMust(t, vars.Close()) }()
	fmMust(t, vars.add(variable))
	root, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	fmMust(t, root.initialize(truth))
	fmMust(t, vars.consume(root))
	fmMust(t, constrainExprQEBranch(root, vars, solver, variable, big.NewRat(3, 1)))
	proxy, err := vars.branch(variable)
	fmMust(t, err)
	for _, tc := range []struct {
		n    int64
		want lbool.LBool
	}{{2, lbool.True}, {3, lbool.False}} {
		eq, e := exprQEBranchEquality(m, proxy, big.NewRat(tc.n, 1))
		fmMust(t, e)
		h, e := m.RefGet(eq)
		fmMust(t, e)
		fmMust(t, solver.Push())
		fmMust(t, solver.AssertFormula(h))
		status, e := solver.CheckSat(nil)
		fmMust(t, e)
		if status != tc.want {
			t.Fatalf("proxy %d: %v", tc.n, status)
		}
		fmMust(t, solver.Pop(1))
		fmMust(t, m.DelRef(&eq))
	}
	if index, err := root.variableIndex(variable); err != nil || index >= 0 {
		t.Fatal("selected variable was not removed")
	}
}
