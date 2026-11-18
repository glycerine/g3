package numeral

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestFixedPointFloorCeilSourceTrace(t *testing.T) {
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
	set := func(num int64, den uint64) {
		t.Helper()
		if err := m.SetRatio(n, num, den); err != nil {
			t.Fatal(err)
		}
	}
	check := func(want string) {
		t.Helper()
		if !tags.ShouldTrace("mpfx_trace") {
			want = ""
		}
		if got := output.String(); got != want {
			t.Fatalf("trace = %q, want %q", got, want)
		}
		output.Reset()
	}
	set(-5, 2)
	if err := m.Floor(n); err != nil {
		t.Fatal(err)
	}
	check("")
	tags.Enable("mpfx_trace")
	set(-5, 2)
	if err := m.Floor(n); err != nil {
		t.Fatal(err)
	}
	check("[mpfx] (-5) / (2) >= -5/2\n[mpfx] Floor[-5/2] == -3\n")
	set(-5, 2)
	if err := m.Ceil(n); err != nil {
		t.Fatal(err)
	}
	check("[mpfx] (-5) / (2) >= -5/2\n[mpfx] Ceiling[-5/2] == -2\n")
	set(5, 2)
	if err := m.Floor(n); err != nil {
		t.Fatal(err)
	}
	check("[mpfx] (5) / (2) >= 5/2\n[mpfx] Floor[5/2] == 2\n")
	set(5, 2)
	if err := m.Ceil(n); err != nil {
		t.Fatal(err)
	}
	check("[mpfx] (5) / (2) >= 5/2\n[mpfx] Ceiling[5/2] == 3\n")
	if err := m.SetUint64(n, (1<<32)-1); err != nil {
		t.Fatal(err)
	}
	// An overflowing ceil writes only the source prefix.
	fixedWordsInc(n.words) // one unit in the last place
	before, err := m.String(n)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Ceil(n); !errors.Is(err, ErrFixedPointOverflow) {
		t.Fatalf("ceil overflow = %v", err)
	}
	check("[mpfx] Ceiling[" + before + "] == ")
}
