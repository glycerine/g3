package numeral

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestArbitraryFloatNumeralSourcePredicatesAndOrder(t *testing.T) {
	core, _, _ := mpfNumeralHelpersFixture(t)
	m, err := NewArbitraryFloatNumeralManager(core, 11, 53)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	}()
	var negative, zero, positive ArbitraryFloat
	defer core.Del(&negative)
	defer core.Del(&zero)
	defer core.Del(&positive)
	for _, c := range []struct {
		out   *ArbitraryFloat
		value int32
	}{{&negative, -1}, {&positive, 1}} {
		if err := m.SetInt32(c.out, c.value); err != nil {
			t.Fatal(err)
		}
	}
	// f2n rejects zero from set as non-regular; classification still delegates
	// to the core for values produced by other float operations.
	if err := core.SetInt32(&zero, 11, 53, 0); err != nil {
		t.Fatal(err)
	}
	check := func(name string, want bool, query func() (bool, error)) {
		t.Helper()
		got, err := query()
		if err != nil || got != want {
			t.Fatalf("%s: got %v, %v; want %v", name, got, err, want)
		}
	}
	check("negative", true, func() (bool, error) { return m.IsNeg(&negative) })
	check("positive", true, func() (bool, error) { return m.IsPos(&positive) })
	check("zero", true, func() (bool, error) { return m.IsZero(&zero) })
	check("zero nonnegative", true, func() (bool, error) { return m.IsNonNeg(&zero) })
	check("zero nonpositive", true, func() (bool, error) { return m.IsNonPos(&zero) })
	check("minus one", true, func() (bool, error) { return m.IsMinusOne(&negative) })
	check("minus one restored", true, func() (bool, error) { return m.IsNeg(&negative) })
	check("one", true, func() (bool, error) { return m.IsOne(&positive) })
	check("integer", true, func() (bool, error) { return m.IsInteger(&negative) })
	check("less", true, func() (bool, error) { return m.Less(&negative, &zero) })
	check("less equal", true, func() (bool, error) { return m.LessEqual(&negative, &zero) })
	check("greater", true, func() (bool, error) { return m.Greater(&positive, &zero) })
	check("greater equal", true, func() (bool, error) { return m.GreaterEqual(&positive, &zero) })
	check("equal", true, func() (bool, error) { return m.Equal(&zero, &zero) })
	if err := m.Abs(&negative, &positive); err != nil {
		t.Fatal(err)
	}
	check("absolute copy", true, func() (bool, error) { return m.IsOne(&positive) })
	if err := m.Neg(&positive, &positive); err != nil {
		t.Fatal(err)
	}
	check("negative in place", true, func() (bool, error) { return m.IsMinusOne(&positive) })
}

func TestArbitraryFloatNumeralSourceFloorCeilAndClose(t *testing.T) {
	core, _, _ := mpfNumeralHelpersFixture(t)
	m, err := NewArbitraryFloatNumeralManager(core, 11, 53)
	if err != nil {
		t.Fatal(err)
	}
	var x, out ArbitraryFloat
	for _, c := range []struct {
		numerator   int32
		floor, ceil float64
	}{{-11, -4, -3}, {11, 3, 4}} {
		if err := m.SetFractionSigned(&x, c.numerator, 3); err != nil {
			t.Fatal(err)
		}
		if err := m.Floor(&x, &out); err != nil {
			t.Fatal(err)
		}
		got, err := core.ToDouble(&out)
		if err != nil || got != c.floor {
			t.Fatalf("floor %d: %v, %v", c.numerator, got, err)
		}
		if err := m.Ceil(&x, &out); err != nil {
			t.Fatal(err)
		}
		got, err = core.ToDouble(&out)
		if err != nil || got != c.ceil {
			t.Fatalf("ceil %d: %v, %v", c.numerator, got, err)
		}
		if err := m.Floor(&x, &x); err != nil {
			t.Fatal(err)
		}
		got, err = core.ToDouble(&x)
		if err != nil || got != c.floor {
			t.Fatalf("alias floor %d: %v, %v", c.numerator, got, err)
		}
	}
	if err := core.Del(&x); err != nil {
		t.Fatal(err)
	}
	if err := core.Del(&out); err != nil {
		t.Fatal(err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if err := m.Floor(&x, &out); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("closed floor", err)
	}
}

// AUDIT.md util_numeral_02-F1: f2n::floor/ceil only SASSERT regularity, so a
// result that rounds to (signed) zero is returned rather than refused.
func TestArbitraryFloatNumeralFloorCeilToZero(t *testing.T) {
	core, _, _ := mpfNumeralHelpersFixture(t)
	m, err := NewArbitraryFloatNumeralManager(core, 11, 53)
	if err != nil {
		t.Fatal(err)
	}
	var x, out ArbitraryFloat
	if err := m.SetFractionSigned(&x, 1, 2); err != nil {
		t.Fatal(err)
	}
	if err := m.Floor(&x, &out); err != nil {
		t.Fatal(err)
	}
	got, err := core.ToDouble(&out)
	if err != nil || got != 0 || math.Signbit(got) {
		t.Fatalf("floor 1/2: %v, %v", got, err)
	}
	if err := m.SetFractionSigned(&x, -1, 2); err != nil {
		t.Fatal(err)
	}
	if err := m.Ceil(&x, &out); err != nil {
		t.Fatal(err)
	}
	got, err = core.ToDouble(&out)
	if err != nil || got != 0 || !math.Signbit(got) {
		t.Fatalf("ceil -1/2: %v, %v", got, err)
	}
	if err := core.Del(&x); err != nil {
		t.Fatal(err)
	}
	if err := core.Del(&out); err != nil {
		t.Fatal(err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
}
