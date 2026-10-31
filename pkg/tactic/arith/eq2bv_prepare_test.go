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

func TestEq2BVPrepareRegistersBitVectors(t *testing.T) {
	base, source := fixDLFixture(t, false)
	m, e := ast.NewASTManager(ast.ASTManagerConfig{Symbols: source.Symbols, Numerals: source.NumeralService(), Budget: source.Budget()})
	equationMust(t, e)
	t.Cleanup(func() { equationMust(t, m.Close()) })
	if _, e = m.BitVectorSort(2); e == nil {
		t.Fatal("fixture unexpectedly registered bit-vectors")
	}
	bounds, e := NewBoundManager(m, base.context, base.globals)
	equationMust(t, e)
	t.Cleanup(func() { equationMust(t, bounds.Close()) })
	x := fixDLConst(t, m, true, "new-manager-x")
	n := fixDLNumber(t, m, true, "3")
	root := collectorEq(t, m, x, n)
	equationMust(t, bounds.Collect(root, ast.ASTDependency{}))
	plan, e := eq2bvPrepare(m, bounds, []ast.ASTHandle{root})
	equationMust(t, e)
	equationMust(t, plan.Close())
}

func TestEq2BVPrepareWidthsCoverLowerAndLargeBoundaries(t *testing.T) {
	bounds, m := boundManagerFixture(t)
	x := fixDLConst(t, m, true, "lower-width-x")
	y := fixDLConst(t, m, true, "large-unbounded-y")
	z := fixDLConst(t, m, true, "large-bounded-z")
	n := func(value string) ast.ASTHandle { return fixDLNumber(t, m, true, value) }
	maxEquality := collectorEq(t, m, y, n("4294967295"))
	unequalMax, err := m.MkNot(maxEquality)
	equationMust(t, err)
	fixDLPin(t, m, unequalMax)
	zEquality := collectorEq(t, m, z, n("1"))
	unequalZ, err := m.MkNot(zEquality)
	equationMust(t, err)
	fixDLPin(t, m, unequalZ)
	roots := []ast.ASTHandle{
		collectorEq(t, m, x, n("1")), fixDLApp(t, m, 3, x, n("5")),
		unequalMax,
		unequalZ, fixDLApp(t, m, 2, z, n("2147483648")),
	}
	for _, root := range roots {
		equationMust(t, bounds.Collect(root, ast.ASTDependency{}))
	}
	plan, err := eq2bvPrepare(m, bounds, roots)
	equationMust(t, err)
	defer func() { equationMust(t, plan.Close()) }()
	want := map[ast.ASTHandle]uint32{x: 3, y: 33, z: 32}
	for _, entry := range plan.entries {
		width, found := want[entry.original]
		if !found || entry.width != width {
			t.Fatalf("replacement width %d, expected %d", entry.width, width)
		}
		delete(want, entry.original)
	}
	if len(want) != 0 {
		t.Fatal("missing replacement widths", want)
	}
}

func TestEq2BVPrepareReservesNegativeSentinel(t *testing.T) {
	bounds, m := boundManagerFixture(t)
	x := fixDLConst(t, m, true, "negative-sentinel-x")
	zero := fixDLNumber(t, m, true, "0")
	eq := collectorEq(t, m, x, zero)
	neq, err := m.MkNot(eq)
	equationMust(t, err)
	fixDLPin(t, m, neq)
	upper := fixDLApp(t, m, 2, x, zero)
	for _, root := range []ast.ASTHandle{neq, upper} {
		equationMust(t, bounds.Collect(root, ast.ASTDependency{}))
	}
	plan, err := eq2bvPrepare(m, bounds, []ast.ASTHandle{neq, upper})
	equationMust(t, err)
	defer func() { equationMust(t, plan.Close()) }()
	if len(plan.entries) != 1 || !plan.entries[0].negativeSentinel || plan.entries[0].width != 1 {
		t.Fatal("negative sentinel allocation", plan.entries)
	}
}

func TestEq2BVSentinelReservesDistinctLargeValue(t *testing.T) {
	for _, row := range []struct {
		bound string
		width uint32
	}{{"3", 3}, {"4294967295", 33}} {
		t.Run(row.bound, func(t *testing.T) {
			bounds, m := boundManagerFixture(t)
			x := fixDLConst(t, m, true, "sentinel-boundary-x")
			n := fixDLNumber(t, m, true, row.bound)
			eq := collectorEq(t, m, x, n)
			neq, err := m.MkNot(eq)
			equationMust(t, err)
			fixDLPin(t, m, neq)
			upper := fixDLApp(t, m, 2, x, n)
			for _, root := range []ast.ASTHandle{neq, upper} {
				equationMust(t, bounds.Collect(root, ast.ASTDependency{}))
			}
			plan, err := eq2bvPrepare(m, bounds, []ast.ASTHandle{neq, upper})
			equationMust(t, err)
			defer func() { equationMust(t, plan.Close()) }()
			if len(plan.entries) != 1 || !plan.entries[0].negativeSentinel || plan.entries[0].width != row.width {
				t.Fatal("sentinel collides with observed numeral", plan.entries)
			}
		})
	}
}
