package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestCheckedWidenUsesUnionVerificationAndDelta(t *testing.T) {
	t.Run("formula-relation", func(t *testing.T) {
		f := newTableRelationFixture(t)
		makeChecked := func() *FormulaCheckedTable {
			table, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean})
			if err != nil {
				t.Fatal(err)
			}
			checked, err := NewFormulaCheckedTable(table, formulaCheckerConfig(t, f.ast))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { astTestMust(t, checked.Close()) })
			return checked
		}
		target, source, delta := makeChecked(), makeChecked(), makeChecked()
		if _, err := target.AddFact([]ast.ASTHandle{f.falsehood}); err != nil {
			t.Fatal(err)
		}
		if _, err := source.AddFact([]ast.ASTHandle{f.truth}); err != nil {
			t.Fatal(err)
		}
		if err := target.WidenFrom(source, delta); err != nil {
			t.Fatal(err)
		}
		if rows, err := target.SizeEstimateRows(); err != nil || rows != 2 {
			t.Fatal("widen target", rows, err)
		}
		if rows, err := delta.SizeEstimateRows(); err != nil || rows != 1 {
			t.Fatal("widen delta", rows, err)
		}
		if err := target.WidenFrom(source, delta); err != nil {
			t.Fatal("repeated widen", err)
		}
		if rows, err := delta.SizeEstimateRows(); err != nil || rows != 1 {
			t.Fatal("widen preserves prior delta", rows, err)
		}
	})

	t.Run("paired-table", func(t *testing.T) {
		signature, err := NewSignature([]uint64{3})
		must(t, err)
		target := newCheckTableWithFacts(t, signature, Fact{0})
		source := newCheckTableWithFacts(t, signature, Fact{2})
		delta := newCheckTableWithFacts(t, signature)
		defer closeCheckTables(t, target, source, delta)
		if err := target.WidenFrom(source, delta); err != nil {
			t.Fatal(err)
		}
		requireCheckedFact(t, target, Fact{0}, true)
		requireCheckedFact(t, target, Fact{2}, true)
		requireCheckedFact(t, delta, Fact{2}, true)
		if err := target.WidenFrom(source, delta); err != nil {
			t.Fatal("repeated widen", err)
		}
		if rows, err := delta.Facts(); err != nil || len(rows) != 1 || rows[0][0] != 2 {
			t.Fatal("paired widen preserves prior delta", rows, err)
		}
	})
}
