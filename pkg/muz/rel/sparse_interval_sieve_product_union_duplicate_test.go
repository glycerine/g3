package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// vector_relation::copy copies representative values into fresh destination
// equality classes. DuplicateColumn is exact before union, while union into an
// empty interval target deliberately keeps only the per-column intervals.
func TestSparseIntervalSieveProductUnionCoarsensOnlyIntervalDuplicateEquality(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	relations, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, relations.Close()) }()
	source, err := NewSparseIntervalSieveProduct(relations,
		[]ast.ASTHandle{f.boolean, integer},
		[]string{"tr_sparse", "interval_relation"}, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	five, zero, hundred := numeral("5"), numeral("0"), numeral("100")
	astTestMust(t, source.AddFact([]ast.ASTHandle{f.falsehood, five}))

	appendedFinite, err := source.AppendFullColumn(f.boolean)
	astTestMust(t, err)
	defer func() { astTestMust(t, appendedFinite.Close()) }()
	duplicatedFinite, err := appendedFinite.DuplicateColumn(2)
	astTestMust(t, err)
	defer func() { astTestMust(t, duplicatedFinite.Close()) }()
	appendedInterval, err := duplicatedFinite.AppendFullColumn(integer)
	astTestMust(t, err)
	defer func() { astTestMust(t, appendedInterval.Close()) }()
	duplicatedInterval, err := appendedInterval.DuplicateColumn(4)
	astTestMust(t, err)
	defer func() { astTestMust(t, duplicatedInterval.Close()) }()
	assembled, err := duplicatedInterval.Project([]int{0, 1})
	astTestMust(t, err)
	defer func() { astTestMust(t, assembled.Close()) }()

	contains := func(product *SparseIntervalSieveProduct, values ...ast.ASTHandle) bool {
		t.Helper()
		found, cause := product.ContainsFact(values)
		astTestMust(t, cause)
		return found
	}
	if !contains(assembled, f.truth, f.truth, zero, zero) ||
		contains(assembled, f.truth, f.falsehood, zero, zero) ||
		contains(assembled, f.truth, f.truth, zero, hundred) {
		t.Fatal("duplicate construction did not preserve exact component equalities")
	}

	target, err := NewSparseIntervalSieveProduct(relations,
		[]ast.ASTHandle{f.boolean, f.boolean, integer, integer},
		[]string{"tr_sparse", "interval_relation"}, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	changed, err := target.Union(assembled, false)
	astTestMust(t, err)
	if !changed || !contains(target, f.truth, f.truth, zero, zero) {
		t.Fatal("union into empty target lost the duplicated tuple", changed)
	}
	if contains(target, f.truth, f.falsehood, zero, zero) {
		t.Fatal("sparse finite union lost explicit duplicate-column equality")
	}
	if !contains(target, f.truth, f.truth, zero, hundred) {
		t.Fatal("interval union unexpectedly preserved an equality class discarded by source copy")
	}
}
