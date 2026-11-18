package smtcore

import "github.com/glycerine/g3/pkg/util/numeral"

// Test helpers for old_interval values over testQ's rationals.

func oiRat(n, d int64) *numeral.Rational {
	r := smtQZero()
	if err := testQ().SetFractionSigned(&r, int32(n), int32(d)); err != nil {
		panic(err)
	}
	return &r
}

func oiR(n int64) *numeral.Rational { return oiRat(n, 1) }

func oiFinite(n, d int64) smtExtendedRational {
	v, err := smtFiniteExtended(testQ(), oiRat(n, d))
	if err != nil {
		panic(err)
	}
	return v
}

func oiClone(i *smtOldInterval) *smtOldInterval {
	c, err := i.clone()
	if err != nil {
		panic(err)
	}
	return c
}

func oiCloneExt(n smtExtendedRational) smtExtendedRational {
	c, err := n.clone(testQ())
	if err != nil {
		panic(err)
	}
	return c
}

func (n *smtExtendedRational) testString() string {
	s, err := n.String(testQ())
	if err != nil {
		panic(err)
	}
	return s
}

func (n *smtExtendedRational) testCmp(other *smtExtendedRational) int {
	c, err := n.cmp(testQ(), other)
	if err != nil {
		panic(err)
	}
	return c
}

func (n *smtExtendedRational) testEqual(other *smtExtendedRational) bool {
	return n.testCmp(other) == 0
}

// oiIs reports whether r is n/d.
func oiIs(r *numeral.Rational, n, d int64) bool {
	c, err := testQ().Compare(r, oiRat(n, d))
	if err != nil {
		panic(err)
	}
	return c == 0
}

func (i *smtOldInterval) testContains(r *numeral.Rational) bool {
	ok, err := i.contains(r)
	if err != nil {
		panic(err)
	}
	return ok
}
