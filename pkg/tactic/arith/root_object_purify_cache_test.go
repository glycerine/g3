package arith

import "testing"

func TestRootObjectPurifyCachesSharedNumeral(t *testing.T) {
	r, m, _ := rootObjectFixture(t, false, true)
	root := rootObjectNumeral(t, m)
	x := fixDLConst(t, m, false, "root-cache-x")
	y := fixDLConst(t, m, false, "root-cache-y")
	a, e := m.MkEq(root, x)
	equationMust(t, e)
	b, e := m.MkEq(root, y)
	equationMust(t, e)
	fixDLPin(t, m, a)
	fixDLPin(t, m, b)
	g := rootObjectGoal(t, m, false, false, a, b)
	rootObjectApply(t, r, g)
	size, e := g.Size()
	equationMust(t, e)
	if size != 5 {
		t.Fatal("root cache duplicated constraints", size)
	}
}
