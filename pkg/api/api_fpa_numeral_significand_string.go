package api

// Source: src/api/api_fpa.cpp:1003-1036.

import (
	"github.com/glycerine/g3/pkg/util/numeral"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

// FloatingPointNumeralSignificandString returns the source normalized decimal
// significand. Dyadic values terminate, so source decimal rendering has no
// repeating-value marker.
func (c *SMTContext) FloatingPointNumeralSignificandString(value ast.ASTHandle) (string, error) {
	parts, err := c.floatingPointRawNumeralParts(value)
	if err != nil {
		return "", err
	}
	if parts.Infinite {
		return "0", nil
	}
	if parts.SignificandBits == 0 {
		return "", ast.ErrASTDomain
	}
	m, err := c.Manager()
	if err != nil {
		return "", err
	}
	// scoped_mpq q; set(q, sig); add(q, 2^(sbits-1)) unless denormal;
	// div(q, 2^(sbits-1)); display_decimal(q, sbits)
	var text string
	err = withRationals(m, 2, func(q *numeral.RationalManager, rs []numeral.Rational) error {
		value, power := &rs[0], &rs[1]
		if q.SetString(value, parts.Significand) != nil || value.IsNeg() {
			return ast.ErrASTDomain
		}
		if e := powerOfTwo(q, parts.SignificandBits-1, power); e != nil {
			return e
		}
		if !parts.Denormal {
			if e := q.Add(value, power, value); e != nil {
				return e
			}
		}
		if e := q.Div(value, power, value); e != nil {
			return e
		}
		var e error
		text, e = q.DecimalString(value, parts.SignificandBits, false)
		return e
	})
	if err != nil {
		return "", err
	}
	if strings.Contains(text, ".") {
		text = strings.TrimRight(text, "0")
	}
	text = strings.TrimSuffix(text, ".")
	if text == "" {
		text = "0"
	}
	return text, nil
}
