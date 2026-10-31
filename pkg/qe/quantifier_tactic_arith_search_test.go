package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestQuantifierArithmeticSearchSymbolicBounds(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	x := f.own(m.MkVar(0, f.integer))
	y := f.constant("arith-search-y", f.integer)
	zero := f.number("0")
	lower := f.own(m.MkArithmeticGE(x, y))
	upper := f.own(m.MkArithmeticLE(x, zero))
	body := f.own(m.MkAnd(lower, upper))
	quantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
	result, complete, err := eliminateQuantifierArithmeticSearch(m, quantifier, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("symbolic bounds were not eliminated")
	}
	defer func() { fmMust(t, m.DelRef(&result)) }()
	value, err := m.RefGet(result)
	fmMust(t, err)
	expected := f.own(m.MkArithmeticLE(y, zero))
	iff := f.own(m.MkIff(value, expected))
	different := f.own(m.MkNot(iff))
	k := qsatKernelOwner(t, f)
	solver, err := k.solver()
	fmMust(t, err)
	fmMust(t, solver.AssertFormula(different))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("symbolic bound result differs from y <= 0")
	}
	greater := f.own(m.MkArithmeticGT(x, y))
	forall := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: greater}))
	falseResult, complete, err := eliminateQuantifierArithmeticSearch(m, forall, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("unbounded universal comparison was not eliminated")
	}
	defer func() { fmMust(t, m.DelRef(&falseResult)) }()
	falseValue, err := m.RefGet(falseResult)
	fmMust(t, err)
	if !m.IsFalse(falseValue) {
		t.Fatal("not every integer exceeds y")
	}
}

func TestQuantifierArithmeticSearchStrictBounds(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	x := f.own(m.MkVar(0, f.integer))
	y := f.constant("arith-strict-int-y", f.integer)
	oneAbove := f.own(m.MkArithmeticAdd(y, f.number("1")))
	lower := f.own(m.MkArithmeticGT(x, y))
	upper := f.own(m.MkArithmeticLT(x, oneAbove))
	body := f.own(m.MkAnd(lower, upper))
	quantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
	result, complete, err := eliminateQuantifierArithmeticSearch(m, quantifier, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("empty integer interval was not eliminated")
	}
	defer func() { fmMust(t, m.DelRef(&result)) }()
	value, err := m.RefGet(result)
	fmMust(t, err)
	if !m.IsFalse(value) {
		t.Fatal("there is no integer strictly between y and y+1")
	}
	r := f.own(m.MkVar(0, f.real))
	bound := f.constant("arith-strict-real-bound", f.real)
	zero := f.own(m.ArithmeticRealNumeralText("0"))
	rLower := f.own(m.MkArithmeticGT(r, zero))
	rUpper := f.own(m.MkArithmeticLT(r, bound))
	rBody := f.own(m.MkAnd(rLower, rUpper))
	realQuantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.real}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: rBody}))
	realResult, complete, err := eliminateQuantifierArithmeticSearch(m, realQuantifier, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("real open interval was not eliminated")
	}
	defer func() { fmMust(t, m.DelRef(&realResult)) }()
	realValue, err := m.RefGet(realResult)
	fmMust(t, err)
	expected := f.own(m.MkArithmeticGT(bound, zero))
	iff := f.own(m.MkIff(realValue, expected))
	different := f.own(m.MkNot(iff))
	k := qsatKernelOwner(t, f)
	solver, err := k.solver()
	fmMust(t, err)
	fmMust(t, solver.AssertFormula(different))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("real open interval differs from a positive bound")
	}
}

