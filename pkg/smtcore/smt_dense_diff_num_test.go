package smtcore

import (
	"math"
	"math/big"
	"testing"
)

// smtDenseNum must agree with exact smtInfRational arithmetic, including
// past the int64 range and for non-integer values.
func TestSMTDenseNumMatchesExactArithmetic(t *testing.T) {
	values := []smtInfRational{
		newSMTInfRational(big.NewRat(0, 1), nil),
		newSMTInfRational(big.NewRat(5, 1), big.NewRat(-1, 1)),
		newSMTInfRational(big.NewRat(-7, 1), big.NewRat(2, 1)),
		newSMTInfRational(new(big.Rat).SetInt64(math.MaxInt64), nil),
		newSMTInfRational(new(big.Rat).SetInt64(math.MinInt64), big.NewRat(1, 1)),
		newSMTInfRational(big.NewRat(1, 3), nil),
		newSMTInfRational(big.NewRat(2, 1), new(big.Rat).SetInt64(math.MinInt64)),
	}
	for i := range values {
		a := smtDenseNumOf(&values[i])
		if got := a.inf(); got.Cmp(&values[i]) != 0 {
			t.Fatal("round trip", i, smtDenseInfText(&got), smtDenseInfText(&values[i]))
		}
		var neg smtInfRational
		neg.Neg(&values[i])
		if got := smtDenseNumNeg(a).inf(); got.Cmp(&neg) != 0 {
			t.Fatal("negation", i, smtDenseInfText(&got))
		}
		for j := range values {
			b := smtDenseNumOf(&values[j])
			var sum smtInfRational
			sum.Add(&values[i], &values[j])
			if got := smtDenseNumAdd(a, b).inf(); got.Cmp(&sum) != 0 {
				t.Fatal("addition", i, j, smtDenseInfText(&got), smtDenseInfText(&sum))
			}
			if got, want := smtDenseNumCmp(a, b), values[i].Cmp(&values[j]); got != want {
				t.Fatal("comparison", i, j, got, want)
			}
		}
	}
	// A sum past the int64 range is exact, and returns to int64 when it fits.
	big1 := smtDenseNumAdd(smtDenseNum{r: math.MaxInt64}, smtDenseNum{r: 1})
	if big1.x == nil {
		t.Fatal("overflow stayed in int64")
	}
	if back := smtDenseNumAdd(big1, smtDenseNum{r: -1}); back.x != nil || back.r != math.MaxInt64 {
		t.Fatal("value that fits was not narrowed", back)
	}
}
