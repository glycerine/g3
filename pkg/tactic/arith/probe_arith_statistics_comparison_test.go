package arith

// Source: src/tactic/arith/probe_arith.cpp:53-61.
import "testing"

func TestArithmeticDegreeProbeIgnoresNonArithmeticEquality(t *testing.T) {
	_, m := fixDLFixture(t, false)
	sort, e := m.MkBoolSort()
	equationMust(t, e)
	pName, e := m.Symbols.Intern("degree-bool-p")
	equationMust(t, e)
	qName, e := m.Symbols.Intern("degree-bool-q")
	equationMust(t, e)
	p, e := m.MkConst(pName, sort)
	equationMust(t, e)
	q, e := m.MkConst(qName, sort)
	equationMust(t, e)
	g := statisticGoal(t, m, statisticEquality(t, m, p, q))
	probe, e := NewArithmeticMaximumDegreeProbe(m.Budget())
	equationMust(t, e)
	defer probe.Close()
	got, e := probe.Eval(g)
	equationMust(t, e)
	if got != 0 {
		t.Fatal("Boolean equality counted", got)
	}
}
