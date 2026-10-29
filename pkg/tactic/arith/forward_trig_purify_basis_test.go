package arith

import "testing"

func TestForwardTrigPurifySharedSafeBasis(t *testing.T) {
	r, m, _ := forwardTrigFixture(t, false)
	theta := fixDLConst(t, m, false, "safe-theta")
	sin := fixDLApp(t, m, forwardTrigSin, theta)
	cos := fixDLApp(t, m, forwardTrigCos, theta)
	g := forwardTrigGoal(t, m, false, false, forwardTrigEq(t, m, sin, "0"), forwardTrigEq(t, m, cos, "1"))
	forwardTrigApply(t, r, g)
	size, e := g.Size()
	equationMust(t, e)
	if size != 4 {
		t.Fatal("source basis constraints", size)
	}
	first, e := g.Form(0)
	equationMust(t, e)
	second, e := g.Form(1)
	equationMust(t, e)
	x, e := m.GetArg(first, 0)
	equationMust(t, e)
	y, e := m.GetArg(second, 0)
	equationMust(t, e)
	if x == y {
		t.Fatal("sin/cos witnesses aliased")
	}
}
