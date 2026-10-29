package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type apiAlgebraicFixture struct {
	*smtAPIFixture
	zero, one, two, three, half ast.ASTHandle
}

func newAPIAlgebraicFixture(t *testing.T) *apiAlgebraicFixture {
	f := newSMTAPIFixture(t, "ALL", nil)
	realSort := f.pin(f.c.RealSort())
	numeral := func(text string) ast.ASTHandle { return f.pin(f.c.Numeral(text, realSort)) }
	return &apiAlgebraicFixture{
		smtAPIFixture: f,
		zero:          numeral("0"),
		one:           numeral("1"),
		two:           numeral("2"),
		three:         numeral("3"),
		half:          numeral("1/2"),
	}
}

func (f *apiAlgebraicFixture) sqrt2(t *testing.T) ast.ASTHandle {
	root, err := f.c.AlgebraicRoot(f.two, 2)
	astTestMust(t, err)
	return f.pin(root, nil)
}