func TestQuantifierArithmeticSearchTwoSymbolicBounds(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		real, strict, forall bool
	}{{"integer-closed", false, false, false}, {"integer-open", false, true, false}, {"real-closed", true, false, false}, {"real-open", true, true, false}, {"integer-universal", false, false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			f := qsatKernelFixture(t)
			m := f.r.manager
			sort := f.integer
			if tc.real {
				sort = f.real
			}
			x := f.own(m.MkVar(0, sort))
			lower := f.constant("two-bound-lower", sort)
			upper := f.constant("two-bound-upper", sort)
			var body, expected ast.ASTHandle
			if tc.forall {
				below := f.own(m.MkArithmeticLT(x, lower))
				above := f.own(m.MkArithmeticGT(x, upper))
				body = f.own(m.MkOr(below, above))
				expected = f.own(m.MkArithmeticGT(lower, upper))
			} else if tc.strict {
				above := f.own(m.MkArithmeticGT(x, lower))
				below := f.own(m.MkArithmeticLT(x, upper))
				body = f.own(m.MkAnd(above, below))
				if tc.real {
					expected = f.own(m.MkArithmeticLT(lower, upper))
				} else {
					next := f.own(m.MkArithmeticAdd(lower, f.number("1")))
					expected = f.own(m.MkArithmeticLT(next, upper))
				}
			} else {
				above := f.own(m.MkArithmeticGE(x, lower))
				below := f.own(m.MkArithmeticLE(x, upper))
				body = f.own(m.MkAnd(above, below))
				expected = f.own(m.MkArithmeticLE(lower, upper))
			}
			quantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: tc.forall, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
			result, complete, err := eliminateQuantifierArithmeticSearch(m, quantifier, f.r.config)
			fmMust(t, err)
			if !complete {
				t.Fatal("two-symbolic-bound case was not eliminated")
			}
			defer func() { fmMust(t, m.DelRef(&result)) }()
			value, err := m.RefGet(result)
			fmMust(t, err)
			iff := f.own(m.MkIff(value, expected))
			different := f.own(m.MkNot(iff))
			k := qsatKernelOwner(t, f)
			solver, err := k.solver()
			fmMust(t, err)
			fmMust(t, solver.AssertFormula(different))
			status, err := solver.CheckSat(nil)
			fmMust(t, err)
			if status != lbool.False {
				t.Fatal("quantifier result differs from the symbolic bound condition")
			}
		})
	}
}

func TestQuantifierArithmeticSearchUnsupportedDivisibility(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	x := f.own(m.MkVar(0, f.integer))
	y := f.constant("arith-search-divisibility-y", f.integer)
	twoX := f.own(m.MkArithmeticMul(f.number("2"), x))
	body := f.own(m.MkEq(twoX, y))
	quantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
	result, complete, err := eliminateQuantifierArithmeticSearch(m, quantifier, f.r.config)
	if result != (ast.ASTRef{}) {
		fmMust(t, m.DelRef(&result))
	}
	if complete || !errors.Is(err, errArithmeticBranchUnsupported) {
		t.Fatalf("unsupported divisibility returned complete=%v, err=%v", complete, err)
	}
}

func TestQuantifierArithmeticSearchOneUnitCoefficient(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	x := f.own(m.MkVar(0, f.integer))
	y := f.constant("arith-unit-upper", f.integer)
	z := f.constant("arith-unit-lower", f.integer)
	two := f.number("2")
	twoX := f.own(m.MkArithmeticMul(two, x))
	upper := f.own(m.MkArithmeticLE(twoX, y))
	lower := f.own(m.MkArithmeticGE(x, z))
	body := f.own(m.MkAnd(upper, lower))
	quantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
	result, complete, err := eliminateQuantifierArithmeticSearch(m, quantifier, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("integer bounds with one unit coefficient were not eliminated")
	}
	defer func() { fmMust(t, m.DelRef(&result)) }()
	value, err := m.RefGet(result)
	fmMust(t, err)
	twoZ := f.own(m.MkArithmeticMul(two, z))
	expected := f.own(m.MkArithmeticLE(twoZ, y))
	iff := f.own(m.MkIff(value, expected))
	different := f.own(m.MkNot(iff))
	k := qsatKernelOwner(t, f)
	solver, err := k.solver()
	fmMust(t, err)
	fmMust(t, solver.AssertFormula(different))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("integer QE result differs from 2*z <= y")
	}
}
