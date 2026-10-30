package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// arith_project_util::factor_mod_terms.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestLegacyArithmeticProjectionFactorsSharedModTerms(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArithmeticProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	x := f.constant("legacy-factor-mod-x", f.integer)
	one := f.own(m.ArithmeticIntegerNumeralText("1"))
	three := f.own(m.ArithmeticIntegerNumeralText("3"))
	four := f.own(m.ArithmeticIntegerNumeralText("4"))
	sum := f.own(m.MkArithmeticAdd(x, one))
	modulus := f.own(m.MkFamilyApp(f.r.arithmetic, 13, nil, []ast.ASTHandle{sum, three}))
	formula := f.own(m.MkEq(modulus, modulus))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	xDecl, err := m.GetDecl(x)
	fmMust(t, err)
	fmMust(t, md.Core().RegisterConstant(xDecl, four))
	result, err := p.factorModTerms(md.Core(), formula)
	fmMust(t, err)
	defer func() { fmMust(t, result.Close()) }()
	if len(result.variables) != 1 {
		t.Fatal("shared mod term fresh-variable count", len(result.variables))
	}
	variable, err := m.RefGet(result.variables[0])
	fmMust(t, err)
	decl, err := m.GetDecl(variable)
	fmMust(t, err)
	value, err := md.Core().ConstantInterpretation(decl)
	fmMust(t, err)
	if value == (ast.ASTHandle{}) {
		t.Fatal("fresh mod variable missing model interpretation")
	}
	text, _, numeral, err := m.ArithmeticNumeralText(value)
	fmMust(t, err)
	if !numeral || text != "2" {
		t.Fatal("fresh mod variable model value", text, numeral)
	}
	handle, err := m.RefGet(result.formula)
	fmMust(t, err)
	family, err := m.GetFamilyID(handle)
	fmMust(t, err)
	op, err := m.GetDeclKind(handle)
	fmMust(t, err)
	if family != m.BasicFamilyID() || op != ast.ASTOpAnd {
		t.Fatal("factored formula did not conjoin definition", family, op)
	}
}

func TestLegacyModFactoringFollowsSourceSiblingOrder(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArithmeticProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	x := f.constant("mod-stack-x", f.integer)
	zero := f.own(m.ArithmeticIntegerNumeralText("0"))
	three := f.own(m.ArithmeticIntegerNumeralText("3"))
	five := f.own(m.ArithmeticIntegerNumeralText("5"))
	seven := f.own(m.ArithmeticIntegerNumeralText("7"))
	left := f.own(m.MkFamilyApp(f.r.arithmetic, 13, nil, []ast.ASTHandle{x, three}))
	right := f.own(m.MkFamilyApp(f.r.arithmetic, 13, nil, []ast.ASTHandle{x, five}))
	geLeft := f.own(m.MkArithmeticGE(left, zero))
	geRight := f.own(m.MkArithmeticGE(right, zero))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	decl, err := m.GetDecl(x)
	fmMust(t, err)
	fmMust(t, md.Core().RegisterConstant(decl, seven))
	for _, shared := range []bool{false, true} {
		forms := []ast.ASTHandle{geLeft, geRight}
		if shared {
			forms = append(forms, geRight)
		}
		formula := f.own(m.MkAndArray(forms))
		result, err := p.factorModTerms(md.Core(), formula)
		fmMust(t, err)
		if len(result.variables) != 2 {
			t.Fatal("shared mod terms must be factored once", shared, len(result.variables))
		}
		for index, want := range []string{"2", "1"} {
			variable, err := m.RefGet(result.variables[index])
			fmMust(t, err)
			decl, err := m.GetDecl(variable)
			fmMust(t, err)
			value, err := md.Core().ConstantInterpretation(decl)
			fmMust(t, err)
			text, _, numeral, err := m.ArithmeticNumeralText(value)
			fmMust(t, err)
			if !numeral || text != want {
				t.Fatal("source fresh-mod order", shared, index, text, want)
			}
		}
		fmMust(t, result.Close())
	}
}

func TestLegacyModFactoringRebuildsNestedSharedTerms(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArithmeticProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	x := f.constant("mod-nested-x", f.integer)
	zero := f.own(m.ArithmeticIntegerNumeralText("0"))
	one := f.own(m.ArithmeticIntegerNumeralText("1"))
	three := f.own(m.ArithmeticIntegerNumeralText("3"))
	five := f.own(m.ArithmeticIntegerNumeralText("5"))
	seven := f.own(m.ArithmeticIntegerNumeralText("7"))
	inner := f.own(m.MkFamilyApp(f.r.arithmetic, 13, nil, []ast.ASTHandle{x, three}))
	sum := f.own(m.MkArithmeticAdd(inner, one))
	outer := f.own(m.MkFamilyApp(f.r.arithmetic, 13, nil, []ast.ASTHandle{sum, five}))
	innerBound := f.own(m.MkArithmeticGE(inner, zero))
	outerBound := f.own(m.MkArithmeticGE(outer, zero))
	formula := f.own(m.MkAndArray([]ast.ASTHandle{innerBound, outerBound, outerBound}))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	decl, err := m.GetDecl(x)
	fmMust(t, err)
	fmMust(t, md.Core().RegisterConstant(decl, seven))
	result, err := p.factorModTerms(md.Core(), formula)
	fmMust(t, err)
	defer func() { fmMust(t, result.Close()) }()
	if len(result.variables) != 2 {
		t.Fatal("nested shared mod variable count", len(result.variables))
	}
	for index, want := range []string{"1", "2"} {
		variable, err := m.RefGet(result.variables[index])
		fmMust(t, err)
		decl, err := m.GetDecl(variable)
		fmMust(t, err)
		value, err := md.Core().ConstantInterpretation(decl)
		fmMust(t, err)
		text, _, numeral, err := m.ArithmeticNumeralText(value)
		fmMust(t, err)
		if !numeral || text != want {
			t.Fatal("nested mod interpretation", index, text, want)
		}
	}
	root, err := m.RefGet(result.formula)
	fmMust(t, err)
	definitions, err := m.GetArg(root, 1)
	fmMust(t, err)
	outerDefinition, err := m.GetArg(definitions, 1)
	fmMust(t, err)
	rhs, err := m.GetArg(outerDefinition, 1)
	fmMust(t, err)
	innerVariable, err := m.RefGet(result.variables[0])
	fmMust(t, err)
	occurs, err := m.OccursExpr(innerVariable, rhs)
	fmMust(t, err)
	if !occurs {
		t.Fatal("outer mod definition does not use the inner fresh variable", rhs)
	}
	occurs, err = m.OccursExpr(inner, rhs)
	fmMust(t, err)
	if occurs {
		t.Fatal("outer definition kept the original shared inner mod", rhs)
	}
}
