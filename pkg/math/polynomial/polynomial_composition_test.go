package polynomial

// Source: src/math/polynomial/polynomial.cpp:4653-4890,7248-7284;
// src/test/polynomial.cpp:561-575.

import (
	"fmt"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"testing"
)

// The registered source driver returns after PSC before tst_compose. These two
// exact dormant ENSUREs use dense term construction instead of expression refs.
func TestPolynomialCompositionDormantOriginal(t *testing.T) {
	m, p, _ := polynomialEvalFixture(t, []polynomialEvalTerm{{1, [3]uint32{3, 0, 0}}, {-1, [3]uint32{1, 0, 0}}, {3, [3]uint32{}}})
	minus, e := NewPolynomialRef(m.core)
	serviceMust(t, e)
	t.Cleanup(func() { serviceMust(t, minus.Close()) })
	plus, e := NewPolynomialRef(m.core)
	serviceMust(t, e)
	t.Cleanup(func() { serviceMust(t, plus.Close()) })
	serviceMust(t, m.ComposeXMinusY(p, 1, minus))
	serviceMust(t, m.ComposeXPlusY(p, 1, plus))
	wantedMinus := []polynomialEvalTerm{{1, [3]uint32{3, 0, 0}}, {-3, [3]uint32{2, 1, 0}}, {3, [3]uint32{1, 2, 0}}, {-1, [3]uint32{0, 3, 0}}, {-1, [3]uint32{1, 0, 0}}, {1, [3]uint32{0, 1, 0}}, {3, [3]uint32{}}}
	wantedPlus := []polynomialEvalTerm{{1, [3]uint32{3, 0, 0}}, {3, [3]uint32{2, 1, 0}}, {3, [3]uint32{1, 2, 0}}, {1, [3]uint32{0, 3, 0}}, {-1, [3]uint32{1, 0, 0}}, {-1, [3]uint32{0, 1, 0}}, {3, [3]uint32{}}}
	for i, expected := range [][]polynomialEvalTerm{wantedMinus, wantedPlus} {
		want := polynomialCompositionTerms(t, m, expected)
		actual := minus.root.value
		if i == 1 {
			actual = plus.root.value
		}
		equal, e := m.core.Equal(actual, want)
		serviceMust(t, e)
		if !equal {
			t.Fatal("original dormant composition ENSURE", i)
		}
	}
}

func polynomialCompositionTerms(t *testing.T, m *MultivariatePolynomialManager, terms []polynomialEvalTerm) Polynomial {
	t.Helper()
	as := make([]unumeral.Int, len(terms))
	ms := make([]Monomial, len(terms))
	for i, v := range terms {
		serviceMust(t, m.core.modular.SetInt32(&as[i], v.coefficient))
		powers := []MonomialPower{}
		for x, d := range v.degrees {
			if d != 0 {
				powers = append(powers, MonomialPower{Var: uint32(x), Degree: d})
			}
		}
		var e error
		ms[i], e = m.core.monomials.MkMonomial(powers)
		serviceMust(t, e)
	}
	p, e := m.core.MkTerms(as, ms)
	serviceMust(t, e)
	serviceMust(t, m.core.IncRef(p))
	t.Cleanup(func() { serviceMust(t, m.core.DecRef(p)) })
	return p
}

func TestPolynomialCompositionReciprocalAndVariableOrders(t *testing.T) {
	m, p, _ := polynomialEvalFixture(t, []polynomialEvalTerm{{2, [3]uint32{0, 3, 0}}, {-3, [3]uint32{0, 1, 0}}, {5, [3]uint32{}}})
	reciprocal, e := m.ComposeReciprocal(p)
	serviceMust(t, e)
	serviceMust(t, m.core.IncRef(reciprocal))
	t.Cleanup(func() { serviceMust(t, m.core.DecRef(reciprocal)) })
	want := polynomialCompositionTerms(t, m, []polynomialEvalTerm{{5, [3]uint32{0, 3, 0}}, {-3, [3]uint32{0, 2, 0}}, {2, [3]uint32{}}})
	eq, e := m.core.Equal(reciprocal, want)
	serviceMust(t, e)
	if !eq {
		t.Fatal("independent reversed coefficients")
	}
	for _, y := range []uint32{0, 2} {
		got, e := m.ComposeXDivY(p, y)
		serviceMust(t, e)
		serviceMust(t, m.core.IncRef(got))
		t.Cleanup(func() { serviceMust(t, m.core.DecRef(got)) })
		terms := []polynomialEvalTerm{{2, [3]uint32{0, 3, 0}}, {-3, [3]uint32{0, 1, 0}}, {5, [3]uint32{}}}
		for i := range terms {
			terms[i].degrees[y] = 3 - terms[i].degrees[1]
		}
		expected := polynomialCompositionTerms(t, m, terms)
		eq, e = m.core.Equal(got, expected)
		serviceMust(t, e)
		if !eq {
			t.Fatal("source x/y power order", y)
		}
	}
}

