package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func qPolyFromInts(t *testing.T, s *realValueStore, values ...int64) []numeral.Rational {
	t.Helper()
	out := newQPoly(len(values))
	for i, v := range values {
		mustRealFunction(t, s.numbers.SetInt64(&out[i], v))
	}
	return out
}

func qPolyTexts(t *testing.T, s *realValueStore, p []numeral.Rational) []string {
	t.Helper()
	out := make([]string, len(p))
	for i := range p {
		text, err := s.numbers.String(&p[i])
		mustRealFunction(t, err)
		out[i] = text
	}
	return out
}

func TestAlgebraicRationalPolynomialDivisionAndInverseIdentity(t *testing.T) {
	f := newRealFunctionFixture(t)
	s := f.store
	q := qPolyFromInts(t, s, 0, 1)
	p := qPolyFromInts(t, s, -2, 0, 1)
	defer func() { mustRealFunction(t, s.qPolyDel(q)); mustRealFunction(t, s.qPolyDel(p)) }()
	h, ok, err := s.invertAlgebraicPolynomial(q, p)
	mustRealFunction(t, err)
	defer func() { mustRealFunction(t, s.qPolyDel(h)) }()
	if texts := qPolyTexts(t, s, h); !ok || len(texts) != 2 || texts[0] != "0" || texts[1] != "1/2" {
		t.Fatal("extended gcd inverse", ok, texts)
	}
	product, err := s.qPolyMul(q, h)
	mustRealFunction(t, err)
	remainder, ok, err := s.qPolyRem(product, p)
	mustRealFunction(t, err)
	mustRealFunction(t, s.qPolyDel(product))
	defer func() { mustRealFunction(t, s.qPolyDel(remainder)) }()
	if texts := qPolyTexts(t, s, remainder); !ok || len(texts) != 1 || texts[0] != "1" {
		t.Fatal("inverse identity modulo definition", ok, texts)
	}
}
