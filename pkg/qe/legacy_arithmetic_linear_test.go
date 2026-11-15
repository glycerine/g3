package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// arith_project_util::is_linear.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestLegacyArithmeticProjectionClassifiesLinearLiterals(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArithmeticProjection(m, f.r.config)
	fmMust(t, err)
	x := f.constant("legacy-linear-x", f.real)
	y := f.constant("legacy-linear-y", f.real)
	two := f.own(m.ArithmeticRealNumeralText("2"))
	three := f.own(m.ArithmeticRealNumeralText("3"))
	twoX := f.own(m.MkArithmeticMul(two, x))
	left := f.own(m.MkArithmeticAdd(twoX, y))
	closed := f.own(m.MkArithmeticLE(left, three))
	literal := f.own(m.MkNot(closed))
	classified, err := p.linearLiteral(x, literal)
	fmMust(t, err)
	if classified == nil || !classified.strict || classified.equality || classified.disequality || classified.coefficient.Cmp(big.NewRat(-2, 1)) != 0 {
		t.Fatal("legacy linear classification", classified)
	}
	md, err := model.NewModel(m)
	fmMust(t, err)
	yDecl, err := m.GetDecl(y)
	fmMust(t, err)
	five := f.own(m.ArithmeticRealNumeralText("5"))
	fmMust(t, md.Core().RegisterConstant(yDecl, five))
	evaluator, err := model.NewModelEvaluator(md.Core(), model.ModelEvaluatorConfig{Context: f.r.config.Context, Globals: f.r.config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: f.r.config.Cooperation})
	fmMust(t, err)
	term, err := m.RefGet(classified.term)
	fmMust(t, err)
	value, err := evaluator.EvaluateValue(term)
	fmMust(t, err)
	valueHandle, err := m.RefGet(value)
	fmMust(t, err)
	text, _, numeral, err := m.ArithmeticNumeralText(valueHandle)
	fmMust(t, err)
	if !numeral || text != "-2" {
		t.Fatal("legacy residual term", text, numeral)
	}
	fmMust(t, m.DelRef(&value))
	fmMust(t, evaluator.Close())
	fmMust(t, md.Close())
	fmMust(t, classified.Close())

	nonlinear := f.own(m.MkArithmeticMul(x, y))
	zero := f.own(m.ArithmeticRealNumeralText("0"))
	nonlinearBound := f.own(m.MkArithmeticLE(nonlinear, zero))
	classified, err = p.linearLiteral(x, nonlinearBound)
	fmMust(t, err)
	if classified != nil {
		t.Fatal("legacy classifier accepted nonlinear occurrence")
	}
	fmMust(t, p.Close())
}

func TestLegacyArithmeticProjectionClassifiesEqualities(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArithmeticProjection(m, f.r.config)
	fmMust(t, err)
	x := f.constant("legacy-equality-x", f.integer)
	one := f.own(m.ArithmeticIntegerNumeralText("1"))
	equality := f.own(m.MkEq(x, one))
	classified, err := p.linearLiteral(x, equality)
	fmMust(t, err)
	if classified == nil || !classified.equality || classified.disequality || classified.strict || classified.coefficient.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("legacy equality classification", classified)
	}
	fmMust(t, classified.Close())
	notEquality := f.own(m.MkNot(equality))
	classified, err = p.linearLiteral(x, notEquality)
	fmMust(t, err)
	if classified == nil || classified.equality || !classified.disequality {
		t.Fatal("legacy disequality classification", classified)
	}
	fmMust(t, closeLegacyLinearLiterals(classified))
	two := f.own(m.ArithmeticIntegerNumeralText("2"))
	three := f.own(m.ArithmeticIntegerNumeralText("3"))
	twoX := f.own(m.MkArithmeticMul(two, x))
	sum := f.own(m.MkArithmeticAdd(twoX, one))
	modulus := f.own(m.MkFamilyApp(f.r.arithmetic, 13, nil, []ast.ASTHandle{sum, three}))
	zero := f.own(m.ArithmeticIntegerNumeralText("0"))
	divides := f.own(m.MkEq(modulus, zero))
	classified, err = p.linearLiteral(x, divides)
	fmMust(t, err)
	if classified == nil || classified.equality || classified.disequality || classified.divisor.Cmp(big.NewRat(3, 1)) != 0 || classified.coefficient.Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal("legacy divisibility classification", classified)
	}
	fmMust(t, classified.Close())
	fmMust(t, p.Close())
}

// AUDIT.md qe_02-F7: arith_project_util::is_linear visits only t of a
// divisibility literal (= (mod t k) 0), builds one n-ary mk_add over ts, and
// mk_mul always builds (* r t), even for r = 1.
func TestLegacyArithmeticLinearTermShapes(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArithmeticProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	x := f.constant("legacy-shape-x", f.integer)
	y := f.constant("legacy-shape-y", f.integer)
	a := f.constant("legacy-shape-a", f.integer)
	b := f.constant("legacy-shape-b", f.integer)
	c := f.constant("legacy-shape-c", f.integer)
	zero := f.own(m.ArithmeticIntegerNumeralText("0"))
	three := f.own(m.ArithmeticIntegerNumeralText("3"))
	mod := f.own(m.MkFamilyApp(p.syntax.family, 13, nil, []ast.ASTHandle{f.own(m.MkArithmeticAdd(x, y)), three}))
	divisible, err := p.linearLiteral(x, f.own(m.MkEq(mod, zero)))
	fmMust(t, err)
	if divisible == nil || divisible.divisor.Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatal("divisibility literal", divisible)
	}
	term, err := m.RefGet(divisible.term)
	fmMust(t, err)
	if term != y {
		t.Fatal("divisibility term is not y")
	}
	fmMust(t, divisible.Close())

	sum := f.own(m.MkFamilyApp(p.syntax.family, 6, nil, []ast.ASTHandle{x, a, b, c}))
	bound, err := p.linearLiteral(x, f.own(m.MkArithmeticLE(sum, zero)))
	fmMust(t, err)
	term, err = m.RefGet(bound.term)
	fmMust(t, err)
	// is_linear(-mul, 0) also pushes the numeral 0 of the right side.
	if term != f.own(m.MkFamilyApp(p.syntax.family, 6, nil, []ast.ASTHandle{a, b, c, zero})) {
		t.Fatal("bound term is not the n-ary (+ a b c 0)")
	}
	fmMust(t, bound.Close())

	unit, err := p.multiplyLegacy(big.NewRat(1, 1), y)
	fmMust(t, err)
	one := f.own(m.ArithmeticIntegerNumeralText("1"))
	if unit != f.own(m.MkArithmeticMul(one, y)) {
		t.Fatal("mk_mul(1, y) is not (* 1 y)")
	}
}
