package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestCanSelectSparseIntervalSieveProductExactPairWithoutAllocation(t *testing.T) {
	f, integer, _ := intervalSieveFixture(t)
	relations, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, relations.Close()) }()
	mixed := []ast.ASTHandle{f.boolean, integer}
	beforeRefs := f.ast.Stats().LiveReferences
	beforeBytes := f.ast.Budget().Used()
	for _, kinds := range [][]string{
		{"tr_sparse", "interval_relation"},
		{"interval_relation", "tr_sparse"},
	} {
		selected, cause := CanSelectSparseIntervalSieveProduct(relations, mixed, kinds)
		if cause != nil || !selected {
			t.Fatalf("exact mixed pair was not selected: %v, %v", kinds, cause)
		}
	}
	if got := f.ast.Stats().LiveReferences; got != beforeRefs {
		t.Fatal("selection probe allocated AST references", beforeRefs, got)
	}
	if got := f.ast.Budget().Used(); got != beforeBytes {
		t.Fatal("selection probe allocated manager storage", beforeBytes, got)
	}
}

func TestCanSelectSparseIntervalSieveProductRejectsNonMixedAndNames(t *testing.T) {
	f, integer, _ := intervalSieveFixture(t)
	relations, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, relations.Close()) }()
	for _, tc := range []struct {
		name  string
		sorts []ast.ASTHandle
	}{
		{name: "finite", sorts: []ast.ASTHandle{f.boolean}},
		{name: "arithmetic", sorts: []ast.ASTHandle{integer}},
		{name: "nullary"},
	} {
		selected, cause := CanSelectSparseIntervalSieveProduct(relations, tc.sorts, []string{"tr_sparse", "interval_relation"})
		if cause != nil || selected {
			t.Fatalf("%s signature selected at caller boundary: %v, %v", tc.name, selected, cause)
		}
	}
	for _, kinds := range [][]string{
		nil,
		{"tr_sparse"},
		{"interval_relation"},
		{"tr_sparse", "tr_sparse"},
		{"interval_relation", "interval_relation"},
		{"tr_sparse", "unknown"},
		{"tr_sparse", "interval_relation", "tr_sparse"},
	} {
		selected, cause := CanSelectSparseIntervalSieveProduct(nil, nil, kinds)
		if cause != nil || selected {
			t.Fatalf("invalid names consulted relation manager: %v, %v, %v", kinds, selected, cause)
		}
	}
}

func TestCanSelectSparseIntervalSieveProductReportsUnsupportedSort(t *testing.T) {
	f, integer, _ := intervalSieveFixture(t)
	relations, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, relations.Close()) }()
	name, err := f.ast.Symbols.Intern("sparse-interval-selection-unsupported")
	astTestMust(t, err)
	unsupported, err := f.ast.MkUninterpretedSort(name, nil)
	astTestMust(t, err)
	astTestPin(t, f.ast, unsupported)
	selected, cause := CanSelectSparseIntervalSieveProduct(relations,
		[]ast.ASTHandle{unsupported, integer}, []string{"tr_sparse", "interval_relation"})
	if cause == nil || selected {
		t.Fatal("unsupported finite sort did not report selection error", selected, cause)
	}
}
