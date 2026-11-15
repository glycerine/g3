package subpaving

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSubpavingRationalIntervalDivSixSignAndOpenZeroBranches(t *testing.T) {
	budget := arena.NewBudget(1 << 24)
	m, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{
		Cells: arena.Config{Budget: budget},
		Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	a, err := NewRationalInterval(m)
	must(err)
	b, err := NewRationalInterval(m)
	must(err)
	out, err := NewRationalInterval(m)
	must(err)
	lo, hi := numeral.NewRational(), numeral.NewRational()
	t.Cleanup(func() {
		for _, x := range []*RationalInterval{out, b, a} {
			if err := x.Close(); err != nil {
				t.Error(err)
			}
		}
		for _, x := range []*numeral.Rational{&lo, &hi} {
			if err := m.Del(x); err != nil {
				t.Error(err)
			}
		}
		if err := m.CloseChecked(); err != nil {
			t.Error(err)
		}
		if budget.Used() != 0 {
			t.Error("rational interval quotient retained budget", budget.Used())
		}
	})
	set := func(v *RationalInterval, lower, upper string, lowerOpen, upperOpen bool) {
		t.Helper()
		must(m.SetString(&lo, lower))
		must(m.SetString(&hi, upper))
		must(v.SetFinite(&lo, &hi, lowerOpen, upperOpen))
	}
	show := func(v *RationalInterval, want string) {
		t.Helper()
		var rendered bytes.Buffer
		must(v.Display(&rendered))
		if rendered.String() != want {
			t.Fatalf("interval = %q, want %q", rendered.String(), want)
		}
		must(v.CheckInvariant())
	}
	inputs := map[byte][2]string{'N': {"-4", "-2"}, 'M': {"-2", "4"}, 'P': {"2", "4"}}
	for _, tc := range []struct{ pair, want string }{
		{"NN", "[1/2, 2]"}, {"NP", "[-2, -1/2]"},
		{"MN", "[-2, 1]"}, {"MP", "[-1, 2]"},
		{"PN", "[-2, -1/2]"}, {"PP", "[1/2, 2]"},
	} {
		t.Run(tc.pair, func(t *testing.T) {
			ab, bb := inputs[tc.pair[0]], inputs[tc.pair[1]]
			set(a, ab[0], ab[1], false, false)
			set(b, bb[0], bb[1], false, false)
			must(out.Div(a, b))
			show(out, tc.want)
		})
	}
	set(a, "2", "4", false, false)
	set(b, "2", "4", false, false)
	must(out.SetFrom(b))
	must(out.Div(a, out))
	show(out, "[1/2, 2]")
	if err := a.Div(a, b); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("numerator alias accepted", err)
	}
	set(a, "0", "0", false, false)
	must(out.Div(a, b))
	show(out, "[0, 0]")
	set(a, "2", "4", false, false)
	set(b, "0", "2", true, false)
	must(out.Div(a, b))
	show(out, "[1, +oo)")
	set(a, "-4", "-2", false, false)
	must(out.Div(a, b))
	show(out, "(-oo, -1]")
	set(a, "-2", "4", false, false)
	must(out.Div(a, b))
	show(out, "(-oo, +oo)")
	set(a, "-4", "-2", false, false)
	set(b, "-2", "0", false, true)
	must(out.Div(a, b))
	show(out, "[1, +oo)")
	set(a, "2", "4", false, false)
	must(out.Div(a, b))
	show(out, "(-oo, -1]")
	set(a, "-2", "4", false, false)
	must(out.Div(a, b))
	show(out, "(-oo, +oo)")
	set(a, "2", "4", false, false)
	set(b, "2", "4", false, false)
	must(b.ResetUpper())
	must(out.Div(a, b))
	show(out, "(0, 2]")
	// math_subpaving_01-F1: the source only SASSERTs a zero-free denominator.
	// A closed zero lower endpoint takes the P1 branches, where the raw
	// m().is_zero(c) test turns the c-quotient into an infinity. The
	// infinite endpoint keeps the closed flag b_o || c_o, as in the source.
	set(b, "1", "8", false, false)
	set(a, "0", "4", false, false)
	must(out.Div(b, a))
	show(out, "[1/4, +oo]")
	set(b, "-8", "-1", false, false)
	must(out.Div(b, a))
	show(out, "[-oo, -1/4]")
	set(b, "2", "4", false, false)
	must(b.ResetUpper())
	// A straddling denominator with nonzero endpoints yields an inverted
	// result in the source; the Go invariant check reports it.
	set(a, "-1", "1", false, false)
	if err := out.Div(b, a); !errors.Is(err, interval.ErrExtendedDomain) {
		t.Fatal("zero-containing divisor accepted", err)
	}
}
