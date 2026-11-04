//go:build g3trace

package interval

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestRationalIntervalSourceArithmeticTrace(t *testing.T) {
	budget := arena.NewBudget(1 << 24)
	m, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	lo, hi := numeral.NewRational(), numeral.NewRational()
	a, err := NewRationalInterval(m)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewRationalInterval(m)
	if err != nil {
		t.Fatal(err)
	}
	out, err := NewRationalInterval(m)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, e := range []error{out.Close(), b.Close(), a.Close(), m.Del(&hi), m.Del(&lo), m.CloseChecked()} {
			if e != nil {
				t.Error(e)
			}
		}
		if budget.Used() != 0 {
			t.Errorf("rational interval trace leaked %d bytes", budget.Used())
		}
	}()
	must := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	must(m.SetInt32(&lo, -3))
	must(m.SetInt32(&hi, -1))
	must(a.SetFinite(&lo, &hi, false, false))
	must(m.SetInt32(&lo, -4))
	must(m.SetInt32(&hi, -2))
	must(b.SetFinite(&lo, &hi, false, false))
	var stream bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&stream)
	tags.Enable("interval_bug")
	out.TraceTags = tags
	intervalBugMulCallID = 0
	must(out.Mul(a, b))
	got := stream.String()
	for _, want := range []string{
		"-------- [interval_bug] mul src/math/interval/interval_def.h:1044 ---------\n",
		"(N, N) #0\n[-3, -1]\n[-4, -2]\na: -3\nb: -1\nc: -4\nd: -2\nis_N0(i1): false\nis_N0(i2): false\n",
		"-------- [interval_bug] mul src/math/interval/interval_def.h:1210 ---------\nresult: [2, 12]\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("multiplication source trace missing %q in %q", want, got)
		}
	}
	stream.Reset()
	intervalBugPowerCallID = 0
	must(m.SetInt32(&lo, -2))
	must(m.SetInt32(&hi, 2))
	must(a.SetFinite(&lo, &hi, false, false))
	must(out.Power(a, 2))
	if !strings.Contains(stream.String(), "-------- [interval_bug] power src/math/interval/interval_def.h:1331 ---------\n(M) #0\n[-2, 2]\n") ||
		!strings.Contains(stream.String(), "-------- [interval_bug] power src/math/interval/interval_def.h:1378 ---------\nresult: [0, 4]\n") {
		t.Fatalf("power source trace: %q", stream.String())
	}
	stream.Reset()
	intervalBugInvCallID = 0
	must(m.SetInt32(&lo, 1))
	must(m.SetInt32(&hi, 2))
	must(a.SetFinite(&lo, &hi, false, false))
	must(out.Inv(a))
	if !strings.Contains(stream.String(), "-------- [interval_bug] inv src/math/interval/interval_def.h:1522 ---------\n(inv) #0\n[1, 2]\n") ||
		!strings.Contains(stream.String(), "-------- [interval_bug] inv src/math/interval/interval_def.h:1589 ---------\nresult: [1/2, 1]\n") {
		t.Fatalf("inverse source trace: %q", stream.String())
	}
	stream.Reset()
	intervalBugDivCallID = 0
	must(m.SetInt32(&lo, -4))
	must(m.SetInt32(&hi, -2))
	must(b.SetFinite(&lo, &hi, false, false))
	must(out.Div(a, b))
	for _, want := range []string{
		"-------- [interval_bug] div src/math/interval/interval_def.h:1696 ---------\ndiv #0\n[1, 2]\n[-4, -2]\na: 1\nb: 2\nc: -4\nd: -2\n",
		"-------- [interval_bug] div src/math/interval/interval_def.h:1798 ---------\n(P, N) #0\n",
		"-------- [interval_bug] div src/math/interval/interval_def.h:1843 ---------\nresult: [-1, -1/4]\n",
	} {
		if !strings.Contains(stream.String(), want) {
			t.Fatalf("division source trace missing %q in %q", want, stream.String())
		}
	}
}
