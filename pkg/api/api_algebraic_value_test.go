package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIAlgebraicValueAndSign(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	root := f.sqrt2(t)
	negativeRoot, err := f.c.AlgebraicSub(f.zero, root)
	astTestMust(t, err)
	negativeRoot = f.pin(negativeRoot, nil)
	for _, test := range []struct {
		value    ast.ASTHandle
		sign     int
		positive bool
		negative bool
		zero     bool
	}{
		{f.zero, 0, false, false, true},
		{f.half, 1, true, false, false},
		{root, 1, true, false, false},
		{negativeRoot, -1, false, true, false},
	} {
		h := test.value
		if yes, err := f.c.IsAlgebraicValue(h); err != nil || !yes {
			t.Fatal("algebraic value", yes, err)
		}
		sign, err := f.c.AlgebraicSign(h)
		astTestMust(t, err)
		positive, err := f.c.AlgebraicIsPositive(h)
		astTestMust(t, err)
		negative, err := f.c.AlgebraicIsNegative(h)
		astTestMust(t, err)
		zero, err := f.c.AlgebraicIsZero(h)
		astTestMust(t, err)
		if sign != test.sign || positive != test.positive || negative != test.negative || zero != test.zero {
			t.Fatal("sign predicates", sign, positive, negative, zero)
		}
	}
	truth := f.pin(f.m.MkTrue())
	if yes, err := f.c.IsAlgebraicValue(truth); err != nil || yes {
		t.Fatal("Boolean classified as algebraic", yes, err)
	}
}
