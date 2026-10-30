package arith

// Source: src/tactic/arith/nla2bv_tactic.cpp:377-388.

import "testing"

func TestNLA2BVCountMulBVIgnoresNumeralCoefficients(t *testing.T) {
	r := nla2BVFixture(t, false)
	m := r.manager
	_, err := m.RegisterBitVectorPlugin()
	equationMust(t, err)
	sort, err := m.BitVectorSort(8)
	equationMust(t, err)
	xName, err := m.Symbols.Intern("count-bv-x")
	equationMust(t, err)
	x, err := m.MkConst(xName, sort)
	equationMust(t, err)
	// A source var is non-numeral without declaration-family access.
	y, err := m.MkVar(0, sort)
	equationMust(t, err)
	two, err := m.BitVectorNumeralUint64(2, 8)
	equationMust(t, err)
	coefficient, err := m.MkBitVectorMul(two, x)
	equationMust(t, err)
	symbolic, err := m.MkBitVectorMul(coefficient, y)
	equationMust(t, err)
	goalRoot, err := m.MkEq(symbolic, two)
	equationMust(t, err)
	fixDLPin(t, m, goalRoot)
	g := fixDLGoal(t, m, false, false, false, goalRoot)
	got, err := (&nla2BVInvocation{t: r}).countMultiplications(g)
	equationMust(t, err)
	if got != 1 {
		t.Fatal("BV symbolic factor count", got)
	}
}
