package polynomial

import (
	"errors"
	"testing"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Source: upolynomial_factorization.cpp:583-674. Validation and output
// coverage must run even when failure diagnostics are compiled out.
func TestUPolynomialHenselPairConditionalValidation(t *testing.T) {
	for _, correct := range []bool{false, true} {
		name := "invalid-bezout"
		if correct {
			name = "correct"
		}
		t.Run(name, func(t *testing.T) {
			polynomialThrowAssertionFixture(t)
			q, s := preparationPhysicalFixture(t)
			m := s.manager
			s.config.FactorizationDebugTag = true
			a := upolySourceNew(t, m, []int32{0, 1})
			b := upolySourceNew(t, m, []int32{1, 1})
			c := upolySourceNew(t, m, []int32{3, 1, 1})
			var uc, vc []int32
			if correct {
				// -x + (x+1) = 1, the required Bezout identity.
				uc, vc = []int32{-1}, []int32{1}
			}
			u, v := upolySourceNew(t, m, uc), upolySourceNew(t, m, vc)
			al, bl := upolySourceNew(t, m, nil), upolySourceNew(t, m, nil)
			one, three := unumeral.NewInt(1), unumeral.NewInt(3)
			body := s.HenselLiftPair(&one, &three, &three, mustUPolyView(t, m, u), mustUPolyView(t, m, a), mustUPolyView(t, m, v), mustUPolyView(t, m, b), mustUPolyView(t, m, c), al, bl)
			if !correct && polynomialSourceDebug {
				if !errors.Is(body, ErrUPolynomialAssertion) {
					t.Fatal("invalid Bezout pair did not reach final validation", body)
				}
			} else {
				serviceMust(t, body)
			}
			want := [2][2]int64{{0, 1}, {1, 1}}
			if correct {
				// (x+3)(x-2) = x^2+x-6 = C modulo 9.
				want = [2][2]int64{{3, 1}, {-2, 1}}
			}
			for i, p := range []UPolynomial{al, bl} {
				size, e := m.Size(p)
				serviceMust(t, e)
				if size != 2 {
					t.Fatal("lifted degree", i, size)
				}
				for j, expected := range want[i] {
					var coefficient unumeral.Int
					serviceMust(t, m.ReadCoefficient(p, uint32(j), &coefficient))
					actual, e := q.Integers().Int64(&coefficient)
					serviceMust(t, e)
					serviceMust(t, q.Integers().Del(&coefficient))
					if actual != expected {
						t.Fatal("lifted coefficient", i, j, actual, expected)
					}
				}
			}
			serviceMust(t, s.CleanupRetained())
		})
	}
}
