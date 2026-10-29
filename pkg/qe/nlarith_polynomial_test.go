package qe_test

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/qe"
	"testing"
)

func TestNLArithmeticPolynomialDecompositionOperationsAndDerivative(t *testing.T) {
	f := newArithmeticQEFixture(t)
	u, err := qe.NewNonlinearArithmeticUtil(f.manager)
	pluginMust(t, err)
	defer func() { pluginMust(t, u.Close()) }()
	contains, err := ast.NewContainsApp(f.manager, f.rx)
	pluginMust(t, err)
	defer func() { pluginMust(t, contains.Close()) }()
	left := f.pin(f.manager.MkArithmeticAdd(f.rx, f.ry))
	right := f.pin(f.manager.MkArithmeticSub(f.rx, f.ry))
	product := f.pin(f.manager.MkArithmeticMul(left, right))
	p, ok, err := u.Decompose(contains, product)
	pluginMust(t, err)
	if !ok {
		t.Fatal("quadratic rejected")
	}
	defer func() { pluginMust(t, p.Close()) }()
	degree, err := p.Degree()
	pluginMust(t, err)
	if degree != 2 {
		t.Fatal("degree", degree)
	}
	for i := uint32(0); i <= degree; i++ {
		coefficient, err := p.Coefficient(i)
		pluginMust(t, err)
		if found, err := contains.Check(coefficient); err != nil || found {
			t.Fatal("coefficient retained x", i, found, err)
		}
	}
	d, err := u.Derivative(p)
	pluginMust(t, err)
	defer func() { pluginMust(t, d.Close()) }()
	if degree, err = d.Degree(); err != nil || degree != 1 {
		t.Fatal("derivative degree", degree, err)
	}
	clone, err := p.Clone()
	pluginMust(t, err)
	defer func() { pluginMust(t, clone.Close()) }()
	pluginMust(t, u.Negate(clone))
	pluginMust(t, u.Add(clone, p))
	if clone.Len() != 3 {
		t.Fatal("operation changed degree", clone.Len())
	}
}

func TestNLArithmeticNumericGrammarAndPolynomialDivision(t *testing.T) {
	f := newArithmeticQEFixture(t)
	u, err := qe.NewNonlinearArithmeticUtil(f.manager)
	pluginMust(t, err)
	defer func() { pluginMust(t, u.Close()) }()
	contains, err := ast.NewContainsApp(f.manager, f.rx)
	pluginMust(t, err)
	defer func() { pluginMust(t, contains.Close()) }()
	two := f.realNumeral(t, "2")
	three := f.realNumeral(t, "3")
	sum := f.pin(f.manager.MkArithmeticAdd(two, three))
	neg := f.pin(f.manager.MkArithmeticNeg(sum))
	value, ok, err := u.Numeral(neg)
	pluginMust(t, err)
	if !ok || value.RatString() != "-5" {
		t.Fatal("numeral grammar", ok, value)
	}
	x2 := f.pin(f.manager.MkArithmeticMul(f.rx, f.rx))
	dividend, ok, err := u.Decompose(contains, x2)
	pluginMust(t, err)
	if !ok {
		t.Fatal("x2 decomposition")
	}
	defer func() { pluginMust(t, dividend.Close()) }()
	divisor, ok, err := u.Decompose(contains, f.rx)
	pluginMust(t, err)
	if !ok {
		t.Fatal("x decomposition")
	}
	defer func() { pluginMust(t, divisor.Close()) }()
	q, r, handled, err := u.NumericQuotientRemainder(dividend, divisor)
	pluginMust(t, err)
	if !handled {
		t.Fatal("numeric divisor declined")
	}
	defer func() { pluginMust(t, q.Close()); pluginMust(t, r.Close()) }()
	if degree, err := q.Degree(); err != nil || degree != 1 {
		t.Fatal("quotient", degree, err)
	}
	if r.Len() != 1 {
		t.Fatal("remainder shape", r.Len())
	}
}
