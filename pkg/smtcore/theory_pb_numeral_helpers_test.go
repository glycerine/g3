package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// Test helpers for pseudo-Boolean rationals.

// pbRat is the rational with the given text on q.
func pbRat(t testing.TB, q *numeral.RationalManager, text string) numeral.Rational {
	t.Helper()
	r := smtQZero()
	astTestMust(t, q.SetString(&r, text))
	return r
}

// pbText is r's text on q.
func pbText(q *numeral.RationalManager, r *numeral.Rational) string {
	s, err := q.String(r)
	if err != nil {
		panic(err)
	}
	return s
}

// pbZText is n's text on q's integer manager.
func pbZText(q *numeral.RationalManager, n *numeral.Int) string {
	s, err := q.Integers().String(n)
	if err != nil {
		panic(err)
	}
	return s
}

// pbArgsOf builds an arg_t over q with coefficient texts and bound text.
func pbArgsOf(t testing.TB, q *numeral.RationalManager, bound string, literals []SMTLiteral, coefficients []string) smtPBArgs {
	t.Helper()
	a := newSMTPBArgs()
	a.bound = pbRat(t, q, bound)
	for i, l := range literals {
		c := pbRat(t, q, coefficients[i])
		astTestMust(t, a.pushBack(q, l, &c))
		astTestMust(t, smtQDel(q, &c))
	}
	return a
}

// pbZInt is n as an int64 on q's integer manager.
func pbZInt(q *numeral.RationalManager, n *numeral.Int) int64 {
	v, err := q.Integers().Int64(n)
	if err != nil {
		panic(err)
	}
	return v
}
