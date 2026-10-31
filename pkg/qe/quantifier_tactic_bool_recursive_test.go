package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestRewriteQuantifierBoolPreservesOuterBindings(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	innerVar := f.own(m.MkVar(0, f.boolean))
	outerVar := f.own(m.MkVar(1, f.boolean))
	eq := f.own(m.MkIff(innerVar, outerVar))
	for _, tc := range []struct {
		name                           string
		outerForall, innerForall, want bool
	}{{"forall-exists", true, false, true}, {"exists-forall", false, true, false}} {
		t.Run(tc.name, func(t *testing.T) {
			inner := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: tc.innerForall, Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(1)}, Body: eq}))
			outer := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: tc.outerForall, Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: inner}))
			result, complete, err := rewriteQuantifierScalar(m, outer, f.r.config)
			fmMust(t, err)
			if !complete {
				t.Fatal("alternating Boolean quantifiers were not eliminated")
			}
			defer func() { fmMust(t, m.DelRef(&result)) }()
			h, err := m.RefGet(result)
			fmMust(t, err)
			if (tc.want && !m.IsTrue(h)) || (!tc.want && !m.IsFalse(h)) {
				t.Fatal("outer variable binding changed during elimination")
			}
		})
	}
}

func TestRewriteQuantifierBoolInsideApplication(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	v := f.own(m.MkVar(0, f.boolean))
	q := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: v}))
	formula := f.predicate("qe-recursive-predicate", q)
	result, complete, err := rewriteQuantifierScalar(m, formula, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("quantifier inside an application was not eliminated")
	}
	defer func() { fmMust(t, m.DelRef(&result)) }()
	value, err := m.RefGet(result)
	fmMust(t, err)
	truth := f.own(m.MkTrue())
	expected := f.predicate("qe-recursive-predicate", truth)
	iff := f.own(m.MkIff(value, expected))
	different := f.own(m.MkNot(iff))
	k := qsatKernelOwner(t, f)
	solver, err := k.solver()
	fmMust(t, err)
	fmMust(t, solver.AssertFormula(different))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("nested application differs after quantifier elimination")
	}
}

func TestRewriteQuantifierBoolPreservesMultipleOuterVariableOrder(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	c := f.own(m.MkVar(0, f.boolean))
	b := f.own(m.MkVar(1, f.boolean))
	a := f.own(m.MkVar(2, f.boolean))
	equality := f.own(m.MkIff(c, a))
	body := f.own(m.MkAnd(equality, b))
	inner := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(2)}, Body: body}))
	middle := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(1)}, Body: inner}))
	outer := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: middle}))
	result, complete, err := rewriteQuantifierScalar(m, outer, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("three nested Boolean binders were not eliminated")
	}
	defer func() { fmMust(t, m.DelRef(&result)) }()
	value, err := m.RefGet(result)
	fmMust(t, err)
	if !m.IsFalse(value) {
		t.Fatal("outer variable order changed: exists a forall b exists c ((c = a) and b) is false")
	}
}
