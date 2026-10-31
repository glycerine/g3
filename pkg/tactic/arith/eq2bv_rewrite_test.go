package arith

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestEq2BVRewriteNestedEqualities(t *testing.T) {
	bounds, m := boundManagerFixture(t)
	x := fixDLConst(t, m, true, "rewrite-x")
	y := fixDLConst(t, m, true, "rewrite-y")
	n := func(s string) ast.ASTHandle { return fixDLNumber(t, m, true, s) }
	a := collectorEq(t, m, x, n("2"))
	b := collectorEq(t, m, n("3"), x)
	c := collectorEq(t, m, y, n("1"))
	root, e := m.MkOrArgs([]ast.ASTHandle{a, b, c})
	equationMust(t, e)
	fixDLPin(t, m, root)
	equationMust(t, bounds.Collect(root, ast.ASTDependency{}))
	plan, e := eq2bvPrepare(m, bounds, []ast.ASTHandle{root})
	equationMust(t, e)
	defer func() { equationMust(t, plan.Close()) }()
	if len(plan.entries) != 2 {
		t.Fatalf("replacements: %d", len(plan.entries))
	}
	output, e := eq2bvRewrite(plan, root)
	equationMust(t, e)
	defer func() { equationMust(t, m.DelRef(&output)) }()
	rewritten, e := m.RefGet(output)
	equationMust(t, e)
	for i, want := range []string{"2", "3", "1"} {
		eq, e := m.GetArg(rewritten, uint32(i))
		equationMust(t, e)
		fresh, e := m.GetArg(eq, 0)
		equationMust(t, e)
		numeral, e := m.GetArg(eq, 1)
		equationMust(t, e)
		text, found, e := m.BitVectorNumeralText(numeral)
		equationMust(t, e)
		if !found || text != want {
			t.Fatalf("numeral %d: %q", i, text)
		}
		sort, e := m.GetSort(fresh)
		equationMust(t, e)
		isBV, e := m.IsBitVectorSort(sort)
		equationMust(t, e)
		if !isBV {
			t.Fatal("replacement is not a bit-vector")
		}
	}
}
