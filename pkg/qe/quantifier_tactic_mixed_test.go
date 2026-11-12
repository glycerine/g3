package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestQuantifierMixedBooleanArithmeticGroups(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	b := f.own(m.MkVar(1, f.boolean))
	x := f.own(m.MkVar(0, f.integer))
	y := f.constant("mixed-qe-y", f.integer)
	zero := f.number("0")
	equality := f.own(m.MkEq(x, y))
	positive := f.own(m.MkArithmeticGT(x, zero))
	body := f.own(m.MkAndArray([]ast.ASTHandle{b, equality, positive}))
	quantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.boolean, f.integer}, Names: []symbol.Symbol{symbol.Numerical(0), symbol.Numerical(1)}, Body: body}))
	result, complete, err := qeTestEliminate(m, quantifier, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("mixed Boolean and integer block was not eliminated")
	}
	defer func() { fmMust(t, m.DelRef(&result)) }()
	value, err := m.RefGet(result)
	fmMust(t, err)
	expected := f.own(m.MkArithmeticGT(y, zero))
	iff := f.own(m.MkIff(value, expected))
	different := f.own(m.MkNot(iff))
	k := qsatKernelOwner(t, f)
	solver, err := k.solver()
	fmMust(t, err)
	fmMust(t, solver.AssertFormula(different))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("mixed result differs from the free arithmetic condition")
	}
	independentBody := f.own(m.MkAnd(b, positive))
	independent := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.boolean, f.integer}, Names: []symbol.Symbol{symbol.Numerical(0), symbol.Numerical(1)}, Body: independentBody}))
	independentResult, complete, err := qeTestEliminate(m, independent, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("independent mixed inequality was not eliminated")
	}
	defer func() { fmMust(t, m.DelRef(&independentResult)) }()
	independentValue, err := m.RefGet(independentResult)
	fmMust(t, err)
	if !m.IsTrue(independentValue) {
		t.Fatal("mixed Boolean and positive integer witnesses should exist")
	}
	notEq := f.own(m.MkNot(equality))
	implies := f.own(m.MkImplies(b, notEq))
	forall := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.boolean, f.integer}, Names: []symbol.Symbol{symbol.Numerical(0), symbol.Numerical(1)}, Body: implies}))
	falseResult, complete, err := qeTestEliminate(m, forall, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("universal mixed block was not eliminated")
	}
	defer func() { fmMust(t, m.DelRef(&falseResult)) }()
	falseValue, err := m.RefGet(falseResult)
	fmMust(t, err)
	if !m.IsFalse(falseValue) {
		t.Fatal("universal mixed formula should be false")
	}
}

func TestQuantifierMixedArrayAndBitVectorGroups(t *testing.T) {
	for _, name := range []string{"array-bool", "array-arithmetic", "bitvector-bool", "bitvector-block-bool"} {
		t.Run(name, func(t *testing.T) {
			f := qsatKernelFixture(t)
			m := f.r.manager
			boolean := f.constant("mixed-group-bool", f.boolean)
			var variables []ast.ASTHandle
			var body, expected ast.ASTHandle
			if name == "bitvector-bool" || name == "bitvector-block-bool" {
				sort := f.own(m.BitVectorSort(2))
				x := f.constant("mixed-group-bv", sort)
				y := f.constant("mixed-group-bv-free", sort)
				one := f.own(m.BitVectorNumeralUint64(1, 2))
				body = f.own(m.MkAnd(boolean, f.own(m.MkEq(f.own(m.MkBitVectorAdd(x, y)), one))))
				expected = f.own(m.MkTrue())
				variables = []ast.ASTHandle{boolean, x}
				if name == "bitvector-block-bool" {
					variables = append(variables, y)
				}
			} else {
				family, err := m.RegisterArrayPlugin()
				fmMust(t, err)
				sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
				a, b := f.constant("mixed-array-a", sort), f.constant("mixed-array-b", sort)
				i := f.constant("mixed-array-i", f.integer)
				v, w := f.constant("mixed-array-v", f.integer), f.constant("mixed-array-w", f.integer)
				selectAt := func(array ast.ASTHandle) ast.ASTHandle {
					return f.own(m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{array, i}))
				}
				eqArray := f.own(m.MkEq(a, b))
				if name == "array-bool" {
					choice := f.own(m.MkIte(boolean, v, w))
					body = f.own(m.MkAnd(eqArray, f.own(m.MkEq(selectAt(a), choice))))
					expected = f.own(m.MkOr(f.own(m.MkEq(selectAt(b), v)), f.own(m.MkEq(selectAt(b), w))))
					variables = []ast.ASTHandle{a, boolean}
				} else {
					x := f.constant("mixed-array-x", f.integer)
					body = f.own(m.MkAndArray([]ast.ASTHandle{eqArray, f.own(m.MkEq(selectAt(a), x)), f.own(m.MkEq(x, v))}))
					expected = f.own(m.MkEq(selectAt(b), v))
					variables = []ast.ASTHandle{a, x}
				}
			}
			bound, err := bindExprQuantifierVariables(m, variables, body)
			fmMust(t, err)
			defer func() { fmMust(t, m.DelRef(&bound)) }()
			quantifier, err := m.RefGet(bound)
			fmMust(t, err)
			result, complete, err := qeTestEliminate(m, quantifier, f.r.config)
			fmMust(t, err)
			if !complete {
				t.Fatal("mixed theory block was not eliminated")
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
				t.Fatal("mixed projection differs from expected", status)
			}
		})
	}
}
