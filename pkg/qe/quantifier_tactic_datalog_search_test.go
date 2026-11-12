package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestQuantifierTacticEliminatesDatalogFiniteEquality(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	name, err := m.Symbols.Intern("qe-datalog-domain")
	fmMust(t, err)
	sort := f.own(m.MkDatalogFiniteSort(name, 4))
	x, y := f.constant("qe-datalog-x", sort), f.constant("qe-datalog-y", sort)
	body := f.own(m.MkEq(x, y))
	quantifierRef, err := bindExprQuantifierVariables(m, []ast.ASTHandle{x}, body)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&quantifierRef)) }()
	quantifier, err := m.RefGet(quantifierRef)
	fmMust(t, err)
	port, err := NewQuantifierTactic(m, nil, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, port.Close()) }()
	goal, err := tactic.NewGoal(m, false, false, false)
	fmMust(t, err)
	defer func() { fmMust(t, goal.Close()) }()
	fmMust(t, goal.Assert(quantifier))
	var output tactic.TacticResult
	defer func() { fmMust(t, output.Close()) }()
	fmMust(t, port.Apply(goal, &output))
	if len(output.Goals) != 1 {
		t.Fatalf("quantifier tactic returned %d goals", len(output.Goals))
	}
	size, err := goal.Size()
	fmMust(t, err)
	if size != 0 {
		t.Fatalf("finite-domain existential equality should reduce to true; goal size = %d", size)
	}
}

func TestQuantifierTacticEliminatesDatalogBlocks(t *testing.T) {
	for _, forall := range []bool{false, true} {
		t.Run(map[bool]string{false: "exists", true: "forall"}[forall], func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			name, err := m.Symbols.Intern("qe-block-domain")
			fmMust(t, err)
			sort := f.own(m.MkDatalogFiniteSort(name, 2))
			x, y := f.own(m.MkVar(1, sort)), f.own(m.MkVar(0, sort))
			body := f.own(m.MkNot(f.own(m.MkEq(x, y))))
			q := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{
				Forall: forall, Sorts: []ast.ASTHandle{sort, sort},
				Names: []symbol.Symbol{symbol.Numerical(0), symbol.Numerical(1)}, Body: body,
			}))
			result, complete, err := qeTestEliminate(m, q, f.r.config)
			fmMust(t, err)
			if !complete {
				t.Fatal("finite-domain block was not eliminated")
			}
			defer func() { fmMust(t, m.DelRef(&result)) }()
			h, err := m.RefGet(result)
			fmMust(t, err)
			if !forall {
				h = f.own(m.MkNot(h))
			}
			kernel := qsatKernelOwner(t, f)
			solver, err := kernel.solver()
			fmMust(t, err)
			fmMust(t, solver.AssertFormula(h))
			status, err := solver.CheckSat(nil)
			fmMust(t, err)
			if status != lbool.False {
				t.Fatalf("block truth value is wrong: forall=%t, status=%v", forall, status)
			}
		})
	}
}

func TestQuantifierMixedBooleanAndDatalogBlock(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	name, err := m.Symbols.Intern("qe-mixed-domain")
	fmMust(t, err)
	sort := f.own(m.MkDatalogFiniteSort(name, 2))
	boolean, err := m.MkBoolSort()
	fmMust(t, err)
	b, x := f.own(m.MkVar(1, boolean)), f.own(m.MkVar(0, sort))
	zero := f.own(m.MkDatalogNumeral(0, sort))
	body := f.own(m.MkAnd(b, f.own(m.MkEq(x, zero))))
	q := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{
		Sorts: []ast.ASTHandle{boolean, sort},
		Names: []symbol.Symbol{symbol.Numerical(0), symbol.Numerical(1)}, Body: body,
	}))
	result, complete, err := qeTestEliminate(m, q, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("mixed Boolean and finite-domain quantifier was not eliminated")
	}
	defer func() { fmMust(t, m.DelRef(&result)) }()
	h, err := m.RefGet(result)
	fmMust(t, err)
	if !m.IsTrue(h) {
		t.Fatal("mixed existential witnesses should exist")
	}
}

func TestQuantifierTacticEliminatesDatalogDisequalities(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	name, err := m.Symbols.Intern("qe-two-element-domain")
	fmMust(t, err)
	sort := f.own(m.MkDatalogFiniteSort(name, 2))
	x, y, z := f.constant("qe-dl-x", sort), f.constant("qe-dl-y", sort), f.constant("qe-dl-z", sort)
	xy, xz := f.own(m.MkEq(x, y)), f.own(m.MkEq(x, z))
	neqY, neqZ := f.own(m.MkNot(xy)), f.own(m.MkNot(xz))
	body := f.own(m.MkAnd(neqY, neqZ))
	quantifierRef, err := bindExprQuantifierVariables(m, []ast.ASTHandle{x}, body)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&quantifierRef)) }()
	quantifier, err := m.RefGet(quantifierRef)
	fmMust(t, err)
	result, complete, err := qeTestEliminate(m, quantifier, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("finite-domain disequalities were not eliminated")
	}
	defer func() { fmMust(t, m.DelRef(&result)) }()
	h, err := m.RefGet(result)
	fmMust(t, err)
	want := f.own(m.MkEq(y, z))
	difference := f.own(m.MkNot(f.own(m.MkEq(h, want))))
	kernel := qsatKernelOwner(t, f)
	solver, err := kernel.solver()
	fmMust(t, err)
	fmMust(t, solver.AssertFormula(difference))
	zero, one := f.own(m.MkDatalogNumeral(0, sort)), f.own(m.MkDatalogNumeral(1, sort))
	for _, value := range []ast.ASTHandle{y, z} {
		isZero, isOne := f.own(m.MkEq(value, zero)), f.own(m.MkEq(value, one))
		fmMust(t, solver.AssertFormula(f.own(m.MkOr(isZero, isOne))))
	}
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("finite-domain disequality projection is not equivalent to y = z", status)
	}
}
