package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestCheckTableInterpretedFilterAndProject(t *testing.T) {
	f := newTableRelationFixture(t)
	rewriter := tableRelationTheoryRewriter(t, f.ast)
	table, err := NewCheckTable(lazySignature(t, 2))
	if err != nil {
		t.Fatal(err)
	}
	closeCheckTables(t, table)
	for _, fact := range []Fact{{0}, {1}} {
		if _, err = table.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	variable, err := f.ast.MkVar(0, f.boolean)
	if err != nil {
		t.Fatal(err)
	}
	columns := map[uint32]int{0: 0}
	sorts := map[uint32]ast.ASTHandle{0: f.boolean}
	if err = table.FilterInterpreted(f.relations, []ast.ASTHandle{f.boolean}, rewriter, []ast.ASTHandle{variable}, columns, sorts); err != nil {
		t.Fatal(err)
	}
	if rows, err := table.Facts(); err != nil || len(rows) != 1 || rows[0][0] != 1 {
		t.Fatalf("checked interpreted filter rows = %v, %v", rows, err)
	}

	projected, err := table.FilterInterpretedAndProject(f.relations, []ast.ASTHandle{f.boolean}, rewriter, []ast.ASTHandle{variable}, columns, sorts, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	closeCheckTables(t, projected)
	rows, err := projected.Facts()
	if err != nil || len(rows) != 1 || len(rows[0]) != 0 {
		t.Fatalf("checked interpreted zero-column projection = %v, %v", rows, err)
	}
}
