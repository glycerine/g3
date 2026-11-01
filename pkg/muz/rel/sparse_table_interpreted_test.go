package rel

import (
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSparseTableDefaultInterpretedFilterEvaluatesFiniteRows(t *testing.T) {
	f := newTableRelationFixture(t)
	sorts := []ast.ASTHandle{f.boolean, f.bits}
	signature, err := tableRelationSignature(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	table, err := NewSparseTable(signature, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer table.Close()
	for _, fact := range []Fact{{0, 1}, {1, 2}, {1, 3}} {
		if _, err = table.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	rewriter := tableRelationTheoryRewriter(t, f.ast)
	own := func(handle ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		astTestPin(t, f.ast, handle)
		return handle
	}
	boolVar := own(f.ast.MkVar(0, f.boolean))
	if err = table.FilterInterpreted(f.relations, sorts, rewriter, boolVar); err != nil {
		t.Fatal(err)
	}
	bitVar := own(f.ast.MkVar(1, f.bits))
	bitTwo := own(f.ast.BitVectorNumeralUint64(2, 2))
	condition := own(f.ast.MkEq(bitVar, bitTwo))
	if err = table.FilterInterpreted(f.relations, sorts, rewriter, condition); err != nil {
		t.Fatal(err)
	}
	facts, err := table.Facts()
	if err != nil || !reflect.DeepEqual(facts, []Fact{{1, 2}}) {
		t.Fatalf("interpreted-filter facts = %v, err=%v", facts, err)
	}
}

func TestSparseTableDefaultInterpretedFilterRejectsMismatchBeforeMutation(t *testing.T) {
	f := newTableRelationFixture(t)
	sorts := []ast.ASTHandle{f.boolean, f.bits}
	signature, err := tableRelationSignature(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	table, err := NewSparseTable(signature, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer table.Close()
	if _, err = table.Add(Fact{1, 2}); err != nil {
		t.Fatal(err)
	}
	before, err := table.Facts()
	if err != nil {
		t.Fatal(err)
	}
	rewriter := tableRelationTheoryRewriter(t, f.ast)
	variable, err := f.ast.MkVar(0, f.boolean)
	if err != nil {
		t.Fatal(err)
	}
	astTestPin(t, f.ast, variable)
	for _, invalidSorts := range [][]ast.ASTHandle{sorts[:1], {f.bits, f.boolean}} {
		if err = table.FilterInterpreted(f.relations, invalidSorts, rewriter, variable); err == nil {
			t.Fatal("invalid relation sorts accepted")
		}
		facts, factErr := table.Facts()
		if factErr != nil || !reflect.DeepEqual(facts, before) {
			t.Fatalf("invalid filter changed table: facts=%v err=%v", facts, factErr)
		}
	}
	unknown, err := f.ast.MkConst(astTestSymbol(t, f.ast, "sparse-filter-unknown"), f.boolean)
	if err != nil {
		t.Fatal(err)
	}
	astTestPin(t, f.ast, unknown)
	if err = table.FilterInterpreted(f.relations, sorts, rewriter, unknown); err != nil {
		t.Fatal(err)
	}
	facts, err := table.Facts()
	if err != nil || !reflect.DeepEqual(facts, before) {
		t.Fatalf("symbolic condition should keep the row: facts=%v err=%v", facts, err)
	}
	bitTwo, err := f.ast.BitVectorNumeralUint64(2, 2)
	if err != nil {
		t.Fatal(err)
	}
	astTestPin(t, f.ast, bitTwo)
	if err = table.FilterInterpreted(f.relations, sorts, rewriter, bitTwo); err == nil {
		t.Fatal("non-Boolean condition accepted")
	}
}
