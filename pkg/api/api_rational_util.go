package api

// Rationals of the AST manager's NumeralService().Manager, the API's
// `rational` locals (src/api/api_*.cpp).

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// withRationals runs body with n fresh zero rationals of m's rational
// manager and destroys them afterwards.
func withRationals(m *ast.ASTManager, n int, body func(q *numeral.RationalManager, rs []numeral.Rational) error) (err error) {
	q := m.NumeralService().Manager
	rs := make([]numeral.Rational, n)
	for i := range rs {
		rs[i] = numeral.NewRational()
	}
	defer func() {
		for i := range rs {
			err = errors.Join(err, q.Del(&rs[i]))
		}
	}()
	return body(q, rs)
}

// powerOfTwo sets out to 2^k (rational::power_of_two).
func powerOfTwo(q *numeral.RationalManager, k uint32, out *numeral.Rational) error {
	return errors.Join(q.SetInt32(out, 2), q.Power(out, k, out))
}
