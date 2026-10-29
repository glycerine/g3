package arith

import "testing"

func TestRootObjectPurifyDefiningPolynomialAndBounds(t *testing.T) {
	r, m, _ := rootObjectFixture(t, false, true)
	root := rootObjectNumeral(t, m)
	x := fixDLConst(t, m, false, "root-x")
	formula, e := m.MkEq(root, x)
	equationMust(t, e)
	fixDLPin(t, m, formula)
	g := rootObjectGoal(t, m, false, false, formula)
	rootObjectApply(t, r, g)
	size, e := g.Size()
	equationMust(t, e)
	if size != 4 {
		t.Fatal("root constraints", size)
	}
	rewritten, e := g.Form(0)
	equationMust(t, e)
	left, e := m.GetArg(rewritten, 0)
	equationMust(t, e)
	if left == root {
		t.Fatal("root object survived")
	}
}
func TestRootObjectPurifyDisabledLeavesNumeral(t *testing.T) {
	r, m, _ := rootObjectFixture(t, false, false)
	root := rootObjectNumeral(t, m)
	x := fixDLConst(t, m, false, "root-disabled-x")
	formula, e := m.MkEq(root, x)
	equationMust(t, e)
	fixDLPin(t, m, formula)
	g := rootObjectGoal(t, m, false, false, formula)
	rootObjectApply(t, r, g)
	h, e := g.Form(0)
	equationMust(t, e)
	if h != formula {
		t.Fatal("disabled root changed")
	}
}
