package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSparseIntervalSieveProductAuxiliaryPureAndNullaryConstruction(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	relations, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, relations.Close()) })
	kinds := []string{"tr_sparse", "interval_relation"}

	for _, tc := range []struct {
		name   string
		sorts  []ast.ASTHandle
		values [][]ast.ASTHandle
	}{
		{name: "finite", sorts: []ast.ASTHandle{f.boolean}, values: [][]ast.ASTHandle{{f.falsehood}, {f.truth}}},
		{name: "interval", sorts: []ast.ASTHandle{integer}, values: [][]ast.ASTHandle{{numeral("-20")}, {numeral("30")}}},
		{name: "nullary", values: [][]ast.ASTHandle{{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selectable, selectErr := CanSelectSparseIntervalSieveProduct(relations, tc.sorts, kinds)
			astTestMust(t, selectErr)
			if selectable {
				t.Fatal("public representation selector accepted auxiliary signature")
			}
			if public, publicErr := NewSparseIntervalSieveProduct(relations, tc.sorts, kinds, false); publicErr == nil {
				_ = public.Close()
				t.Fatal("public constructor accepted auxiliary signature")
			}

			empty, makeErr := NewSparseIntervalSieveProductAuxiliary(relations, tc.sorts, false)
			astTestMust(t, makeErr)
			t.Cleanup(func() { astTestMust(t, empty.Close()) })
			isEmpty, emptyErr := empty.Empty()
			astTestMust(t, emptyErr)
			if !isEmpty {
				t.Fatal("empty auxiliary constructor produced a nonempty relation")
			}

			full, makeErr := NewSparseIntervalSieveProductAuxiliary(relations, tc.sorts, true)
			astTestMust(t, makeErr)
			t.Cleanup(func() { astTestMust(t, full.Close()) })
			isEmpty, emptyErr = full.Empty()
			astTestMust(t, emptyErr)
			if isEmpty {
				t.Fatal("full auxiliary constructor produced an empty relation")
			}
			for _, values := range tc.values {
				contains, containsErr := full.ContainsFact(values)
				astTestMust(t, containsErr)
				if !contains {
					t.Fatal("full auxiliary relation omitted concrete tuple", values)
				}
			}
		})
	}

	selectable, err := CanSelectSparseIntervalSieveProduct(relations, []ast.ASTHandle{f.boolean, integer}, kinds)
	astTestMust(t, err)
	if !selectable {
		t.Fatal("public representation selector rejected a mixed signature")
	}
}

func TestSparseIntervalSieveProductAuxiliaryColumnsPreserveSourceAndValues(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	relations, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, relations.Close()) })
	source, err := NewSparseIntervalSieveProduct(relations, []ast.ASTHandle{f.boolean, integer}, []string{"tr_sparse", "interval_relation"}, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, source.Close()) })
	two, three, far := numeral("2"), numeral("3"), numeral("999")
	astTestMust(t, source.AddFact([]ast.ASTHandle{f.falsehood, two}))

	appendedFinite, err := source.AppendFullColumn(f.boolean)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, appendedFinite.Close()) })
	for _, value := range []ast.ASTHandle{f.falsehood, f.truth} {
		contains, containsErr := appendedFinite.ContainsFact([]ast.ASTHandle{f.falsehood, two, value})
		astTestMust(t, containsErr)
		if !contains {
			t.Fatal("full finite column omitted value", value)
		}
	}

	appendedInterval, err := source.AppendFullColumn(integer)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, appendedInterval.Close()) })
	contains, err := appendedInterval.ContainsFact([]ast.ASTHandle{f.falsehood, two, far})
	astTestMust(t, err)
	if !contains {
		t.Fatal("full interval column omitted value")
	}

	duplicateFinite, err := source.DuplicateColumn(0)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, duplicateFinite.Close()) })
	contains, err = duplicateFinite.ContainsFact([]ast.ASTHandle{f.falsehood, two, f.falsehood})
	astTestMust(t, err)
	if !contains {
		t.Fatal("finite duplicate omitted equal value")
	}
	contains, err = duplicateFinite.ContainsFact([]ast.ASTHandle{f.falsehood, two, f.truth})
	astTestMust(t, err)
	if contains {
		t.Fatal("finite duplicate admitted unequal value")
	}

	duplicateInterval, err := source.DuplicateColumn(1)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, duplicateInterval.Close()) })
	contains, err = duplicateInterval.ContainsFact([]ast.ASTHandle{f.falsehood, two, two})
	astTestMust(t, err)
	if !contains {
		t.Fatal("interval duplicate omitted equal value")
	}
	contains, err = duplicateInterval.ContainsFact([]ast.ASTHandle{f.falsehood, two, three})
	astTestMust(t, err)
	if contains {
		t.Fatal("interval duplicate admitted unequal value")
	}

	contains, err = source.ContainsFact([]ast.ASTHandle{f.falsehood, two})
	astTestMust(t, err)
	if !contains {
		t.Fatal("column transforms mutated their source")
	}
}
