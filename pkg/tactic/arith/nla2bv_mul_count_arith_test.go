package arith

// Source: src/tactic/arith/nla2bv_tactic.cpp:366-376,393-397.

import "testing"

func TestNLA2BVCountMulArithmeticEdgesAndSharedRoots(t *testing.T) {
	r := nla2BVFixture(t, false)
	m := r.manager
	x := fixDLConst(t, m, true, "count-mul-x")
	y := fixDLConst(t, m, true, "count-mul-y")
	z := fixDLConst(t, m, true, "count-mul-z")
	xy, err := m.MkArithmeticMul(x, y)
	equationMust(t, err)
	xyz, err := m.MkArithmeticMul(xy, z)
	equationMust(t, err)
	zero := fixDLNumber(t, m, true, "0")
	first, err := m.MkEq(xyz, zero)
	equationMust(t, err)
	second, err := m.MkEq(xy, zero)
	equationMust(t, err)
	fixDLPin(t, m, first)
	fixDLPin(t, m, second)
	g := fixDLGoal(t, m, false, false, false, first, second)
	got, err := (&nla2BVInvocation{t: r}).countMultiplications(g)
	equationMust(t, err)
	if got != 2 {
		t.Fatal("arithmetic multiplication edges or shared MarkAll traversal", got)
	}
}
