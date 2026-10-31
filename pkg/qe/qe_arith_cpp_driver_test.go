package qe

// Source: src/test/qe_arith.cpp test_project integer cases.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestArithmeticProjectionSourceIntegerExamples(t *testing.T) {
	for _, name := range []string{"modulo", "inequalities", "nonunit-equalities"} {
		t.Run(name, func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			model := predicateModel(t, f)
			p := arithmeticProjectionOwner(t, f)
			x, y, z, u := f.constant("x", f.integer), f.constant("y", f.integer), f.constant("z", f.integer), f.constant("u", f.integer)
			num := func(n string) ast.ASTHandle { return f.own(m.ArithmeticIntegerNumeralText(n)) }
			values := []string{"0", "1", "0", "6"}
			if name == "modulo" {
				values = []string{"0", "4", "8", "6"}
			}
			if name == "nonunit-equalities" {
				values = []string{"1", "4", "2", "3"}
			}
			for i, variable := range []ast.ASTHandle{x, y, z, u} {
				predicateRegister(t, f, model, variable, num(values[i]))
			}
			var variables, forms []ast.ASTHandle
			switch name {
			case "modulo":
				variables = []ast.ASTHandle{y}
				mod := f.own(m.MkFamilyApp(f.r.arithmetic, 13, nil, []ast.ASTHandle{y, num("3")}))
				forms = []ast.ASTHandle{f.own(m.MkEq(mod, num("1"))), f.own(m.MkEq(f.own(m.MkArithmeticMul(num("2"), y)), z))}
			case "inequalities":
				variables = []ast.ASTHandle{x, y}
				forms = []ast.ASTHandle{
					f.own(m.MkArithmeticLE(z, f.own(m.MkArithmeticAdd(x, f.own(m.MkArithmeticMul(num("2"), y)))))),
					f.own(m.MkArithmeticLT(f.own(m.MkArithmeticMul(num("2"), x)), f.own(m.MkArithmeticAdd(u, num("3"))))),
					f.own(m.MkArithmeticLE(f.own(m.MkArithmeticMul(num("2"), y)), u)),
				}
			case "nonunit-equalities":
				variables = []ast.ASTHandle{x}
				forms = []ast.ASTHandle{f.own(m.MkEq(f.own(m.MkArithmeticMul(num("2"), x)), z)), f.own(m.MkEq(f.own(m.MkArithmeticMul(num("3"), x)), u))}
			}
			refs := predicateRefs(t, f, forms...)
			vars := predicateRefs(t, f, variables...)
			fmMust(t, p.Project(model.Core(), vars, refs))
			arithmeticEvaluate(t, f, model, *refs)
			if len(*vars) != 0 {
				t.Fatalf("source example left %d variables unprojected", len(*vars))
			}
		})
	}
}
