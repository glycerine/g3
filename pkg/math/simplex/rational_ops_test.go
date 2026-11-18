package simplex

import (
	"math/big"
	"testing"
)

// The manager's Sign reports any value of the sign (an inline value's
// numerator); ratCalc.sign and Infinitesimal.Sign normalize it to -1/0/1,
// which the sign-equality tests of resolve, mul_add and pick_leaving need.
func TestRatCalcSignIsNormalized(t *testing.T) {
	c := ratCalc{q: optQ()}
	huge := new(big.Int).Lsh(big.NewInt(1), 100)
	for _, tc := range []struct {
		value *big.Rat
		want  int
	}{
		{big.NewRat(0, 1), 0}, {big.NewRat(2, 1), 1}, {big.NewRat(-7, 3), -1},
		{big.NewRat(1, 1), 1}, {big.NewRat(-1, 1), -1},
		{new(big.Rat).SetInt(huge), 1}, {new(big.Rat).SetInt(new(big.Int).Neg(huge)), -1},
	} {
		r := bigRat(tc.value)
		if got := c.sign(r); got != tc.want {
			t.Fatalf("sign(%s) = %d, want %d", tc.value.RatString(), got, tc.want)
		}
		v := testNI(r, nil)
		got, err := v.Sign(optQ())
		mustOK(t, err)
		if got != tc.want {
			t.Fatalf("Infinitesimal.Sign(%s) = %d, want %d", tc.value.RatString(), got, tc.want)
		}
		e := testNI(nil, r)
		if got, err = e.Sign(optQ()); err != nil || got != tc.want {
			t.Fatalf("Infinitesimal.Sign(%s*epsilon) = %d, want %d", tc.value.RatString(), got, tc.want)
		}
	}
	if c.sign(bigRat(big.NewRat(2, 1))) != c.sign(bigRat(big.NewRat(3, 1))) {
		t.Fatal("equal-sign values compare unequal")
	}
	mustOK(t, c.err)
}
