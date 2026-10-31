package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSATPartialArithmeticOutputsAddVariablesAndConstraints(t *testing.T) {
	f := newArithmeticModelFixture(t, false, "5")
	m := f.r.manager
	k := qsatKernelOwner(t, f.fmFixture)
	solver, err := k.solver()
	fmMust(t, err)
	c, err := newSATSolverContext(m, solver, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, c.Close()) }()
	y := f.constant("sat-cooper-y", f.integer)
	twoX := f.own(m.MkArithmeticMul(f.number("2"), f.x))
	polynomial := f.own(m.MkArithmeticAdd(twoX, y))
	mod := f.own(m.MkFamilyApp(f.r.arithmetic, 13, nil, []ast.ASTHandle{polynomial, f.number("7")}))
	formula := f.own(m.MkEq(mod, f.number("0")))
	ok, err := f.plugin.Prepare(f.contains, formula, []ast.ASTHandle{formula}, nil)
	fmMust(t, err)
	if !ok {
		t.Fatal("modular bound preparation")
	}
	branch, err := m.NumeralService().NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, m.NumeralService().Del(&branch)) }()
	fmMust(t, m.NumeralService().SetUnsigned(&branch, 0))
	fmMust(t, f.plugin.Assign(f.contains, formula, &branch))
	fmMust(t, c.takePartialArithmeticOutputs(f.plugin))
	if len(c.variables) != 1 {
		t.Fatalf("got %d new variables", len(c.variables))
	}
	fresh, err := m.RefGet(c.variables[0])
	fmMust(t, err)
	if fresh == f.x || fresh == y {
		t.Fatal("expected fresh variable")
	}
	count, err := solver.NumAssertions()
	fmMust(t, err)
	if count < 4 {
		t.Fatalf("got %d generated constraints", count)
	}
}
