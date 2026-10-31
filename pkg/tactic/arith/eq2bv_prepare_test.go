package arith

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestEq2BVPrepareUsesBoundsAndOwnsFreshVariables(t *testing.T) {
	bounds, m := boundManagerFixture(t)
	x := fixDLConst(t, m, true, "prepare-x")
	y := fixDLConst(t, m, true, "prepare-y")
	z := fixDLConst(t, m, true, "prepare-z")
	numeral := func(s string) ast.ASTHandle { return fixDLNumber(t, m, true, s) }
	notEq := func(variable ast.ASTHandle, n string) ast.ASTHandle {
		eq := collectorEq(t, m, variable, numeral(n))
		h, e := m.MkNot(eq)
		equationMust(t, e)
		fixDLPin(t, m, h)
		return h
	}
	upper := fixDLApp(t, m, 2, x, numeral("10"))
	roots := []ast.ASTHandle{notEq(x, "3"), upper, notEq(y, "3"), notEq(z, "0")}
	for _, root := range roots {
		equationMust(t, bounds.Collect(root, ast.ASTDependency{}))
	}
	plan, e := eq2bvPrepare(m, bounds, roots)
	equationMust(t, e)
	if len(plan.entries) != 3 {
		t.Fatalf("replacements: %d", len(plan.entries))
	}
	expected := map[ast.ASTHandle]struct{ max, width uint32 }{x: {10, 4}, y: {4, 3}, z: {1, 1}}
	var fresh []ast.ASTHandle
	for _, entry := range plan.entries {
		want, ok := expected[entry.original]
		if !ok || entry.maximum != want.max || entry.width != want.width {
			t.Fatalf("replacement %+v want %+v", entry, want)
		}
		h, e := m.RefGet(entry.fresh)
		equationMust(t, e)
		fresh = append(fresh, h)
		s, e := m.GetSort(h)
		equationMust(t, e)
		size, e := m.BitVectorSize(s)
		equationMust(t, e)
		if size != want.width {
			t.Fatalf("bit-vector width %d want %d", size, want.width)
		}
	}
	equationMust(t, plan.Close())
	for _, h := range fresh {
		if _, e := m.GetKind(h); e == nil {
			t.Fatal("fresh variable outlived its owner")
		}
	}
}

func TestEq2BVWidthSourceUnsignedBoundaries(t *testing.T) {
	cases := []struct {
		max             uint32
		bounded         bool
		adjusted, width uint32
	}{
		{0, true, 0, 1}, {1, true, 1, 1}, {2, true, 2, 2}, {3, true, 3, 2}, {3, false, 4, 3},
		{2147483648, true, 2147483648, 0}, {4294967295, false, 0, 1},
	}
	for _, tc := range cases {
		adjusted, width := eq2bvWidth(tc.max, tc.bounded)
		if adjusted != tc.adjusted || width != tc.width {
			t.Fatalf("max %d bounded %v: %d/%d want %d/%d", tc.max, tc.bounded, adjusted, width, tc.adjusted, tc.width)
		}
	}
}
