package subpaving

import (
	"bytes"
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSubpavingFixedIntervalDivSignBranches(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	m, err := numeral.NewFixedPointManager(2, 1)
	must(err)
	lo, err := m.New()
	must(err)
	hi, err := m.New()
	must(err)
	a, err := NewFixedInterval(m)
	must(err)
	b, err := NewFixedInterval(m)
	must(err)
	c, err := NewFixedInterval(m)
	must(err)
	defer func() {
		must(a.Close())
		must(b.Close())
		must(c.Close())
		must(lo.Close())
		must(hi.Close())
		must(m.Close())
	}()
	set := func(v *FixedInterval, l, u int64, lOpen, uOpen bool) {
		t.Helper()
		must(m.SetInt64(lo, l))
		must(m.SetInt64(hi, u))
		must(v.SetFinite(lo, hi, lOpen, uOpen))
	}
	show := func(v *FixedInterval) string {
		t.Helper()
		var out bytes.Buffer
		must(v.Display(&out))
		return out.String()
	}
	for _, tc := range []struct {
		name           string
		al, au, bl, bu int64
		want           string
	}{
		{"NN", -4, -2, -2, -1, "[1, 4]"},
		{"NP", -4, -2, 1, 2, "[-4, -1]"},
		{"MN", -2, 2, -2, -1, "[-2, 2]"},
		{"MP", -2, 2, 1, 2, "[-2, 2]"},
		{"PN", 2, 4, -2, -1, "[-4, -1]"},
		{"PP", 2, 4, 1, 2, "[1, 4]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			set(a, tc.al, tc.au, false, false)
			set(b, tc.bl, tc.bu, false, false)
			must(c.Div(a, b))
			if got := show(c); got != tc.want {
				t.Fatalf("quotient = %q, want %q", got, tc.want)
			}
			must(c.CheckInvariant())
		})
	}
	set(a, 1, 2, false, false)
	set(b, 0, 2, true, false)
	must(c.Div(a, b))
	if got := show(c); got != "[1/2, +oo)" {
		t.Fatalf("positive open-zero divisor = %q", got)
	}
	set(a, -2, -1, false, false)
	must(c.Div(a, b))
	if got := show(c); got != "(-oo, -1/2]" {
		t.Fatalf("negative open-zero divisor = %q", got)
	}
	set(a, -1, 1, false, false)
	must(c.Div(a, b))
	if got := show(c); got != "(-oo, +oo)" {
		t.Fatalf("mixed open-zero divisor = %q", got)
	}
	set(b, -2, 0, false, true)
	set(a, 1, 2, false, false)
	must(c.Div(a, b))
	if got := show(c); got != "(-oo, -1/2]" {
		t.Fatalf("positive over negative open-zero divisor = %q", got)
	}
	set(a, -2, -1, false, false)
	must(c.Div(a, b))
	if got := show(c); got != "[1/2, +oo)" {
		t.Fatalf("negative over negative open-zero divisor = %q", got)
	}
	set(a, -1, 1, false, false)
	must(c.Div(a, b))
	if got := show(c); got != "(-oo, +oo)" {
		t.Fatalf("mixed over negative open-zero divisor = %q", got)
	}
	set(a, 0, 0, false, false)
	must(c.Div(a, b))
	if got := show(c); got != "[0, 0]" {
		t.Fatalf("zero numerator = %q", got)
	}
	// math_subpaving_01-F1: the source only SASSERTs a zero-free denominator.
	// A closed zero lower endpoint takes the P1 branches, where the raw
	// m().is_zero(c) test turns the c-quotient into an infinity. The
	// infinite endpoint keeps the closed flag b_o || c_o, as in the source.
	set(a, 1, 8, false, false)
	set(b, 0, 4, false, false)
	must(c.Div(a, b))
	if got := show(c); got != "[1/2^2, +oo]" {
		t.Fatalf("positive over closed-zero divisor = %q", got)
	}
	set(a, -8, -1, false, false)
	must(c.Div(a, b))
	if got := show(c); got != "[-oo, -1/2^2]" {
		t.Fatalf("negative over closed-zero divisor = %q", got)
	}
	// A straddling denominator with nonzero endpoints yields an inverted
	// result in the source; the Go invariant check reports it.
	set(a, 1, 2, false, false)
	set(b, -1, 1, false, false)
	if err := c.Div(a, b); !errors.Is(err, interval.ErrExtendedDomain) {
		t.Fatalf("zero-containing denominator error = %v", err)
	}
	if err := a.Div(a, b); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("numerator alias error = %v", err)
	}
	set(a, 2, 4, false, false)
	set(b, 1, 2, false, false)
	must(b.Div(a, b))
	if got := show(b); got != "[1, 4]" {
		t.Fatalf("denominator-alias quotient = %q", got)
	}
	set(a, 1, 2, false, false)
	set(b, 1, 2, false, false)
	must(b.ResetUpper())
	must(c.Div(a, b))
	if got := show(c); got != "(0, 2]" {
		t.Fatalf("infinite denominator quotient = %q", got)
	}
	set(a, 1, 2, false, false)
	set(b, 3, 5, false, false)
	must(c.Div(a, b))
	resultLower, err := m.Rational(c.lower)
	must(err)
	resultUpper, err := m.Rational(c.upper)
	must(err)
	if resultLower.Cmp(big.NewRat(1, 5)) > 0 || resultUpper.Cmp(big.NewRat(2, 3)) < 0 {
		t.Fatalf("directed quotient [%s, %s] excludes [1/5, 2/3]", resultLower, resultUpper)
	}
}
