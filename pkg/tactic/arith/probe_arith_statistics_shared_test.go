package arith

// Source: src/ast/for_each_expr.h:27-105, MarkAll goal traversal.
import "testing"

func TestArithmeticStatisticProbeDeduplicatesSharedNumeralAcrossRoots(t *testing.T) {
	_, m := fixDLFixture(t, false)
	x := fixDLConst(t, m, false, "shared-bw-x")
	y := fixDLConst(t, m, false, "shared-bw-y")
	n := fixDLNumber(t, m, false, "3/4")
	g := statisticGoal(t, m, statisticEquality(t, m, x, n), statisticEquality(t, m, y, n))
	p, e := NewArithmeticAverageBitWidthProbe(m.Budget())
	equationMust(t, e)
	defer p.Close()
	got, e := p.Eval(g)
	equationMust(t, e)
	if got != 5 {
		t.Fatal("shared numeral visited repeatedly", got)
	}
}
