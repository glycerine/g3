package api

// Source: src/api/api_fpa.cpp:908-1158.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/numeral"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *SMTContext) FloatingPointNumeralSign(value ast.ASTHandle) (int, error) {
	parts, err := c.floatingPointRawNumeralParts(value)
	if err != nil {
		return 0, err
	}
	if parts.Sign {
		return 1, nil
	}
	return 0, nil
}

func (c *SMTContext) FloatingPointNumeralSignificandUint64(value ast.ASTHandle) (uint64, error) {
	parts, err := c.floatingPointRawNumeralParts(value)
	if err != nil {
		return 0, err
	}
	// mpz const & z = mpfm.sig(val); mpzm.is_uint64(z); get_uint64(z)
	m, err := c.Manager()
	if err != nil {
		return 0, err
	}
	var result uint64
	err = withRationals(m, 1, func(q *numeral.RationalManager, rs []numeral.Rational) error {
		if q.SetString(&rs[0], parts.Significand) != nil {
			return ast.ErrASTDomain
		}
		ok, e := q.IsUint64(&rs[0])
		if e == nil && !ok {
			e = ast.ErrASTDomain
		}
		if e == nil {
			result, e = q.Uint64(&rs[0])
		}
		return e
	})
	return result, err
}

func (c *SMTContext) FloatingPointNumeralExponentInt64(value ast.ASTHandle, biased bool) (int64, error) {
	parts, err := c.floatingPointRawNumeralParts(value)
	if err != nil {
		return 0, err
	}
	m, err := c.Manager()
	if err != nil {
		return 0, err
	}
	return floatingPointNumeralExponent(m, parts, biased)
}

// floatingPointNumeralExponent follows api_fpa.cpp's MPF field access, including
// its distinct zero, infinity and denormal cases.
func floatingPointNumeralExponent(m *ast.ASTManager, parts ast.FPANumeralParts, biased bool) (int64, error) {
	if parts.Zero {
		return 0, nil
	}
	if parts.ExponentBits < 2 {
		return 0, ast.ErrASTDomain
	}
	var value int64
	err := withRationals(m, 2, func(q *numeral.RationalManager, rs []numeral.Rational) error {
		top, result := &rs[0], &rs[1]
		if e := powerOfTwo(q, parts.ExponentBits-1, top); e != nil {
			return e
		}
		var e error
		if parts.Infinite {
			e = q.Copy(result, top)
		} else if parts.Denormal && !biased {
			e = errors.Join(q.SetInt32(result, 2), q.Sub(result, top, result))
		} else {
			e = q.SetInt64(result, parts.Exponent)
			if e == nil && biased {
				e = errors.Join(q.Add(result, top, result), q.Dec(result))
			}
		}
		if e != nil {
			return e
		}
		ok, e := q.IsInt64(result)
		if e == nil && !ok {
			e = ast.ErrASTDomain
		}
		if e == nil {
			value, e = q.Int64(result)
		}
		return e
	})
	return value, err
}
