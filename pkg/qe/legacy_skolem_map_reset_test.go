package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestLegacyMappedProjectionResetsMapForEachVariable(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArithmeticProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	x := f.constant("mapped-reset-x", f.real)
	y := f.constant("mapped-reset-y", f.real)
	one := f.own(m.ArithmeticRealNumeralText("1"))
	three := f.own(m.ArithmeticRealNumeralText("3"))
	upperX := f.own(m.MkArithmeticLE(x, three))
	upperY := f.own(m.MkArithmeticLE(y, three))
	formula := f.own(m.MkAnd(upperX, upperY))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	for _, variable := range []ast.ASTHandle{x, y} {
		decl, xerr := m.GetDecl(variable)
		fmMust(t, xerr)
		fmMust(t, md.Core().RegisterConstant(decl, one))
	}
	mapping, err := ast.NewASTExprMapWithProofs(m, false)
	fmMust(t, err)
	defer func() { fmMust(t, mapping.Close()) }()
	projected, err := p.projectFormulaMapped(md.Core(), []ast.ASTHandle{x, y}, formula, mapping)
	fmMust(t, err)
	defer func() { fmMust(t, projected.Close()) }()
	first, err := mapping.Contains(upperX)
	fmMust(t, err)
	last, err := mapping.Contains(upperY)
	fmMust(t, err)
	if first || !last {
		t.Fatal("source map reset order", first, last)
	}
}
