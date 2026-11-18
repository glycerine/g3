package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// qeSmallRat is the small rational n/d (d > 0, gcd 1). Small rationals hold
// no numeral storage, so they need no manager and no destruction.
func qeSmallRat(n, d int64) *unumeral.Rational {
	r := unumeral.Rational{Num: unumeral.NewInt(int32(n)), Den: unumeral.NewInt(int32(d))}
	return &r
}

// qeRatIs reports whether r is exactly n/d.
func qeRatIs(m *ast.ASTManager, r *unumeral.Rational, n, d int64) bool {
	equal, err := m.NumeralService().Manager.Equal(r, qeSmallRat(n, d))
	return err == nil && equal
}

// qeRatText is rational::to_string.
func qeRatText(m *ast.ASTManager, r *unumeral.Rational) string {
	text, err := m.NumeralService().Manager.String(r)
	if err != nil {
		return "error:" + err.Error()
	}
	return text
}
