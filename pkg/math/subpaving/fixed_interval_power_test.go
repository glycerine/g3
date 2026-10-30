package subpaving

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSubpavingFixedIntervalPowerRetainedCases(t *testing.T) {
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
	defer func() {
		must(a.Close())
		must(b.Close())
		must(lo.Close())
		must(hi.Close())
		must(m.Close())
	}()
	set := func(l, u int64, lOpen, uOpen bool) {
		t.Helper()
		must(m.SetInt64(lo, l))
		must(m.SetInt64(hi, u))
		must(a.SetFinite(lo, hi, lOpen, uOpen))
	}
	show := func(v *FixedInterval) string {
		t.Helper()
		var out bytes.Buffer
		must(v.Display(&out))
		return out.String()
	}
	check := func(want string) {
		t.Helper()
		if got := show(b); got != want {
			t.Fatalf("power result = %q, want %q", got, want)
		}
		must(b.CheckInvariant())
	}
	set(1, 2, false, true)
	must(b.Power(a, 2))
	check("[1, 4)")
	set(-3, -1, true, false)
	must(b.Power(a, 2))
	check("[1, 9)")
	must(a.Power(a, 2))
	if got := show(a); got != "[1, 9)" {
		t.Fatalf("aliased negative power = %q", got)
	}
	set(-2, 2, true, false)
	must(b.Power(a, 2))
	check("[0, 4]")
	set(-2, 2, false, true)
	must(b.Power(a, 2))
	check("[0, 4]")
	set(-2, 2, true, true)
	must(b.Power(a, 2))
	check("[0, 4)")
	must(a.ResetLower())
	must(b.Power(a, 2))
	check("[0, +oo)")
	set(-3, -2, false, false)
	must(a.ResetLower())
	must(b.Power(a, 2))
	check("[4, +oo)")
	set(-2, 3, true, false)
	must(b.Power(a, 3))
	check("(-8, 27]")
	must(a.ResetUpper())
	must(b.Power(a, 3))
	check("(-8, +oo)")
	must(m.SetRounding(false))
	must(m.SetRatio(lo, 1, 3))
	must(m.SetRounding(true))
	must(m.SetRatio(hi, 2, 3))
	must(a.SetFinite(lo, hi, false, false))
	must(b.Power(a, 2))
	resultLower, err := m.Rational(b.lower)
	must(err)
	resultUpper, err := m.Rational(b.upper)
	must(err)
	if resultLower.Cmp(big.NewRat(1, 9)) > 0 || resultUpper.Cmp(big.NewRat(4, 9)) < 0 {
		t.Fatalf("directed squared bounds [%s, %s] exclude [1/9, 4/9]", resultLower, resultUpper)
	}
	must(b.Power(a, 0))
	check("[1, 1]")
}
