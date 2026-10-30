package rel_test

// Source: src/muz/rel/dl_base.cpp and dl_table.cpp retained table operations.

import (
	"errors"
	"fmt"
	"sort"
	"testing"

	"github.com/glycerine/g3/pkg/muz/rel"
)

func newSignature(t *testing.T, domains []uint64) rel.Signature {
	t.Helper()
	value, err := rel.NewSignature(domains)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func clone(t *testing.T, source *rel.HashTable) *rel.HashTable {
	t.Helper()
	value, err := source.Clone()
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func join(t *testing.T, left, right *rel.HashTable, leftColumns, rightColumns []int) *rel.HashTable {
	t.Helper()
	value, err := left.Join(right, leftColumns, rightColumns)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func project(t *testing.T, source *rel.HashTable, removed []int) *rel.HashTable {
	t.Helper()
	value, err := source.Project(removed)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func selectAndProject(t *testing.T, source *rel.HashTable, value rel.Element, column int) *rel.HashTable {
	t.Helper()
	result, err := source.SelectEqualAndProject(value, column)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func rename(t *testing.T, source *rel.HashTable, cycle []int) *rel.HashTable {
	t.Helper()
	result, err := source.RenameCycle(cycle)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func add(t *testing.T, table *rel.HashTable, fact ...rel.Element) {
	t.Helper()
	inserted, err := table.Add(fact)
	if err != nil || !inserted {
		t.Fatal("add", fact, inserted, err)
	}
}

func facts(t *testing.T, table *rel.HashTable) []string {
	t.Helper()
	rows, err := table.Facts()
	if err != nil {
		t.Fatal(err)
	}
	result := make([]string, len(rows))
	for i, row := range rows {
		result[i] = fmt.Sprint(row)
	}
	sort.Strings(result)
	return result
}

func TestHashTableSetCloneAndValidation(t *testing.T) {
	signature := newSignature(t, []uint64{3, 2})
	table := rel.NewHashTable(signature)
	add(t, table, 1, 0)
	add(t, table, 2, 1)
	add(t, table, 0, 1)
	ordered, err := table.Facts()
	if err != nil || fmt.Sprint(ordered) != "[[1 0] [2 1] [0 1]]" {
		t.Fatal("fact insertion order", ordered, err)
	}
	inserted, err := table.Add(rel.Fact{1, 0})
	if err != nil || inserted {
		t.Fatal("duplicate insertion", inserted, err)
	}
	if _, err = table.Add(rel.Fact{3, 0}); !errors.Is(err, rel.ErrTable) {
		t.Fatal("out-of-domain fact accepted", err)
	}
	clone := clone(t, table)
	removed, err := clone.Remove(rel.Fact{1, 0})
	if err != nil || !removed {
		t.Fatal("clone remove", removed, err)
	}
	contained, err := table.Contains(rel.Fact{1, 0})
	if err != nil || !contained {
		t.Fatal("clone aliased source", contained, err)
	}
}

func TestHashTableSourceDefaultSizeEstimates(t *testing.T) {
	table := rel.NewHashTable(newSignature(t, []uint64{3, 2, 4}))
	add(t, table, 1, 0, 2)
	add(t, table, 2, 1, 3)
	if rows, err := table.SizeEstimateRows(); err != nil || rows != ^uint32(0) {
		t.Fatalf("row estimate: %d, %v", rows, err)
	}
	if bytes, err := table.SizeEstimateBytes(); err != nil || bytes != ^uint32(0) {
		t.Fatalf("byte estimate: %d, %v", bytes, err)
	}
	emptySignature := newSignature(t, nil)
	empty := rel.NewHashTable(emptySignature)
	add(t, empty)
	if rows, err := empty.SizeEstimateRows(); err != nil || rows != ^uint32(0) {
		t.Fatalf("zero-arity row estimate: %d, %v", rows, err)
	}
	if bytes, err := empty.SizeEstimateBytes(); err != nil || bytes != ^uint32(0) {
		t.Fatalf("zero-arity byte estimate: %d, %v", bytes, err)
	}
}

func TestHashTableComplementSourceZeroAndOneColumnRules(t *testing.T) {
	zero := newSignature(t, nil)
	emptyZero := rel.NewHashTable(zero)
	zeroComplement, err := emptyZero.Complement()
	if err != nil {
		t.Fatal(err)
	}
	if rows, _ := zeroComplement.Facts(); len(rows) != 1 || len(rows[0]) != 0 {
		t.Fatalf("empty zero-arity complement = %#v", rows)
	}
	if err = emptyZero.Close(); err != nil {
		t.Fatal(err)
	}
	if err = zeroComplement.Close(); err != nil {
		t.Fatal(err)
	}

	zero, _ = rel.NewSignature(nil)
	fullZero := rel.NewHashTable(zero)
	if _, err = fullZero.Add(rel.Fact{}); err != nil {
		t.Fatal(err)
	}
	zeroComplement, err = fullZero.Complement()
	if err != nil {
		t.Fatal(err)
	}
	if rows, _ := zeroComplement.Facts(); len(rows) != 0 {
		t.Fatalf("true zero-arity complement = %#v", rows)
	}
	_ = fullZero.Close()
	_ = zeroComplement.Close()

	single := newSignature(t, []uint64{4})
	input := rel.NewHashTable(single)
	for _, value := range []uint64{1, 3} {
		if _, err = input.Add(rel.Fact{rel.Element(value)}); err != nil {
			t.Fatal(err)
		}
	}
	complement, err := input.Complement()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := complement.Facts()
	if err != nil {
		t.Fatal(err)
	}
	want := []rel.Fact{{0}, {2}}
	if fmt.Sprint(rows) != fmt.Sprint(want) {
		t.Fatalf("one-column complement = %v, want %v", rows, want)
	}
	wide := rel.NewHashTable(newSignature(t, []uint64{2, 2}))
	if _, err = wide.Complement(); !errors.Is(err, rel.ErrTable) {
		t.Fatalf("wide complement error = %v", err)
	}
	_ = input.Close()
	_ = complement.Close()
	_ = wide.Close()
}

func TestHashTableJoinProjectSelectionAndRename(t *testing.T) {
	left := rel.NewHashTable(newSignature(t, []uint64{4, 3}))
	right := rel.NewHashTable(newSignature(t, []uint64{3, 2}))
	add(t, left, 1, 0)
	add(t, left, 2, 1)
	add(t, left, 3, 1)
	add(t, right, 1, 0)
	add(t, right, 1, 1)
	joined := join(t, left, right, []int{1}, []int{0})
	wantJoin := []string{"[2 1 1 0]", "[2 1 1 1]", "[3 1 1 0]", "[3 1 1 1]"}
	if got := facts(t, joined); fmt.Sprint(got) != fmt.Sprint(wantJoin) {
		t.Fatal("join", got, wantJoin)
	}
	projected := project(t, joined, []int{1, 2})
	if got := facts(t, projected); fmt.Sprint(got) != "[[2 0] [2 1] [3 0] [3 1]]" {
		t.Fatal("project", got)
	}
	selected := selectAndProject(t, joined, 2, 0)
	if got := facts(t, selected); fmt.Sprint(got) != "[[1 1 0] [1 1 1]]" {
		t.Fatal("select/project", got)
	}
	renamed := rename(t, projected, []int{0, 1})
	if got := facts(t, renamed); fmt.Sprint(got) != "[[0 2] [0 3] [1 2] [1 3]]" {
		t.Fatal("rename", got)
	}
}

func TestHashTableUnionDeltaAndNegation(t *testing.T) {
	signature := newSignature(t, []uint64{4, 4})
	target := rel.NewHashTable(signature)
	source := rel.NewHashTable(signature)
	delta := rel.NewHashTable(signature)
	add(t, target, 0, 1)
	add(t, source, 0, 1)
	add(t, source, 1, 2)
	if err := target.Union(source, delta); err != nil {
		t.Fatal(err)
	}
	if got := facts(t, delta); fmt.Sprint(got) != "[[1 2]]" {
		t.Fatal("delta", got)
	}
	negated := rel.NewHashTable(newSignature(t, []uint64{4}))
	add(t, negated, 2)
	if err := target.FilterByNegation(negated, []int{1}, []int{0}); err != nil {
		t.Fatal(err)
	}
	if got := facts(t, target); fmt.Sprint(got) != "[[0 1]]" {
		t.Fatal("negation", got)
	}
	nonempty := rel.NewHashTable(newSignature(t, nil))
	add(t, nonempty)
	if err := target.FilterByNegation(nonempty, nil, nil); err != nil {
		t.Fatal(err)
	}
	count, err := target.Len()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("zero-column negation", count)
	}
}
