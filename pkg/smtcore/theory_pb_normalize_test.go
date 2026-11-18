package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func smtPBTestValue(l SMTLiteral, bits uint32) bool {
	if l == SMTTrueLiteral {
		return true
	}
	if l == SMTFalseLiteral {
		return false
	}
	v := bits&(1<<uint(l.Var()-1)) != 0
	if l.Sign() {
		return !v
	}
	return v
}

func smtPBTestHolds(a *smtPBArgs, bits uint32, equality bool) bool {
	q := testQ()
	sum := smtQZero()
	defer smtQDel(q, &sum)
	for i := range a.terms {
		if smtPBTestValue(a.terms[i].literal, bits) {
			if err := q.Add(&sum, &a.terms[i].coeff, &sum); err != nil {
				panic(err)
			}
		}
	}
	c, err := q.Compare(&sum, &a.bound)
	if err != nil {
		panic(err)
	}
	if equality {
		return c == 0
	}
	return c >= 0
}

func TestSMTPBLiteralNormalizationIndependentValuationOracle(t *testing.T) {
	a, b, c := NewSMTLiteral(1, false), NewSMTLiteral(2, false), NewSMTLiteral(3, false)
	for _, eq := range []bool{false, true} {
		for _, test := range []struct {
			literals     []SMTLiteral
			coefficients []string
		}{
			{[]SMTLiteral{a, b, c}, []string{"4", "3", "2"}},
			{[]SMTLiteral{a, a.Negated(), b, SMTTrueLiteral, SMTFalseLiteral, a, c}, []string{"2", "3", "-4", "1", "10", "0", "1"}},
			{[]SMTLiteral{a, b, c.Negated()}, []string{"1/2", "3/4", "-5/6"}},
			{[]SMTLiteral{a, a.Negated(), b}, []string{"2", "2", "0"}},
		} {
			for k := int64(-5); k <= 12; k++ {
				q := testQ()
				original := pbArgsOf(t, q, fmt.Sprintf("%d/2", k), test.literals, test.coefficients)
				normal, err := original.copy(q)
				astTestMust(t, err)
				astTestMust(t, normal.unique(q))
				truth, err := normal.normalize(q, eq)
				astTestMust(t, err)
				astTestMust(t, normal.prune(q, eq))
				for bits := uint32(0); bits < 8; bits++ {
					want := smtPBTestHolds(&original, bits, eq)
					got := smtPBTestHolds(&normal, bits, eq)
					if truth == lbool.True {
						got = true
					}
					if truth == lbool.False {
						got = false
					}
					if got != want {
						t.Fatalf("normalization changed valuation: eq=%v coeff=%v bound=%v bits=%d truth=%v", eq, test.coefficients, pbText(q, &original.bound), bits, truth)
					}
				}
				if !eq && truth == lbool.Undef && len(normal.terms) > 0 {
					complement, err := normal.copy(q)
					astTestMust(t, err)
					astTestMust(t, complement.negate(q))
					for bits := uint32(0); bits < 8; bits++ {
						if smtPBTestHolds(&complement, bits, false) == smtPBTestHolds(&original, bits, false) {
							t.Fatal("strict integer complement lost polarity")
						}
					}
				}
			}
		}
	}
}
