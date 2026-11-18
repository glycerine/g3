package numeral

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestFixedPointRetainedDivisionCallersTrace(t *testing.T) {
	m, err := NewFixedPointManager(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	n, err := m.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := n.Close(); err != nil {
			t.Error(err)
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
	check := func(name, want string) {
		t.Helper()
		if !tags.ShouldTrace("mpfx_trace") {
			want = ""
		}
		if got := output.String(); got != want {
			t.Fatalf("%s trace = %q, want %q", name, got, want)
		}
		output.Reset()
	}
	if err := m.SetRatio(n, -3, 2); err != nil {
		t.Fatal(err)
	}
	check("disabled ratio", "")
	tags.Enable("mpfx_trace")
	if err := m.SetRatio(n, -3, 2); err != nil {
		t.Fatal(err)
	}
	check("ratio down", "[mpfx] (-3) / (2) >= -3/2\n")
	if err := m.Inv(n); err != nil {
		t.Fatal(err)
	}
	check("inverse aliases divisor", "[mpfx] (1) / (-3/2) >= -2863311531/2^32\n")
	if err := m.SetRounding(true); err != nil {
		t.Fatal(err)
	}
	if err := m.SetRatio(n, 1, 3); err != nil {
		t.Fatal(err)
	}
	check("ratio up", "[mpfx] (1) / (3) <= 715827883/2^31\n")
	if err := m.Reset(n); err != nil {
		t.Fatal(err)
	}
	if err := m.Inv(n); !errors.Is(err, ErrFixedPointDivZero) {
		t.Fatalf("zero inverse = %v", err)
	}
	check("zero inverse", "")
	if err := m.SetRatio(n, 1, 0); !errors.Is(err, ErrFixedPointDivZero) {
		t.Fatalf("zero denominator = %v", err)
	}
	check("zero denominator", "")
	if err := m.SetRatio(n, 1<<32, 2); !errors.Is(err, ErrFixedPointOverflow) {
		t.Fatalf("numerator construction overflow = %v", err)
	}
	check("operand construction overflow", "")
	m.allocateIfNeeded(n) // the smallest positive fixed-point value
	m.setEpsilon(n)
	n.sign = false
	if err := m.Inv(n); !errors.Is(err, ErrFixedPointOverflow) {
		t.Fatalf("inverse overflow = %v", err)
	}
	check("inverse overflow leaves prefix", "[mpfx] (1) / (1/2^32) <= ")
}
