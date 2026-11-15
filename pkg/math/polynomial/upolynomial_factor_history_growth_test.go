package polynomial

// AUDIT.md math_polynomial_09-F1: manager::factor_core and
// factor_square_free use only scoped locals, so repeated factorizations on
// one long-lived service must not retain per-call history.

import (
	"testing"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func TestUPolynomialFactorRepeatedCallsRetainNoHistory(t *testing.T) {
	m, _, _ := upolySourceOwner(t)
	s, e := NewUPolynomialFactorizationService(m, UPolynomialFactorizationConfig{})
	if e != nil {
		t.Fatal(e)
	}
	fs, e := m.NewFactors()
	if e != nil {
		t.Fatal(e)
	}
	p, e := m.NewPolynomial()
	if e != nil {
		t.Fatal(e)
	}
	// x^4 - 1 = (x - 1)(x + 1)(x^2 + 1): square-free part of degree 4.
	if e := m.SetInts(p, []unumeral.Int{unumeral.NewInt(-1), unumeral.NewInt(0), unumeral.NewInt(0), unumeral.NewInt(0), unumeral.NewInt(1)}); e != nil {
		t.Fatal(e)
	}
	v := mustUPolyView(t, m, p)
	var first UPolynomialFactorizationStats
	for i := 1; i <= 3000; i++ {
		if _, e := s.Factor(v, fs, DefaultUPolynomialFactorParams()); e != nil {
			t.Fatal(i, e)
		}
		if n, e := fs.DistinctFactors(); e != nil || n != 3 {
			t.Fatal("factorization of x^4 - 1", i, n, e)
		}
		if e := fs.Reset(); e != nil {
			t.Fatal(e)
		}
		if i == 1 {
			first = s.Stats()
		}
	}
	last := s.Stats()
	if last.CachedScopes != first.CachedScopes || last.CachedOwnerBytes != first.CachedOwnerBytes {
		t.Fatalf("history grew: after 1 call %d scopes/%d bytes, after 3000 calls %d scopes/%d bytes",
			first.CachedScopes, first.CachedOwnerBytes, last.CachedScopes, last.CachedOwnerBytes)
	}
	if e := fs.Close(); e != nil {
		t.Fatal(e)
	}
	if e := m.DelPolynomial(&p); e != nil {
		t.Fatal(e)
	}
	if e := s.Close(); e != nil {
		t.Fatal(e)
	}
}
