package rel

import (
	"errors"
	"testing"
)

func TestCheckTableJoinProjectRenameAndSelection(t *testing.T) {
	left := newCheckTableWithFacts(t, lazySignature(t, 2, 3), Fact{0, 1}, Fact{1, 2})
	right := newCheckTableWithFacts(t, lazySignature(t, 3, 2), Fact{1, 1}, Fact{2, 0}, Fact{0, 1})
	closeCheckTables(t, left, right)
	joined, err := left.Join(right, []int{1}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	closeCheckTables(t, joined)
	requireCheckedFact(t, joined, Fact{0, 1, 1, 1}, true)
	requireCheckedFact(t, joined, Fact{1, 2, 2, 0}, true)
	if facts, _ := joined.Facts(); len(facts) != 2 {
		t.Fatal("join facts", facts)
	}

	joinProject, err := left.JoinProject(right, []int{1}, []int{0}, []int{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	closeCheckTables(t, joinProject)
	requireCheckedFact(t, joinProject, Fact{0, 1}, true)
	requireCheckedFact(t, joinProject, Fact{1, 0}, true)

	projected, err := left.Project([]int{0})
	if err != nil {
		t.Fatal(err)
	}
	closeCheckTables(t, projected)
	requireCheckedFact(t, projected, Fact{1}, true)
	requireCheckedFact(t, projected, Fact{2}, true)

	renamed, err := left.RenameCycle([]int{0, 1})
	if err != nil {
		t.Fatal(err)
	}
	closeCheckTables(t, renamed)
	requireCheckedFact(t, renamed, Fact{1, 0}, true)
	requireCheckedFact(t, renamed, Fact{2, 1}, true)

	selected, err := left.SelectEqualAndProject(2, 1)
	if err != nil {
		t.Fatal(err)
	}
	closeCheckTables(t, selected)
	requireCheckedFact(t, selected, Fact{1}, true)
}

func TestCheckTableUnionDeltaAndMutatingFilters(t *testing.T) {
	signature := lazySignature(t, 2, 2)
	target := newCheckTableWithFacts(t, signature, Fact{0, 0})
	source := newCheckTableWithFacts(t, signature, Fact{0, 0}, Fact{1, 0}, Fact{1, 1})
	delta := newCheckTableWithFacts(t, signature)
	closeCheckTables(t, target, source, delta)
	if err := target.UnionFrom(source, delta); err != nil {
		t.Fatal(err)
	}
	if targetRows, _ := target.Facts(); len(targetRows) != 3 {
		t.Fatal("union", targetRows)
	}
	if deltaRows, _ := delta.Facts(); len(deltaRows) != 2 {
		t.Fatal("delta", deltaRows)
	}
	if err := target.FilterIdentical([]int{0, 1}); err != nil {
		t.Fatal(err)
	}
	if err := target.FilterEqual(1, 0); err != nil {
		t.Fatal(err)
	}
	requireCheckedFact(t, target, Fact{1, 1}, true)
	if rows, _ := target.Facts(); len(rows) != 1 {
		t.Fatal("filtered rows", rows)
	}
}

func TestCheckTableNegationFilterAndMismatchPrefix(t *testing.T) {
	target := newCheckTableWithFacts(t, lazySignature(t, 2, 3), Fact{0, 0}, Fact{0, 1}, Fact{1, 1}, Fact{1, 2})
	negated := newCheckTableWithFacts(t, lazySignature(t, 3), Fact{1})
	closeCheckTables(t, target, negated)
	if err := target.FilterByNegation(negated, []int{1}, []int{0}); err != nil {
		t.Fatal(err)
	}
	requireCheckedFact(t, target, Fact{0, 0}, true)
	requireCheckedFact(t, target, Fact{1, 2}, true)
	requireCheckedFact(t, target, Fact{0, 1}, false)

	corrupt := newCheckTableWithFacts(t, lazySignature(t, 2), Fact{0})
	closeCheckTables(t, corrupt)
	_, _ = corrupt.primary.Add(Fact{1})
	if _, err := corrupt.Project(nil); !errors.Is(err, ErrCheckTable) {
		t.Fatal("transform did not stop at mismatch prefix", err)
	}
	if corrupt.checkerRowsForTest(t) != 1 {
		t.Fatal("failed transform mutated checker")
	}
}

func (t *CheckTable) checkerRowsForTest(test *testing.T) int {
	test.Helper()
	rows, err := t.checker.Len()
	if err != nil {
		test.Fatal(err)
	}
	return rows
}
