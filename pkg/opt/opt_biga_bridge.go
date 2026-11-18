package opt

// TODO bigA: temporary bridge to the big.Rat APIs of pkg/smtcore
// (theory_wmaxsat) and pkg/math/simplex (Infinitesimal, inf_eps). It goes
// away when those take numeral rationals; nothing else in opt uses big.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// bigOf copies r into a big.Rat for a bigA API.
func (o *ratOps) bigOf(r *numeral.Rational) *big.Rat {
	out := new(big.Rat)
	if o.ok() {
		o.fail(o.q.BigRatOf(r, out))
	}
	return out
}

// fromBig sets dst from a bigA API's big.Rat.
func (o *ratOps) fromBig(dst *numeral.Rational, r *big.Rat) {
	if o.ok() {
		o.fail(o.q.SetBigRat(dst, r))
	}
}

// infinitesimalOf is inf_eps(r) for the simplex API.
func (o *ratOps) infinitesimalOf(r *numeral.Rational) simplex.Infinitesimal {
	return simplex.RationalInfinitesimal(o.bigOf(r))
}

// infiniteInfinitesimal is inf_eps(rational(sign), 0, 0): +oo or -oo.
func infiniteInfinitesimal(sign int64) simplex.Infinitesimal {
	return simplex.NewInfiniteInfinitesimal(big.NewRat(sign, 1), nil, nil)
}
