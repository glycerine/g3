package arith

// Source: src/tactic/arith/probe_arith.cpp:25-85.
import "testing"

func TestArithmeticDegreeProbeAverageAndMaximum(t *testing.T) {
	_, m := fixDLFixture(t, false)
	x := fixDLConst(t, m, true, "degree-x")
	y := fixDLConst(t, m, true, "degree-y")
	one := fixDLNumber(t, m, true, "1")
	lhs, e := m.MkArithmeticMul(x, x)
	equationMust(t, e)
	rhs, e := m.MkArithmeticAdd(y, one)
	equationMust(t, e)
	g := statisticGoal(t, m, statisticEquality(t, m, lhs, rhs))
	avg, e := NewArithmeticAverageDegreeProbe(m.Budget())
	equationMust(t, e)
	defer avg.Close()
	max, e := NewArithmeticMaximumDegreeProbe(m.Budget())
	equationMust(t, e)
	defer max.Close()
	a, e := avg.Eval(g)
	equationMust(t, e)
	b, e := max.Eval(g)
	equationMust(t, e)
	if a != 1.5 || b != 2 {
		t.Fatal("degree statistics", a, b)
	}
}
