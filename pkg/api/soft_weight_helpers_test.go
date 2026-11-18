package api

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// softRat is the small rational n/d (lowest terms, d > 0) as a borrowed
// soft-constraint weight; it holds no numeral storage.
func softRat(n, d int64) *numeral.Rational {
	return &numeral.Rational{Num: numeral.NewInt(int32(n)), Den: numeral.NewInt(int32(d))}
}

// numeralRatOf is NumeralRational as a test oracle *big.Rat (nil when the
// term is not a numeral).
func numeralRatOf(c *SMTContext, h ast.ASTHandle) (*big.Rat, bool, error) {
	m, err := c.Manager()
	if err != nil {
		return nil, false, err
	}
	q := m.NumeralService().Manager
	r := numeral.NewRational()
	found, err := c.NumeralRational(h, &r)
	var out *big.Rat
	if err == nil && found {
		out = new(big.Rat)
		err = q.BigRatOf(&r, out)
	}
	return out, found, errors.Join(err, q.Del(&r))
}
