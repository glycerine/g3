package simplex

import (
	"math/big"
	"testing"
)

func infiniteTestRat(text string) *big.Rat {
	q, ok := new(big.Rat).SetString(text)
	if !ok {
		panic(text)
	}
	return q
}
func infiniteTestValue(infinity, main, epsilon string) Infinitesimal {
	return NewInfiniteInfinitesimal(infiniteTestRat(infinity), infiniteTestRat(main), infiniteTestRat(epsilon))
}
func checkInfiniteTestValue(t *testing.T, actual Infinitesimal, infinity, main, epsilon string) {
	t.Helper()
	expected := infiniteTestValue(infinity, main, epsilon)
	if actual.Cmp(&expected) != 0 {
		t.Fatal(actual.String(), "want", expected.String())
	}
}
func TestInfiniteValueLexicographicOrderAndOwnedCopies(t *testing.T) {
	huge := new(big.Int).Exp(big.NewInt(10), big.NewInt(200), nil)
	finite := NewInfinitesimal(new(big.Rat).SetInt(huge), big.NewRat(999, 1))
	a := infiniteTestValue("1/3", "-999999", "-7/9")
	if a.Cmp(&finite) <= 0 || a.Sign() != 1 || a.IsRational() {
		t.Fatal("infinity did not dominate finite coordinates")
	}
	b := a.Clone()
	b.Main.SetInt64(3)
	b.Epsilon.SetInt64(0)
	if a.Cmp(&b) >= 0 {
		t.Fatal("equal-infinity rational order lost")
	}
	b.Main.Set(&a.Main)
	b.Epsilon.SetInt64(0)
	if a.Cmp(&b) >= 0 {
		t.Fatal("equal-infinity epsilon order lost")
	}
	copy := a.Clone()
	copy.Infinity.SetInt64(99)
	checkInfiniteTestValue(t, a, "1/3", "-999999", "-7/9")
	var assigned Infinitesimal
	assigned.Set(&a)
	assigned.Main.SetInt64(0)
	checkInfiniteTestValue(t, a, "1/3", "-999999", "-7/9")
	zero := a.Clone()
	zero.Sub(&zero)
	if !zero.IsZero() || !zero.IsRational() {
		t.Fatal("self cancellation lost a coordinate")
	}
}
func TestInfiniteValueCoordinateArithmeticOracle(t *testing.T) {
	inputs := [][3]string{{"2/3", "-7/2", "5/11"}, {"-9/4", "17/3", "-2/5"}, {"0", "0", "-1"}}
	for _, input := range inputs {
		a := infiniteTestValue(input[0], input[1], input[2])
		b := infiniteTestValue("-3/7", "11/13", "-2/17")
		sum := a.Clone()
		sum.Add(&b)
		for j, part := range []*big.Rat{&sum.Infinity, &sum.Main, &sum.Epsilon} {
			want := new(big.Rat).Add(infiniteTestRat(input[j]), []*big.Rat{big.NewRat(-3, 7), big.NewRat(11, 13), big.NewRat(-2, 17)}[j])
			if part.Cmp(want) != 0 {
				t.Fatal("coordinate addition", j, part, want)
			}
		}
		sum.Sub(&b)
		if sum.Cmp(&a) != 0 {
			t.Fatal("subtraction did not undo addition")
		}
		multiplier := big.NewRat(-5, 7)
		scaled := a.Clone()
		scaled.Mul(multiplier)
		for j, part := range []*big.Rat{&scaled.Infinity, &scaled.Main, &scaled.Epsilon} {
			want := new(big.Rat).Mul(infiniteTestRat(input[j]), multiplier)
			if part.Cmp(want) != 0 {
				t.Fatal("coordinate product", j, part, want)
			}
		}
		scaled.Quo(multiplier)
		if scaled.Cmp(&a) != 0 {
			t.Fatal("division did not undo product")
		}
		scaled.Neg()
		scaled.Add(&a)
		if !scaled.IsZero() {
			t.Fatal("negation lost a coordinate")
		}
	}
}
func TestInfiniteTableauFeasibilityPivotAndMinimize(t *testing.T) {
	s := NewSolver(nil)
	// y=-2*x and objective=-x. y's lower bound is the tighter bound on x.
	_, err := s.AddIntegerRow(2, []uint32{2, 1}, []big.Int{*big.NewInt(1), *big.NewInt(2)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.AddIntegerRow(3, []uint32{3, 1}, []big.Int{*big.NewInt(1), *big.NewInt(1)})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetUpper(1, infiniteTestValue("3", "5", "-2")); err != nil {
		t.Fatal(err)
	}
	if err = s.SetLower(2, infiniteTestValue("-4", "-2", "6")); err != nil {
		t.Fatal(err)
	}
	status, err := s.MakeFeasible()
	if err != nil || status != 1 {
		t.Fatal(status, err)
	}
	status, err = s.Minimize(3)
	if err != nil || status != 1 {
		t.Fatal(status, err)
	}
	checkInfiniteTestValue(t, s.Value(1), "2", "1", "-3")
	checkInfiniteTestValue(t, s.Value(2), "-4", "-2", "6")
	checkInfiniteTestValue(t, s.Value(3), "-2", "-1", "3")
	if s.Stats.NumPivots == 0 {
		t.Fatal("did not exercise a rational pivot with infinite values")
	}
}
func TestInfiniteTableauBoundsAndBasicSeed(t *testing.T) {
	s := NewSolver(nil)
	if err := s.SetLower(1, infiniteTestValue("3", "-999", "0")); err != nil {
		t.Fatal(err)
	}
	if err := s.SetUpper(1, infiniteTestValue("2", "999", "0")); err == nil {
		t.Fatal("inconsistent infinity endpoints accepted")
	}
	s = NewSolver(nil)
	seed := infiniteTestValue("4/7", "3", "-2")
	s.SetValue(1, seed)
	_, err := s.AddIntegerRow(2, []uint32{2, 1}, []big.Int{*big.NewInt(2), *big.NewInt(-3)})
	if err != nil {
		t.Fatal(err)
	}
	expected := seed.Clone()
	expected.Mul(big.NewRat(3, 2))
	s.SetValue(2, infiniteTestValue("-5", "100", "7"))
	actual := s.Value(2)
	if actual.Cmp(&expected) != 0 {
		t.Fatal("basic seed broke canonical infinite row", actual.String(), expected.String())
	}
}
