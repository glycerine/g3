package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestLegacyMappedProjectionPublishesEqualitySubstitution(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArithmeticProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	x := f.constant("mapped-equality-x", f.real)
	two := f.own(m.ArithmeticRealNumeralText("2"))
	equality := f.own(m.MkEq(x, two))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	decl, err := m.GetDecl(x)
	fmMust(t, err)
	fmMust(t, md.Core().RegisterConstant(decl, two))
	mapping, err := ast.NewASTExprMapWithProofs(m, false)
	fmMust(t, err)
	defer func() { fmMust(t, mapping.Close()) }()
	projected, err := p.projectFormulaMapped(md.Core(), []ast.ASTHandle{x}, equality, mapping)
	fmMust(t, err)
	defer func() { fmMust(t, projected.Close()) }()
	value, _, found, err := mapping.Get(x)
	fmMust(t, err)
	if !found || value != two {
		t.Fatal("equality substitution map", found, value, two)
	}
}
