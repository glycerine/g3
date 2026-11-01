package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestQuantifierArrayEquationsProjection(t *testing.T) {
	for _, name := range []string{"array-equality", "single-select", "two-selects", "disjunction", "guarded-disjunction"} {
		t.Run(name, func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			family, err := m.RegisterArrayPlugin()
			fmMust(t, err)
			sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
			a := f.constant("qe-array-a", sort)
			b := f.constant("qe-array-b", sort)
			c := f.constant("qe-array-c", sort)
			i, j := f.constant("qe-array-i", f.integer), f.constant("qe-array-j", f.integer)
			v, w := f.constant("qe-array-v", f.integer), f.constant("qe-array-w", f.integer)
			selectAt := func(array, index ast.ASTHandle) ast.ASTHandle {
				return f.own(m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{array, index}))
			}
			first := f.own(m.MkEq(selectAt(a, i), v))
			var body, expected ast.ASTHandle
			if name == "array-equality" {
				body = f.own(m.MkAnd(f.own(m.MkEq(a, b)), first))
				expected = f.own(m.MkEq(selectAt(b, i), v))
			} else if name == "single-select" {
				body = first
				expected = f.own(m.MkTrue())
			} else if name == "two-selects" {
				body = f.own(m.MkAnd(first, f.own(m.MkEq(selectAt(a, j), w))))
				expected = f.own(m.MkOr(f.own(m.MkNot(f.own(m.MkEq(i, j)))), f.own(m.MkEq(v, w))))
			} else {
				left := f.own(m.MkAnd(f.own(m.MkEq(a, b)), first))
				right := f.own(m.MkAnd(f.own(m.MkEq(a, c)), f.own(m.MkEq(selectAt(a, i), w))))
				body = f.own(m.MkOr(left, right))
				expected = f.own(m.MkOr(f.own(m.MkEq(selectAt(b, i), v)), f.own(m.MkEq(selectAt(c, i), w))))
				if name == "guarded-disjunction" {
					guard := f.constant("qe-array-guard", f.boolean)
					body = f.own(m.MkAnd(guard, body))
					expected = f.own(m.MkAnd(guard, expected))
				}
			}
			bound, err := bindExprQuantifierVariables(m, []ast.ASTHandle{a}, body)
			fmMust(t, err)
			defer func() { fmMust(t, m.DelRef(&bound)) }()
			quantifier, err := m.RefGet(bound)
			fmMust(t, err)
			result, complete, err := eliminateQuantifierArrayEquations(m, quantifier, f.r.config)
			fmMust(t, err)
			if !complete {
				t.Fatal("array equations were not eliminated")
			}
			defer func() { fmMust(t, m.DelRef(&result)) }()
			value, err := m.RefGet(result)
			fmMust(t, err)
			difference := f.own(m.MkNot(f.own(m.MkEq(value, expected))))
			kernel := qsatKernelOwner(t, f)
			solver, err := kernel.solver()
			fmMust(t, err)
			fmMust(t, solver.AssertFormula(difference))
			status, err := solver.CheckSat(nil)
			fmMust(t, err)
			if status != lbool.False {
				t.Fatal("projected array formula differs from expected", status)
			}
		})
	}
}

func TestQuantifierArrayDisjunctionUnderNegationIsNotDistributed(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	_, err := m.RegisterArrayPlugin()
	fmMust(t, err)
	sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a, b, c := f.constant("neg-array-a", sort), f.constant("neg-array-b", sort), f.constant("neg-array-c", sort)
	body := f.own(m.MkNot(f.own(m.MkOr(f.own(m.MkEq(a, b)), f.own(m.MkEq(a, c))))))
	bound, err := bindExprQuantifierVariables(m, []ast.ASTHandle{a}, body)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&bound)) }()
	quantifier, err := m.RefGet(bound)
	fmMust(t, err)
	result, complete, err := eliminateQuantifierArrayEquations(m, quantifier, f.r.config)
	fmMust(t, err)
	if complete || result != (ast.ASTRef{}) {
		t.Fatal("negative disjunction was incorrectly projected")
	}
}