func polynomialCompositionFiniteValue(t *testing.T, m *MultivariatePolynomialManager, p Polynomial, values [3]int64) int64 {
	t.Helper()
	n, e := m.core.readWord(p, 8)
	serviceMust(t, e)
	sum := int64(0)
	for i := uint32(0); i < n; i++ {
		a, e := m.core.coefficientCore(p, i)
		serviceMust(t, e)
		term, e := m.core.integer.Int64(&a)
		serviceMust(t, e)
		mono, e := m.core.monomialCore(p, i)
		serviceMust(t, e)
		info, e := m.core.monomials.Info(mono)
		serviceMust(t, e)
		for j := uint32(0); j < info.Size; j++ {
			v, e := m.core.monomials.ReadPower(mono, j)
			serviceMust(t, e)
			for k := uint32(0); k < v.Degree; k++ {
				term *= values[v.Var]
			}
		}
		sum += term
	}
	return sum
}
func TestPolynomialCompositionIndependentFiniteAndAliases(t *testing.T) {
	for _, prime := range []uint64{0, 2, 7} {
		t.Run(fmt.Sprint(prime), func(t *testing.T) {
			m, _, _ := polynomialEvalFixture(t, []polynomialEvalTerm{{1, [3]uint32{}}})
			if prime != 0 {
				serviceMust(t, m.core.modular.SetZPUint64(prime))
			}
			p := polynomialCompositionTerms(t, m, []polynomialEvalTerm{{3, [3]uint32{3, 0, 0}}, {-3, [3]uint32{1, 0, 0}}, {5, [3]uint32{}}})
			q := polynomialCompositionTerms(t, m, []polynomialEvalTerm{{1, [3]uint32{1, 0, 0}}, {1, [3]uint32{0, 1, 0}}})
			for op := 0; op < 7; op++ {
				r, e := NewPolynomialRef(m.core)
				serviceMust(t, e)
				serviceMust(t, r.Assign(p))
				var raw Polynomial
				c := unumeral.Int{}
				serviceMust(t, m.core.modular.SetInt32(&c, 3))
				switch op {
				case 0:
					raw, e = m.ComposeReciprocal(p)
				case 1:
					raw, e = m.ComposeXDivY(p, 1)
				case 2:
					raw, e = m.ComposeVariable(p, 1)
				case 3:
					raw, e = m.ComposeMinusX(p)
				case 4:
					e = m.ComposeXMinusY(p, 1, r)
				case 5:
					e = m.ComposeXPlusY(p, 1, r)
				case 6:
					e = m.Compose(p, q, r)
				}
				serviceMust(t, e)
				if raw != (Polynomial{}) {
					serviceMust(t, r.Assign(raw))
				}
				for x := int64(-2); x <= 2; x++ {
					for y := int64(-2); y <= 2; y++ {
						v := x
						switch op {
						case 2:
							v = y
						case 3:
							v = -x
						case 4:
							v = x - y
						case 5, 6:
							v = x + y
						}
						want := 3*v*v*v - 3*v + 5
						if op == 0 {
							want = 5*x*x*x - 3*x*x + 3
						}
						if op == 1 {
							want = 3*x*x*x - 3*x*y*y + 5*y*y*y
						}
						got := polynomialCompositionFiniteValue(t, m, r.root.value, [3]int64{x, y, 0})
						if prime != 0 {
							got %= int64(prime)
							want %= int64(prime)
							if got < 0 {
								got += int64(prime)
							}
							if want < 0 {
								want += int64(prime)
							}
						}
						if got != want {
							t.Fatal("independent finite composition", op, x, y, got, want)
						}
					}
				}
				serviceMust(t, m.CleanupComposition())
				serviceMust(t, r.Close())
				serviceMust(t, m.core.integer.Del(&c))
			}
		})
	}
}
