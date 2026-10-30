package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestLegacyMappedProjectionPublishesLiteralReplacement(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArithmeticProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	x := f.constant("mapped-upper-x", f.real)
	one := f.own(m.ArithmeticRealNumeralText("1"))
	three := f.own(m.ArithmeticRealNumeralText("3"))
	upper := f.own(m.MkArithmeticLE(x, three))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	decl, err := m.GetDecl(x)
	fmMust(t, err)
	fmMust(t, md.Core().RegisterConstant(decl, one))
	mapping, err := ast.NewASTExprMapWithProofs(m, false)
	fmMust(t, err)
	defer func() { fmMust(t, mapping.Close()) }()
	projected, err := p.projectFormulaMapped(md.Core(), []ast.ASTHandle{x}, upper, mapping)
	fmMust(t, err)
	defer func() { fmMust(t, projected.Close()) }()
	value, _, found, err := mapping.Get(upper)
	fmMust(t, err)
	if !found || !m.IsTrue(value) {
		t.Fatal("one-sided replacement map", found, value)
	}
}
