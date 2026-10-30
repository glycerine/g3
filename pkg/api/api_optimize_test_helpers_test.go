package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type apiOptimizeFixture struct {
	*smtAPIFixture
	opt           *APIOptimize
	x, zero, five ast.ASTHandle
	lower, upper  ast.ASTHandle
}

func newAPIOptimizeFixture(t *testing.T) *apiOptimizeFixture {
	f := newSMTAPIFixture(t, "ALL", nil)
	x := f.arithmeticVariable("api-object-opt-x", true)
	zero := f.pin(f.m.ArithmeticIntegerNumeralText("0"))
	five := f.pin(f.m.ArithmeticIntegerNumeralText("5"))
	lower := f.pin(f.m.MkArithmeticGE(x, zero))
	upper := f.pin(f.m.MkArithmeticLE(x, five))
	optimize, err := f.c.NewOptimize()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, optimize.Close()) })
	return &apiOptimizeFixture{smtAPIFixture: f, opt: optimize, x: x, zero: zero, five: five, lower: lower, upper: upper}
}
