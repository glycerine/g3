package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// arith_project_util::project, integer coefficient/divisor normalization.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestLegacyArithmeticProjectionNormalizesIntegerCoefficients(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArithmeticProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	x := f.constant("legacy-integer-x", f.integer)
	one := f.own(m.ArithmeticIntegerNumeralText("1"))
	two := f.own(m.ArithmeticIntegerNumeralText("2"))
	three := f.own(m.ArithmeticIntegerNumeralText("3"))
	four := f.own(m.ArithmeticIntegerNumeralText("4"))
	zero := f.own(m.ArithmeticIntegerNumeralText("0"))
	twoX := f.own(m.MkArithmeticMul(two, x))
	dividend := f.own(m.MkArithmeticAdd(twoX, one))
	modulus := f.own(m.MkFamilyApp(f.r.arithmetic, 13, nil, []ast.ASTHandle{dividend, four}))
	divides := f.own(m.MkEq(modulus, zero))
	threeX := f.own(m.MkArithmeticMul(three, x))
	negativeThreeX := f.own(m.MkArithmeticNeg(threeX))
	boundTerm := f.own(m.MkArithmeticAdd(negativeThreeX, two))
	bound := f.own(m.MkArithmeticLE(boundTerm, zero))
	divLiteral, err := p.linearLiteral(x, divides)
	fmMust(t, err)
	boundLiteral, err := p.linearLiteral(x, bound)
	fmMust(t, err)
	if divLiteral == nil || boundLiteral == nil {
		t.Fatal("unclassified integer normalization input")
	}
	defer func() { fmMust(t, closeLegacyLinearLiterals(divLiteral, boundLiteral)) }()
	lcmCoefficients, lcmDivisors := unumeral.NewRational(), unumeral.NewRational()
	err = p.normalizeIntegers([]*legacyLinearLiteral{divLiteral, boundLiteral}, &lcmCoefficients, &lcmDivisors)
	fmMust(t, err)
	if !qeRatIs(p.syntax.manager, &lcmCoefficients, 6, 1) || !qeRatIs(p.syntax.manager, &lcmDivisors, 12, 1) {
		t.Fatal("integer normalization LCMs", qeRatText(p.syntax.manager, &lcmCoefficients), qeRatText(p.syntax.manager, &lcmDivisors))
	}
	if !qeRatIs(p.syntax.manager, &divLiteral.coefficient, 6, 1) || !qeRatIs(p.syntax.manager, &divLiteral.divisor, 12, 1) {
		t.Fatal("normalized divisibility literal")
	}
	if !qeRatIs(p.syntax.manager, &boundLiteral.coefficient, -6, 1) || !qeRatIs(p.syntax.manager, &boundLiteral.divisor, 0, 1) {
		t.Fatal("normalized inequality literal")
	}
	replacements, err := p.substituteIntegerLiterals(
		[]*legacyLinearLiteral{divLiteral, boundLiteral},
		[]ast.ASTHandle{divides, bound},
		four,
		1,
	)
	fmMust(t, err)
	if len(replacements.replacements) != 2 {
		t.Fatal("integer substitution replacement count", len(replacements.replacements))
	}
	divReplacement, err := m.RefGet(replacements.replacements[0].replacement)
	fmMust(t, err)
	contains, err := p.contains(divReplacement, x)
	fmMust(t, err)
	if contains {
		t.Fatal("integer divisibility replacement retains projected variable")
	}
	selectedReplacement, err := m.RefGet(replacements.replacements[1].replacement)
	fmMust(t, err)
	if !m.IsTrue(selectedReplacement) {
		t.Fatal("selected integer literal was not replaced by true")
	}
	fmMust(t, replacements.Close())
}
