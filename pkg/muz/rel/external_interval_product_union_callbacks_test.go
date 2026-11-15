package rel

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// AUDIT.md muz_rel_02-F3: aligned_union_fn allocates a fresh delta for each
// component in its side-union loop and again in its basic-union loop, so a
// delta union issues OP_RA_EMPTY twice before the two OP_RA_UNIONs; and
// unaligned_union_fn clones a source of another kind (OP_RA_CLONE).
func TestExternalIntervalProductDeltaUnionCallbacks(t *testing.T) {
	f := newTableRelationFixture(t)
	family, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	realSort, err := f.ast.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	astTestPin(t, f.ast, realSort)
	one, err := f.ast.ArithmeticRealNumeralText("1")
	astTestMust(t, err)
	astTestPin(t, f.ast, one)
	var events []int32
	record := func(declaration ast.ASTHandle) error {
		op, cause := f.ast.GetDeclKind(declaration)
		if cause != nil {
			return cause
		}
		events = append(events, op)
		return nil
	}
	callbacks := ExternalRelationCallbacks{
		Reduce: func(declaration ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
			return ast.ASTHandle{}, record(declaration)
		},
		ReduceAssign: func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error {
			return record(declaration)
		},
	}
	plugin, err := NewExternalRelationPlugin(f.relations, callbacks)
	astTestMust(t, err)
	defer func() { astTestMust(t, plugin.Close()) }()
	makeProduct := func(kinds []string, add bool) *ExternalIntervalProduct {
		t.Helper()
		product, selected, cause := NewSelectedExternalIntervalProduct(plugin, []ast.ASTHandle{realSort}, kinds)
		astTestMust(t, cause)
		if !selected {
			t.Fatal("mixed product not selected", kinds)
		}
		if add {
			astTestMust(t, product.AddFact([]ast.ASTHandle{one}))
		}
		return product
	}
	name := func(ops []int32) string {
		names := map[int32]string{ast.DatalogOpEmpty: "EMPTY", ast.DatalogOpUnion: "UNION", ast.DatalogOpWiden: "WIDEN", ast.DatalogOpClone: "CLONE"}
		var out []string
		for _, op := range ops {
			if text, ok := names[op]; ok {
				out = append(out, text)
			}
		}
		return fmt.Sprint(out)
	}
	externalFirst := []string{"external_relation", "interval_relation"}
	intervalFirst := []string{"interval_relation", "external_relation"}
	for _, tc := range []struct {
		source []string
		want   string
	}{
		{source: externalFirst, want: "[EMPTY EMPTY UNION UNION]"},
		{source: intervalFirst, want: "[CLONE EMPTY EMPTY UNION UNION]"},
	} {
		target, source, delta := makeProduct(externalFirst, false), makeProduct(tc.source, true), makeProduct(externalFirst, false)
		events = nil
		astTestMust(t, target.Union(source, delta, false))
		if got := name(events); got != tc.want {
			t.Fatalf("source kinds %v: callbacks %s, want %s", tc.source, got, tc.want)
		}
		astTestMust(t, delta.Close())
		astTestMust(t, source.Close())
		astTestMust(t, target.Close())
	}
}
