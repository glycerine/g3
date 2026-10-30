package arith

// Source: src/tactic/arith/probe_arith.cpp:87-125.
import (
	"math"
	"testing"
)

func TestArithmeticBitWidthProbeAverageAndMaximum(t *testing.T) {
	_, m := fixDLFixture(t, false)
	x := fixDLConst(t, m, false, "bw-x")
	zero := fixDLNumber(t, m, false, "0")
	fraction := fixDLNumber(t, m, false, "3/4")
	negative := fixDLNumber(t, m, false, "-8")
	g := statisticGoal(t, m, statisticEquality(t, m, x, zero), statisticEquality(t, m, x, fraction), statisticEquality(t, m, x, negative))
	avg, e := NewArithmeticAverageBitWidthProbe(m.Budget())
	equationMust(t, e)
	defer avg.Close()
	max, e := NewArithmeticMaximumBitWidthProbe(m.Budget())
	equationMust(t, e)
	defer max.Close()
	a, e := avg.Eval(g)
	equationMust(t, e)
	b, e := max.Eval(g)
	equationMust(t, e)
	if math.Abs(a-10.0/3.0) > 1e-9 || b != 5 {
		t.Fatal("bit-width statistics", a, b)
	}
}
