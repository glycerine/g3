package simplex

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func infiniteTestRat(text string) *big.Rat {
	q, ok := new(big.Rat).SetString(text)
	if !ok {
		panic(text)
	}
	return q
}
func infiniteTestValue(infinity, main, epsilon string) *Infinitesimal {
	return testNII(ratText(infinity), ratText(main), ratText(epsilon))
}
func checkInfiniteTestValue(t *testing.T, actual *Infinitesimal, infinity, main, epsilon string) {
	t.Helper()
	expected := infiniteTestValue(infinity, main, epsilon)
	if icmp(actual, expected) != 0 {
		t.Fatal(itext(actual), "want", itext(expected))
	}
}
func mustOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func TestInfiniteValueLexicographicOrderAndOwnedCopies(t *testing.T) {
	q := optQ()
	huge := new(big.Int).Exp(big.NewInt(10), big.NewInt(200), nil)
	finite := testNI(bigRat(new(big.Rat).SetInt(huge)), rat(999, 1))
	a := infiniteTestValue("1/3", "-999999", "-7/9")
	sign, err := a.Sign(q)
	mustOK(t, err)
	rational, err := a.IsRational(q)
	mustOK(t, err)
	if icmp(a, finite) <= 0 || sign != 1 || rational {
		t.Fatal("infinity did not dominate finite coordinates")
	}
	b := iclone(a)
	mustOK(t, q.SetInt64(&b.Main, 3))
	mustOK(t, q.SetInt64(&b.Epsilon, 0))
	if icmp(a, b) >= 0 {
		t.Fatal("equal-infinity rational order lost")
	}
	mustOK(t, q.Copy(&b.Main, &a.Main))
	mustOK(t, q.SetInt64(&b.Epsilon, 0))
	if icmp(a, b) >= 0 {
		t.Fatal("equal-infinity epsilon order lost")
	}
	copied := iclone(a)
	mustOK(t, q.SetInt64(&copied.Infinity, 99))
	checkInfiniteTestValue(t, a, "1/3", "-999999", "-7/9")
	assigned := ZeroInfinitesimal()
	mustOK(t, assigned.Set(q, a))
	mustOK(t, q.SetInt64(&assigned.Main, 0))
	checkInfiniteTestValue(t, a, "1/3", "-999999", "-7/9")
	zero := iclone(a)
	mustOK(t, zero.Sub(q, zero))
	isZero, err := zero.IsZero(q)
	mustOK(t, err)
	rational, err = zero.IsRational(q)
	mustOK(t, err)
	if !isZero || !rational {
		t.Fatal("self cancellation lost a coordinate")
	}
	for _, v := range []*Infinitesimal{finite, a, b, copied, &assigned, zero} {
		mustOK(t, v.Del(q))
	}
}
func TestInfiniteValueCoordinateArithmeticOracle(t *testing.T) {
	q := optQ()
	inputs := [][3]string{{"2/3", "-7/2", "5/11"}, {"-9/4", "17/3", "-2/5"}, {"0", "0", "-1"}}
	for _, input := range inputs {
		a := infiniteTestValue(input[0], input[1], input[2])
		b := infiniteTestValue("-3/7", "11/13", "-2/17")
		sum := iclone(a)
		mustOK(t, sum.Add(q, b))
		for j, part := range []*numeral.Rational{&sum.Infinity, &sum.Main, &sum.Epsilon} {
			want := new(big.Rat).Add(infiniteTestRat(input[j]), []*big.Rat{big.NewRat(-3, 7), big.NewRat(11, 13), big.NewRat(-2, 17)}[j])
			if ratBig(part).Cmp(want) != 0 {
				t.Fatal("coordinate addition", j, ratBig(part), want)
			}
		}
		mustOK(t, sum.Sub(q, b))
		if icmp(sum, a) != 0 {
			t.Fatal("subtraction did not undo addition")
		}
		multiplier := rat(-5, 7)
		scaled := iclone(a)
		mustOK(t, scaled.Mul(q, multiplier))
		for j, part := range []*numeral.Rational{&scaled.Infinity, &scaled.Main, &scaled.Epsilon} {
			want := new(big.Rat).Mul(infiniteTestRat(input[j]), big.NewRat(-5, 7))
			if ratBig(part).Cmp(want) != 0 {
				t.Fatal("coordinate product", j, ratBig(part), want)
			}
		}
		mustOK(t, scaled.Quo(q, multiplier))
		if icmp(scaled, a) != 0 {
			t.Fatal("division did not undo product")
		}
		mustOK(t, scaled.Neg(q))
		mustOK(t, scaled.Add(q, a))
		isZero, err := scaled.IsZero(q)
		mustOK(t, err)
		if !isZero {
			t.Fatal("negation lost a coordinate")
		}
	}
}

// The solver's eps_numeral is mpq_inf: a bound's infinity coefficient is
// not part of it, and values read back have infinity 0.
func TestSolverValuesAreMpqInf(t *testing.T) {
	s := newTestSolver(nil)
	defer func() { mustOK(t, s.Close()) }()
	seed := infiniteTestValue("4/7", "3", "-2")
	mustOK(t, s.SetValue(1, seed))
	_, err := s.AddInt64Row(2, []uint32{2, 1}, []int64{2, -3})
	mustOK(t, err)
	checkInfiniteTestValue(t, testValue(s, 1), "0", "3", "-2")
	checkInfiniteTestValue(t, testValue(s, 2), "0", "9/2", "-3")
}
