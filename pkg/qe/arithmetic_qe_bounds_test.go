package qe_test

// Source: src/qe/qe_arith_plugin.cpp bounds_proc.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/qe"
)

func arithmeticQEMod(t *testing.T, f *arithmeticQEFixture, value, divisor ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	family, err := f.manager.ArithmeticFamilyID()
	pluginMust(t, err)
	return f.pin(f.manager.MkFamilyApp(family, 13, nil, []ast.ASTHandle{value, divisor}))
}

func TestArithmeticQEBoundsCollectDirectionsStrictnessAndReset(t *testing.T) {
	f := newArithmeticQEFixture(t)
	processor, err := qe.NewArithmeticBoundsProcessor(f.util)
	pluginMust(t, err)
	defer func() { pluginMust(t, processor.Close()) }()
	containsX, err := ast.NewContainsApp(f.manager, f.x)
	pluginMust(t, err)
	defer func() { pluginMust(t, containsX.Close()) }()
	zero := f.integerNumeral(t, "0")
	two := f.integerNumeral(t, "2")
	minusThree := f.integerNumeral(t, "-3")
	twoX := f.pin(f.manager.MkArithmeticMul(two, f.x))
	upperPolynomial := f.pin(f.manager.MkArithmeticAdd(f.y, twoX))
	upper := f.pin(f.manager.MkArithmeticLE(upperPolynomial, zero))
	handled, err := processor.Add(containsX, upper)
	pluginMust(t, err)
	if !handled {
		t.Fatal("upper bound not collected")
	}
	upperBounds := processor.Bounds(false, qe.ArithmeticUpperBound)
	if len(upperBounds) != 1 || upperBounds[0].Coefficient.Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal("upper bound coefficient", len(upperBounds), upperBounds[0].Coefficient)
	}
	term, err := f.manager.RefGet(upperBounds[0].Term)
	pluginMust(t, err)
	if term != f.y {
		t.Fatal("upper bound rest differs from y")
	}

	negativeX := f.pin(f.manager.MkArithmeticMul(minusThree, f.x))
	lowerPolynomial := f.pin(f.manager.MkArithmeticAdd(f.y, negativeX))
	lower := f.pin(f.manager.MkArithmeticLE(lowerPolynomial, zero))
	handled, err = processor.Add(containsX, lower)
	pluginMust(t, err)
	if !handled || len(processor.Bounds(false, qe.ArithmeticLowerBound)) != 1 {
		t.Fatal("lower bound not collected")
	}
	// Marking is insertion based: duplicates succeed without duplicating state.
	handled, err = processor.Add(containsX, lower)
	pluginMust(t, err)
	if !handled || len(processor.Bounds(false, qe.ArithmeticLowerBound)) != 1 {
		t.Fatal("duplicate lower bound changed state")
	}

	containsReal, err := ast.NewContainsApp(f.manager, f.rx)
	pluginMust(t, err)
	defer func() { pluginMust(t, containsReal.Close()) }()
	realZero := f.realNumeral(t, "0")
	realPolynomial := f.pin(f.manager.MkArithmeticAdd(f.ry, f.rx))
	strict := f.pin(f.manager.MkArithmeticLT(realPolynomial, realZero))
	handled, err = processor.Add(containsReal, strict)
	pluginMust(t, err)
	strictBounds := processor.Bounds(true, qe.ArithmeticUpperBound)
	if !handled || len(strictBounds) != 1 || strictBounds[0].Coefficient.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("strict real bound", handled, len(strictBounds))
	}

	pluginMust(t, processor.Reset())
	if len(processor.Bounds(false, qe.ArithmeticUpperBound)) != 0 || len(processor.Bounds(true, qe.ArithmeticUpperBound)) != 0 {
		t.Fatal("bounds survived reset")
	}
	handled, err = processor.Add(containsX, upper)
	pluginMust(t, err)
	if !handled || len(processor.Bounds(false, qe.ArithmeticUpperBound)) != 1 {
		t.Fatal("processor did not reuse after reset")
	}
}

func TestArithmeticQEBoundsDirectNegatedAndNestedDivisibility(t *testing.T) {
	f := newArithmeticQEFixture(t)
	processor, err := qe.NewArithmeticBoundsProcessor(f.util)
	pluginMust(t, err)
	defer func() { pluginMust(t, processor.Close()) }()
	contains, err := ast.NewContainsApp(f.manager, f.x)
	pluginMust(t, err)
	defer func() { pluginMust(t, contains.Close()) }()
	zero := f.integerNumeral(t, "0")
	one := f.integerNumeral(t, "1")
	two := f.integerNumeral(t, "2")
	seven := f.integerNumeral(t, "7")
	twoX := f.pin(f.manager.MkArithmeticMul(two, f.x))
	polynomial := f.pin(f.manager.MkArithmeticAdd(f.y, twoX))
	modular := arithmeticQEMod(t, f, polynomial, seven)
	divides := f.pin(f.manager.MkEq(zero, modular))
	handled, err := processor.Add(contains, divides)
	pluginMust(t, err)
	direct := processor.Divisibilities(false)
	if !handled || len(direct) != 1 || direct[0].Coefficient.Cmp(big.NewRat(2, 1)) != 0 || direct[0].Divisor.Cmp(big.NewRat(7, 1)) != 0 || direct[0].Negated {
		t.Fatal("direct divisibility collection")
	}
	negative := f.pin(f.manager.MkNot(divides))
	handled, err = processor.Add(contains, negative)
	pluginMust(t, err)
	direct = processor.Divisibilities(false)
	if !handled || len(direct) != 2 || !direct[1].Negated {
		t.Fatal("negated divisibility collection")
	}
	nestedAtom := f.pin(f.manager.MkEq(modular, one))
	handled, err = processor.Add(contains, nestedAtom)
	pluginMust(t, err)
	nested := processor.Divisibilities(true)
	if !handled || len(nested) != 1 || nested[0].Coefficient.Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal("nested divisibility collection", handled, len(nested))
	}
}
