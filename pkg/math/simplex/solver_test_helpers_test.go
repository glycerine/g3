package simplex

import (
	"math/big"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// rat is a fresh rational n/d of optQ (tests do not free them).
func rat(n, d int64) *numeral.Rational {
	r := numeral.NewRational()
	if err := optQ().SetFraction(&r, n, uint64(d)); err != nil {
		if d < 0 {
			return rat(-n, -d)
		}
		panic(err)
	}
	return &r
}

// ratText is a fresh rational of optQ parsed from text.
func ratText(text string) *numeral.Rational {
	r := numeral.NewRational()
	if err := optQ().SetString(&r, text); err != nil {
		panic(err)
	}
	return &r
}

// ratBig is r as a math/big value, for oracle comparisons.
func ratBig(r *numeral.Rational) *big.Rat {
	out := new(big.Rat)
	if err := optQ().BigRatOf(r, out); err != nil {
		panic(err)
	}
	return out
}

// bigRat is a fresh rational of optQ with x's value.
func bigRat(x *big.Rat) *numeral.Rational {
	r := numeral.NewRational()
	if err := optQ().SetBigRat(&r, x); err != nil {
		panic(err)
	}
	return &r
}

func integerRow(values ...int64) []numeral.Rational {
	r := make([]numeral.Rational, len(values))
	for i, v := range values {
		r[i] = *rat(v, 1)
	}
	return r
}

func qcmp(a, b *numeral.Rational) int {
	c, err := optQ().Compare(a, b)
	if err != nil {
		panic(err)
	}
	return c
}

func testRI(v *numeral.Rational) *Infinitesimal {
	r, err := RationalInfinitesimal(optQ(), v)
	if err != nil {
		panic(err)
	}
	return &r
}

func testNI(main, epsilon *numeral.Rational) *Infinitesimal {
	r, err := NewInfinitesimal(optQ(), main, epsilon)
	if err != nil {
		panic(err)
	}
	return &r
}

func testNII(infinity, main, epsilon *numeral.Rational) *Infinitesimal {
	r, err := NewInfiniteInfinitesimal(optQ(), infinity, main, epsilon)
	if err != nil {
		panic(err)
	}
	return &r
}

func icmp(a, b *Infinitesimal) int {
	c, err := a.Cmp(optQ(), b)
	if err != nil {
		panic(err)
	}
	return c
}

func itext(a *Infinitesimal) string {
	s, err := a.String(optQ())
	if err != nil {
		panic(err)
	}
	return s
}

func iclone(a *Infinitesimal) *Infinitesimal {
	r, err := a.Clone(optQ())
	if err != nil {
		panic(err)
	}
	return &r
}

func newTestSolver(limit *control.Reslimit) *Solver {
	s, err := NewSolver(limit, testNumerals())
	if err != nil {
		panic(err)
	}
	return s
}

func testValue(s *Solver, v uint32) *Infinitesimal {
	r := ZeroInfinitesimal()
	if err := s.Value(v, &r); err != nil {
		panic(err)
	}
	return &r
}

func testMatrix() *Matrix { return NewMatrix(optQ()) }
