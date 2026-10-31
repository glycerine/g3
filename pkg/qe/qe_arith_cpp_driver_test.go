package qe

// Source: src/test/qe_arith.cpp test_project integer cases.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
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

func TestArithmeticProjectionSourceFunctionNonProjection(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	md := predicateModel(t, f)
	p := arithmeticProjectionOwner(t, f)
	x, y, z, u := f.constant("x", f.integer), f.constant("y", f.integer), f.constant("z", f.integer), f.constant("u", f.integer)
	num := func(n string) ast.ASTHandle { return f.own(m.ArithmeticIntegerNumeralText(n)) }
	for i, variable := range []ast.ASTHandle{x, y, z, u} {
		predicateRegister(t, f, md, variable, num([]string{"0", "1", "2", "3"}[i]))
	}
	name, err := m.Symbols.Intern("f")
	fmMust(t, err)
	decl := f.own(m.MkFuncDecl(name, []ast.ASTHandle{f.integer}, f.integer, ast.ASTFuncDeclInfo{}))
	interp, err := model.NewFunctionInterpretation(m, 1)
	fmMust(t, err)
	for _, pair := range [][2]string{{"0", "1"}, {"1", "2"}, {"2", "3"}} {
		fmMust(t, interp.InsertNewEntry([]ast.ASTHandle{num(pair[0])}, num(pair[1])))
	}
	fmMust(t, interp.SetElse(num("10")))
	fmMust(t, md.Core().RegisterFunction(decl, interp))
	fx := f.own(m.MkApp(decl, []ast.ASTHandle{x}))
	forms := predicateRefs(t, f, f.own(m.MkArithmeticLE(x, fx)), f.own(m.MkArithmeticLT(x, y)))
	vars := predicateRefs(t, f, x)
	fmMust(t, p.Project(md.Core(), vars, forms))
	arithmeticEvaluate(t, f, md, *forms)
	if len(*vars) != 1 || predicateValue(t, f, (*vars)[0]) != x {
		t.Fatal("opaque function application should prevent projection")
	}
}

func TestArithmeticProjectionSourceDistinctAndDisequality(t *testing.T) {
	for _, name := range []string{"disequality", "negative-distinct-bound", "negative-distinct-free", "distinct"} {
		t.Run(name, func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			model := predicateModel(t, f)
			p := arithmeticProjectionOwner(t, f)
			x, y, z, u := f.constant("x", f.integer), f.constant("y", f.integer), f.constant("z", f.integer), f.constant("u", f.integer)
			num := func(n string) ast.ASTHandle { return f.own(m.ArithmeticIntegerNumeralText(n)) }
			for i, variable := range []ast.ASTHandle{x, y, z, u} {
				predicateRegister(t, f, model, variable, num([]string{"0", "1", "0", "6"}[i]))
			}
			var form ast.ASTHandle
			variables := []ast.ASTHandle{x, y}
			if name == "disequality" {
				variables = []ast.ASTHandle{x}
				form = f.own(m.MkNot(f.own(m.MkEq(x, y))))
			} else {
				terms := []ast.ASTHandle{x, y, f.own(m.MkArithmeticAdd(z, num("2"))), u}
				if name == "negative-distinct-bound" {
					terms = append(terms, z)
				}
				if name == "negative-distinct-free" {
					terms = append(terms, f.own(m.MkArithmeticAdd(z, num("10"))), f.own(m.MkArithmeticAdd(u, num("4"))))
				}
				form = f.own(m.MkDistinct(terms))
				if name != "distinct" {
					form = f.own(m.MkNot(form))
				}
			}
			refs := predicateRefs(t, f, form)
			vars := predicateRefs(t, f, variables...)
			fmMust(t, p.Project(model.Core(), vars, refs))
			arithmeticEvaluate(t, f, model, *refs)
			if len(*vars) != 0 {
				t.Fatalf("source example left %d variables unprojected", len(*vars))
			}
		})
	}
}
