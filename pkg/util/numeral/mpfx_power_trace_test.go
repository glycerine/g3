package numeral

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestFixedPointPowerSourceTraceAndRawWords(t *testing.T) {
	m, err := NewFixedPointManager(2, 1)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := m.New()
	out, _ := m.New()
	t.Cleanup(func() {
		for _, n := range []*FixedPoint{a, out} {
			if err := n.Close(); err != nil {
				t.Error(err)
			}
		}
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	})
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	if err := m.BindTrace(tags); err != nil {
		t.Fatal(err)
	}
	check := func(want string) {
		t.Helper()
		if !tags.ShouldTrace("mpfx_trace") && !tags.ShouldTrace("mpfx_power") {
			want = ""
		}
		if got := output.String(); got != want {
			t.Fatalf("trace = %q, want %q", got, want)
		}
		output.Reset()
	}
	if err := m.SetRatio(a, -3, 2); err != nil {
		t.Fatal(err)
	}
	if err := m.Power(a, 1, out); err != nil {
		t.Fatal(err)
	}
	check("")
	tags.Enable("mpfx_power")
	if err := m.Power(a, 1, out); err != nil {
		t.Fatal(err)
	}
	check("-------- [mpfx_power] power src/util/mpfx.cpp:648 ---------\n-0000000000000001.80000000\n------------------------------------------------\n")
	tags.Disable("mpfx_power")
	tags.Enable("mpfx_trace")
	if err := m.Power(a, 1, a); err != nil {
		t.Fatal(err)
	}
	check("[mpfx] (-3/2) ^ 1>=-3/2\n")
	if err := m.SetRounding(true); err != nil {
		t.Fatal(err)
	}
	if err := m.Power(a, 0, out); err != nil {
		t.Fatal(err)
	}
	check("[mpfx] (-3/2) ^ 0<=1\n")
	if err := m.Power(a, 2, out); err != nil {
		t.Fatal(err)
	}
	check("[mpfx] (-3/2) * (-3/2) <= 9/2^2\n[mpfx] (-3/2) ^ 2<=9/2^2\n")
	if err := m.Power(a, 2, a); err != nil {
		t.Fatal(err)
	}
	check("[mpfx] (-3/2) * (-3/2) <= 9/2^2\n[mpfx] (-3/2) ^ 2<=9/2^2\n")
	if err := m.Reset(a); err != nil {
		t.Fatal(err)
	}
	// z3's release build skips SASSERT(p != 0) and resets b: 0^0 is 0.
	if err := m.Power(a, 0, out); err != nil {
		t.Fatalf("zero to the zero = %v", err)
	}
	check("[mpfx] (0) ^ 0<=0\n")
}
