package arith

// Rational helpers shared by the arithmetic tactics: their source holds
// `rational` values; these are rationals of the AST manager's
// NumeralService().Manager.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// rationalUnsigned is r.is_unsigned() and r.get_unsigned().
func rationalUnsigned(q *unumeral.RationalManager, r *unumeral.Rational) (uint32, bool, error) {
	ok, err := q.IsUnsigned(r)
	if err != nil || !ok {
		return 0, false, err
	}
	v, err := q.Unsigned(r)
	return v, err == nil, err
}

// numeralUnsigned is a.is_numeral(h, r) followed by r.is_unsigned() and
// r.get_unsigned().
func numeralUnsigned(m *ast.ASTManager, h ast.ASTHandle) (value uint32, numeral, unsigned bool, resultErr error) {
	q := m.NumeralService().Manager
	r := unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&r)) }()
	_, numeral, err := m.ArithmeticNumeralRational(h, &r)
	if err != nil || !numeral {
		return 0, false, false, err
	}
	value, unsigned, err = rationalUnsigned(q, &r)
	return value, true, unsigned, err
}

// rationalBitLength is the bit length of a nonnegative integer rational
// (zero for zero).
func rationalBitLength(q *unumeral.RationalManager, r *unumeral.Rational) (uint32, error) {
	zero, err := q.IsZero(r)
	if err != nil || zero {
		return 0, err
	}
	return q.Bitsize(r)
}

// rationalCompareInt is r compared with the integer n (-1, 0 or 1).
func rationalCompareInt(q *unumeral.RationalManager, r *unumeral.Rational, n int64) (int, error) {
	v := unumeral.NewRational()
	err := q.SetInt64(&v, n)
	c := 0
	if err == nil {
		c, err = q.Compare(r, &v)
	}
	return c, errors.Join(err, q.Del(&v))
}
