package smtcore

import (
	"math/big"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// Test-only math/big views of the LRA frontend's rationals.

// lraQ is the inline rational n/d (small values only; no manager needed).
func lraQ(n, d int64) numeral.Rational {
	if d < 0 {
		n, d = -n, -d
	}
	a, b := n, d
	if a < 0 {
		a = -a
	}
	for b != 0 {
		a, b = b, a%b
	}
	if a > 1 {
		n, d = n/a, d/a
	}
	return numeral.Rational{Num: numeral.NewInt(int32(n)), Den: numeral.NewInt(int32(d))}
}

func (s *smtLRANumeric) testQ(b *big.Rat) *numeral.Rational {
	r := numeral.NewRational()
	if err := s.manager.SetBigRat(&r, b); err != nil {
		panic(err)
	}
	return &r
}

func (t *SMTLRATheory) testQ(b *big.Rat) *numeral.Rational {
	r := numeral.NewRational()
	if err := t.numerals.SetBigRat(&r, b); err != nil {
		panic(err)
	}
	return &r
}

// The wrappers below free their converted operand after the call.

func (s *smtLRANumeric) testAssertBound(v SMTTheoryVar, bv int32, value *big.Rat, upper, truth bool) (uint32, error) {
	if value == nil {
		return s.assertBound(v, bv, nil, upper, truth)
	}
	q := s.testQ(value)
	defer s.manager.Del(q)
	return s.assertBound(v, bv, q, upper, truth)
}

func (s *smtLRANumeric) testHasRationalBound(index uint32, lower bool, expected *big.Rat) (uint32, bool, error) {
	q := s.testQ(expected)
	defer s.manager.Del(q)
	return s.hasRationalBound(index, lower, q)
}

func (s *smtLRANumeric) testAddTerm(v SMTTheoryVar, coefficients []smtLRACoefficient, constant *big.Rat) (uint32, error) {
	if constant == nil {
		return s.addTerm(v, coefficients, nil)
	}
	q := s.testQ(constant)
	defer s.manager.Del(q)
	return s.addTerm(v, coefficients, q)
}

func (t *SMTLRATheory) testHasFixedBound(index uint32, value *big.Rat, lower bool) (uint32, bool, error) {
	q := t.testQ(value)
	defer t.numerals.Del(q)
	return t.hasFixedBound(index, q, lower)
}

func (t *SMTLRATheory) testFixedKey(v SMTTheoryVar, value *big.Rat) (smtLRAFixedKey, error) {
	q := t.testQ(value)
	defer t.numerals.Del(q)
	return t.fixedKey(v, q)
}

// testBigOf is a math/big copy of one of the theory's rationals.
func (t *SMTLRATheory) testBigOf(r *numeral.Rational) *big.Rat {
	b, err := smtQBig(t.numerals, r)
	if err != nil {
		panic(err)
	}
	return b
}
