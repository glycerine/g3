package interp

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/glycerine/g3/pkg/ast"
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

// AUDIT.md interp_02-F10: output_time writes `os << double` (precision 6),
// and iz3proof::interpolate times Blits, interp_proof and simplifying.
func TestInterpolationProfilerSourcePrecisionAndInterpolateTimers(t *testing.T) {
	if got := profileSeconds(1234567 * time.Nanosecond); got != "0.00123457" {
		t.Fatalf("profileSeconds = %q", got)
	}
	if got := profileSeconds(10 * time.Microsecond); got != "1e-05" {
		t.Fatalf("profileSeconds = %q", got)
	}
	m, manager := interpRewritingManagerFixture(t)
	integer, err := manager.IntType()
	interpMust(t, err)
	x, err := manager.MakeVariable("profile-x", integer)
	interpMust(t, err)
	y, err := manager.MakeVariable("profile-y", integer)
	interpMust(t, err)
	zero, err := manager.MakeIntText("0")
	interpMust(t, err)
	xFrame, err := manager.Make(OperatorLE, x, zero)
	interpMust(t, err)
	yFrame, err := manager.Make(OperatorLE, y, zero)
	interpMust(t, err)
	base, err := NewBase(manager, []ast.ASTHandle{xFrame, yFrame}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	translation, err := NewFullTranslation(base)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, translation.Close()) })
	equality, err := manager.Make(OperatorEqual, x, y)
	interpMust(t, err)
	xLeqY, err := manager.Make(OperatorLE, x, y)
	interpMust(t, err)
	yLeqX, err := manager.Make(OperatorLE, y, x)
	interpMust(t, err)
	notXLeqY, err := manager.MkNot(xLeqY)
	interpMust(t, err)
	notYLeqX, err := manager.MkNot(yLeqX)
	interpMust(t, err)
	conclusion, err := manager.Make(OperatorOr, equality, notXLeqY, notYLeqX)
	interpMust(t, err)
	arithmetic, err := m.ArithmeticFamilyID()
	interpMust(t, err)
	lemma := interpTheoryLemma(t, m, arithmetic, "triangle-eq", conclusion, nil)
	destination, err := NewInterpolationProof(base)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, destination.Close()) })
	node, err := translation.Translate(lemma, destination)
	interpMust(t, err)
	profiler := NewProfiler()
	destination.SetProfiler(profiler)
	interpolant, err := destination.Interpolate(node, DownwardScopeRange(0), false)
	interpMust(t, err)
	interpMust(t, m.DelRef(&interpolant))
	if got := strings.Join(profiler.top.order, ","); got != "Blits,interp_proof,simplifying" {
		t.Fatalf("interpolate timers %q", got)
	}
	if profiler.current != profiler.top {
		t.Fatal("interpolate timers left open")
	}
}
