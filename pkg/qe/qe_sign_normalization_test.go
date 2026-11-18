package qe

// Source: src/qe/qe_arith_plugin.cpp mk_resolve's SASSERT(a.is_neg() !=
// b.is_neg()) and src/muz/spacer/spacer_qe_project.cpp mk_lt/mk_le's
// coefficient sign conditions. The numeral manager's Sign is the raw small
// value, so these must compare normalized signs.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestLegacyArithmeticBoundsLargeCoefficientSigns(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArithmeticProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	x := f.constant("legacy-sign-x", f.real)
	y := f.constant("legacy-sign-y", f.real)
	zero := f.own(m.ArithmeticRealNumeralText("0"))
	// 65536 * 65536 overflows int32 to zero.
	big := f.own(m.ArithmeticRealNumeralText("65536"))
	negBig := f.own(m.ArithmeticRealNumeralText("-65536"))
	bigX := f.own(m.MkArithmeticMul(big, x))
	negBigX := f.own(m.MkArithmeticMul(negBig, x))
	upper := f.own(m.MkArithmeticLE(f.own(m.MkArithmeticAdd(bigX, y)), zero))
	lower := f.own(m.MkArithmeticLE(f.own(m.MkArithmeticAdd(negBigX, y)), zero))
	z := f.constant("legacy-sign-z", f.real)
	upper2 := f.own(m.MkArithmeticLE(f.own(m.MkArithmeticAdd(bigX, z)), zero))
	classify := func(expression ast.ASTHandle) *legacyLinearLiteral {
		literal, e := p.linearLiteral(x, expression)
		fmMust(t, e)
		if literal == nil {
			t.Fatal("unclassified comparison input")
		}
		return literal
	}
	upperLiteral, lowerLiteral, upper2Literal := classify(upper), classify(lower), classify(upper2)
	defer func() { fmMust(t, closeLegacyLinearLiterals(upperLiteral, lowerLiteral, upper2Literal)) }()
	opposite, err := p.oppositeBound(upperLiteral, lowerLiteral)
	fmMust(t, err)
	fmMust(t, m.DelRef(&opposite))
	same, err := p.sameBound(upperLiteral, upper2Literal)
	fmMust(t, err)
	fmMust(t, m.DelRef(&same))
}
