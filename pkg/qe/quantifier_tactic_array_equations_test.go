package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestQuantifierArrayEquationsProjection(t *testing.T) {
	for _, name := range []string{"array-equality", "single-select", "two-selects"} {
		t.Run(name, func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			family, err := m.RegisterArrayPlugin()
			fmMust(t, err)
			sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
			a := f.constant("qe-array-a", sort)
			b := f.constant("qe-array-b", sort)
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
			} else {
				body = f.own(m.MkAnd(first, f.own(m.MkEq(selectAt(a, j), w))))
				expected = f.own(m.MkOr(f.own(m.MkNot(f.own(m.MkEq(i, j)))), f.own(m.MkEq(v, w))))
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
