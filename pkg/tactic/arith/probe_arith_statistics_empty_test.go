package arith

// Source: src/tactic/arith/probe_arith.cpp:77-80,120-123.
import "testing"

func TestArithmeticStatisticProbeEmptyReturnsZero(t *testing.T) {
	_, m := fixDLFixture(t, false)
	truth, e := m.MkTrue()
	equationMust(t, e)
	g := statisticGoal(t, m, truth)
	for _, makeProbe := range []func() (*ArithmeticStatisticProbe, error){func() (*ArithmeticStatisticProbe, error) { return NewArithmeticAverageDegreeProbe(m.Budget()) }, func() (*ArithmeticStatisticProbe, error) { return NewArithmeticMaximumDegreeProbe(m.Budget()) }, func() (*ArithmeticStatisticProbe, error) { return NewArithmeticAverageBitWidthProbe(m.Budget()) }, func() (*ArithmeticStatisticProbe, error) { return NewArithmeticMaximumBitWidthProbe(m.Budget()) }} {
		p, e := makeProbe()
		equationMust(t, e)
		got, e := p.Eval(g)
		equationMust(t, e)
		equationMust(t, p.Close())
		if got != 0 {
			t.Fatal("empty statistic", got)
		}
	}
}
