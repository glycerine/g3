package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// TestTableNotEqualFilterRemovesMatchingColumn checks
// default_table_filter_not_equal_fn: (not (= x c)) removes exactly the facts
// whose column holds c, without grounding the condition per fact.
func TestTableNotEqualFilterRemovesMatchingColumn(t *testing.T) {
	f := newTableRelationFixture(t)
	rewriter := tableRelationTheoryRewriter(t, f.ast)
	x, err := f.ast.MkVar(1, f.bits)
	if err != nil {
		t.Fatal(err)
	}
	two, err := f.ast.MkDatalogNumeral(2, f.bits)
	if err != nil {
		t.Fatal(err)
	}
	equal, err := f.ast.MkEq(two, x)
	if err != nil {
		t.Fatal(err)
	}
	condition, err := f.ast.MkNot(equal)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := f.ast.NewRef(condition)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.ast.DelRef(&reference) }()
	filter, matched, err := f.relations.notEqualFilter(condition)
	if err != nil || !matched || filter.variable != 1 || filter.value != 2 {
		t.Fatal("not-equal recognizer", filter, matched, err)
	}
	if _, matched, err = f.relations.notEqualFilter(equal); err != nil || matched {
		t.Fatal("plain equality recognized as not-equal", matched, err)
	}
	relation, err := NewTableRelation(f.relations, []ast.ASTHandle{f.bits, f.bits})
	if err != nil {
		t.Fatal(err)
	}
	closeTableRelations(t, relation)
	for _, fact := range []Fact{{0, 2}, {1, 2}, {2, 1}, {3, 3}} {
		if _, err = relation.AddTableFact(fact); err != nil {
			t.Fatal(err)
		}
	}
	columns := map[uint32]int{1: 1}
	sorts := map[uint32]ast.ASTHandle{1: f.bits}
	if err = relation.FilterInterpreted(rewriter, []ast.ASTHandle{condition}, columns, sorts); err != nil {
		t.Fatal(err)
	}
	if relation.NumRows() != 2 {
		t.Fatal("not-equal filter rows", relation.NumRows())
	}
}
