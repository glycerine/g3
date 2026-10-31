package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSATSolverContextNNFAndConstraints(t *testing.T) {
	f := qsatKernelFixture(t)
	k := qsatKernelOwner(t, f)
	solver, err := k.solver()
	fmMust(t, err)
	context, err := newSATSolverContext(f.r.manager, solver, f.r.config)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, context.Close()) })
	x := f.constant("qe-sat-x", f.boolean)
	y := f.constant("qe-sat-y", f.boolean)
	z := f.constant("qe-sat-z", f.boolean)
	fmMust(t, context.addVariable(x))
	fmMust(t, context.addVariable(y))
	fmMust(t, context.addVariable(z))
	present, err := context.contains[1].Check(y)
	fmMust(t, err)
	if !present {
		t.Fatal("variable occurrence helper")
	}
	fmMust(t, context.eliminateVariable(1, x, ast.ASTHandle{}))
	if len(context.variables) != 2 {
		t.Fatal("variable removal", len(context.variables))
	}
	first, err := f.r.manager.RefGet(context.variables[0])
	fmMust(t, err)
	second, err := f.r.manager.RefGet(context.variables[1])
	fmMust(t, err)
	if first != x || second != z {
		t.Fatal("ordered erase", first, second)
	}
	if err = context.eliminateVariable(2, x, ast.ASTHandle{}); err == nil {
		t.Fatal("variable bounds")
	}
	implication := f.own(f.r.manager.MkImplies(x, y))
	fmMust(t, context.initializeFormula(implication))
	formula, err := f.r.manager.RefGet(context.formula)
	fmMust(t, err)
	assertQENNFConnectives(t, f.r.manager, formula)
	negative, err := context.atoms.Negative(x)
	fmMust(t, err)
	positive, err := context.atoms.Positive(y)
	fmMust(t, err)
	if !negative || !positive {
		t.Fatal("default relevance and polarities", negative, positive)
	}
	fmMust(t, context.initializeFormula(x))
	negative, err = context.atoms.Negative(x)
	fmMust(t, err)
	if negative {
		t.Fatal("stale negative atom")
	}
	positive, err = context.atoms.Positive(x)
	fmMust(t, err)
	if !positive {
		t.Fatal("missing replacement atom")
	}
	nx := f.own(f.r.manager.MkNot(x))
	ny := f.own(f.r.manager.MkNot(y))
	fmMust(t, context.addConstraint(true, x, ast.ASTHandle{}, y))
	fmMust(t, context.addConstraint(false, nx))
	fmMust(t, context.addConstraint(false, ny))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("asserted disjunction was not enforced", status)
	}
	if err = context.addConstraint(false, x, x, x, x); err == nil {
		t.Fatal("accepted too many literals")
	}
}
