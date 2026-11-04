package interp

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestInterpolationProfilerTotalsSortedAndCombined(t *testing.T) {
	p := NewProfiler()
	base := time.Unix(100, 0)
	tick := 0
	p.now = func() time.Time {
		t := base.Add(time.Duration(tick) * time.Second)
		tick++
		return t
	}
	for _, op := range []struct {
		start bool
		name  string
	}{
		{true, "zeta"}, {true, "shared"}, {false, "shared"}, {false, "zeta"},
		{true, "alpha"}, {true, "shared"}, {false, "shared"}, {false, "alpha"},
	} {
		var err error
		if op.start {
			err = p.TimerStart(op.name)
		} else {
			err = p.TimerStop(op.name)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := p.Print(&out); err != nil {
		t.Fatal(err)
	}
	totals := strings.SplitN(out.String(), "TOTALS:\n", 2)
	if len(totals) != 2 || totals[1] != "alpha 3\nshared 2\nzeta 3\n" {
		t.Fatalf("totals: %q", out.String())
	}
}
