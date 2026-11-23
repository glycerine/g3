package ast

import (
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func TestArithmeticSquareRootFraction(t *testing.T) {
	m := squareRootManager(t)
	am, err := m.ArithmeticAlgebraicManager()
	astTestMust(t, err)
	for _, tc := range []struct {
		numerator, denominator int32
		p, q                   int32 // reduced radicand
		rational               bool
	}{
		{0, 11, 0, 1, true}, {36, 16, 9, 4, true}, {18, 2, 9, 1, true},
		{2, 3, 2, 3, false}, {15, 6, 5, 2, false}, {12, 18, 2, 3, false},
		{1, math.MaxInt32, 1, math.MaxInt32, false},
		{math.MaxInt32, 2, math.MaxInt32, 2, false},
	} {
		for _, neg := range []bool{false, true} {
			h, err := m.ArithmeticSquareRootFraction(tc.numerator, tc.denominator, neg)
			astTestMust(t, err)
			astTestMust(t, m.IncRef(h))
			if !tc.rational && m.unboundedAlgebraicReads() {
				p, q, ok, err := m.ArithmeticBinomialPower(h, 2)
				astTestMust(t, err)
				if !ok || p != int64(tc.p) || q != int64(tc.q) {
					t.Fatalf("square of %d/%d root: %d/%d, found %v", tc.numerator, tc.denominator, p, q, ok)
				}
				format, ok, err := m.SMT2PPArithLiteral(h, false, 5)
				astTestMust(t, err)
				index := 2
				if neg {
					index = 1
				}
				want, err := m.FormatString(fmt.Sprintf("(root-obj (+ (* %d (^ x 2)) (- %d)) %d)", tc.q, tc.p, index))
				astTestMust(t, err)
				if !ok || format != want {
					t.Fatal("incorrect defining polynomial or root index")
				}
			}
			var actual, num, den, fraction, reference polynomial.Algebraic
			found, err := m.ArithmeticAlgebraicNumeralValue(h, &actual)
			astTestMust(t, err)
			if found == tc.rational {
				t.Fatalf("unexpected numeral kind for %d/%d", tc.p, tc.q)
			}
			if tc.rational {
				p, q := floorSquareRoot(tc.p), floorSquareRoot(tc.q)
				if neg {
					p = -p
				}
				want, err := m.ArithmeticRealNumeralText(fmt.Sprintf("%d/%d", p, q))
				astTestMust(t, err)
				if h != want {
					t.Fatalf("incorrect rational root for %d/%d", tc.p, tc.q)
				}
			} else {
				p, q := unumeral.NewInt(tc.p), unumeral.NewInt(tc.q)
				astTestMust(t, am.SetInteger(&num, &p))
				astTestMust(t, am.SetInteger(&den, &q))
				astTestMust(t, am.Div(&num, &den, &fraction))
				astTestMust(t, am.Root(&fraction, 2, &reference))
				if neg {
					astTestMust(t, am.Neg(&reference))
				}
				cmp, err := am.Compare(&actual, &reference)
				astTestMust(t, err)
				if cmp != 0 {
					t.Fatalf("incorrect root for %d/%d, negative %v", tc.p, tc.q, neg)
				}
			}
			for _, v := range []*polynomial.Algebraic{&actual, &reference, &fraction, &num, &den} {
				astTestMust(t, am.Del(v))
			}
			astTestMust(t, m.DecRef(h))
		}
	}
	h, err := m.ArithmeticSquareRootNumeral(7, false)
	astTestMust(t, err)
	astTestMust(t, m.IncRef(h))
	if m.unboundedAlgebraicReads() && m.arithmeticPlugin.algebraic.squareDenominator(0) != 1 {
		t.Fatal("recycled integer root inherited a denominator")
	}
	astTestMust(t, m.DecRef(h))
	for _, fraction := range [][2]int32{{-1, 1}, {1, 0}, {0, -1}} {
		if _, err := m.ArithmeticSquareRootFraction(fraction[0], fraction[1], false); !errors.Is(err, ErrASTDomain) {
			t.Fatalf("invalid fraction %v returned %v", fraction, err)
		}
	}
	var dst polynomial.Algebraic
	for _, row := range [][3]int32{{1, 4, 0}, {2, 4, 0}, {7, 5, math.MaxInt32}, {1, math.MaxInt32, math.MinInt32}} {
		if err := am.SetIsolatedSquareRootFraction(&dst, row[0], row[1], row[2]); !errors.Is(err, polynomial.ErrAlgebraicAssertion) {
			t.Fatalf("invalid isolation %v returned %v", row, err)
		}
	}
}
