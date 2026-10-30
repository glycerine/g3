package rel

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func closeUDoc(t *testing.T, relations ...*UDocRelation) {
	t.Helper()
	t.Cleanup(func() {
		if err := closeRelations(relations...); err != nil {
			t.Error(err)
		}
	})
}

func requireFact(t *testing.T, relation *UDocRelation, fact []uint64, want bool) {
	t.Helper()
	got, err := relation.ContainsFact(fact)
	if err != nil || got != want {
		t.Fatalf("fact %v: got %v want %v: %v", fact, got, want, err)
	}
}

func TestUDocFactsCloneComplementAndMetadata(t *testing.T) {
	manager, _ := newTestDocManager(t, 5)
	relation, err := NewUDocRelation(manager, []uint32{2, 3})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, relation)
	if relation.NumBits() != 5 || relation.NumColumns() != 2 {
		t.Fatal("shape", relation.NumBits(), relation.NumColumns())
	}
	if index, err := relation.ColumnIndex(1); err != nil || index != 2 {
		t.Fatal("column index", index, err)
	}
	if width, err := relation.ColumnNumBits(1); err != nil || width != 3 {
		t.Fatal("column width", width, err)
	}
	inserted, err := relation.AddFact([]uint64{2, 5})
	if err != nil || !inserted {
		t.Fatal("insert", inserted, err)
	}
	inserted, err = relation.AddFact([]uint64{2, 5})
	if err != nil || inserted {
		t.Fatal("duplicate", inserted, err)
	}
	requireFact(t, relation, []uint64{2, 5}, true)
	requireFact(t, relation, []uint64{2, 4}, false)
	if _, err = relation.AddFact([]uint64{4, 0}); !errors.Is(err, ErrDOC) {
		t.Fatal("out-of-width fact", err)
	}

	clone, err := relation.Clone()
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, clone)
	requireFact(t, clone, []uint64{2, 5}, true)
	if err = relation.Reset(); err != nil || !relation.FastEmpty() {
		t.Fatal("reset", relation.FastEmpty(), err)
	}
	requireFact(t, clone, []uint64{2, 5}, true)

	complement, err := clone.Complement()
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, complement)
	requireFact(t, complement, []uint64{2, 5}, false)
	// Source contains_fact is a conservative DOC-containment query and does not
	// prove an exact point is outside a negative cube. Check semantic complement
	// exhaustively through the independent finite oracle instead.
	assertUnionTruth(t, complement.elems, func(assignment uint32) bool { return assignment != 2|(5<<2) })
}

func TestUDocProjectRenameAndExpandedColumns(t *testing.T) {
	manager, _ := newTestDocManager(t, 5)
	relation, err := NewUDocRelation(manager, []uint32{2, 3})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, relation)
	if _, err = relation.AddFact([]uint64{2, 5}); err != nil {
		t.Fatal(err)
	}
	if bits, err := relation.ExpandColumns([]uint32{1, 0}, nil); err != nil || len(bits) != 5 || bits[0] != 2 || bits[3] != 0 {
		t.Fatal("expanded columns", bits, err)
	}

	projectedManager, _ := newTestDocManager(t, 3)
	projected, err := relation.Project([]uint32{0}, projectedManager)
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, projected)
	if projected.NumColumns() != 1 || projected.NumBits() != 3 {
		t.Fatal("projected shape", projected.NumColumns(), projected.NumBits())
	}
	requireFact(t, projected, []uint64{5}, true)
	requireFact(t, projected, []uint64{4}, false)

	renamed, err := relation.Rename([]uint32{0, 1})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, renamed)
	if width, _ := renamed.ColumnNumBits(0); width != 3 {
		t.Fatal("renamed first width", width)
	}
	if width, _ := renamed.ColumnNumBits(1); width != 2 {
		t.Fatal("renamed second width", width)
	}
	requireFact(t, renamed, []uint64{5, 2}, true)
	if _, err = relation.Rename([]uint32{0, 0}); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("duplicate cycle", err)
	}
}

func TestUDocUnionDeltaAndShapeChecks(t *testing.T) {
	manager, _ := newTestDocManager(t, 3)
	target, err := NewUDocRelation(manager, []uint32{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	source, _ := NewUDocRelation(manager, []uint32{1, 2})
	delta, _ := NewUDocRelation(manager, []uint32{1, 2})
	closeUDoc(t, target, source, delta)
	_, _ = target.AddFact([]uint64{0, 1})
	_, _ = source.AddFact([]uint64{0, 1})
	_, _ = source.AddFact([]uint64{1, 3})
	if err = target.UnionFrom(source, delta); err != nil {
		t.Fatal(err)
	}
	if target.NumRecords() != 2 || delta.NumRecords() != 1 {
		t.Fatal("union/delta sizes", target.NumRecords(), delta.NumRecords())
	}
	requireFact(t, delta, []uint64{1, 3}, true)
	requireFact(t, delta, []uint64{0, 1}, false)

	wrong, err := NewUDocRelation(manager, []uint32{2, 1})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, wrong)
	if err = target.UnionFrom(wrong, nil); !errors.Is(err, ErrDOC) {
		t.Fatal("mismatched widths", err)
	}
}

func TestUDocJoinUsesWholeColumnEquality(t *testing.T) {
	leftManager, _ := newTestDocManager(t, 2)
	rightManager, _ := newTestDocManager(t, 2)
	joinManager, _ := newTestDocManager(t, 4)
	left, err := NewUDocRelation(leftManager, []uint32{2})
	if err != nil {
		t.Fatal(err)
	}
	right, _ := NewUDocRelation(rightManager, []uint32{2})
	closeUDoc(t, left, right)
	_, _ = left.AddFact([]uint64{1})
	_, _ = left.AddFact([]uint64{2})
	_, _ = right.AddFact([]uint64{1})
	_, _ = right.AddFact([]uint64{3})
	joined, err := left.Join(right, []uint32{0}, []uint32{0}, joinManager)
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, joined)
	if joined.NumRecords() != 1 {
		t.Fatal("joined records", joined.NumRecords())
	}
	requireFact(t, joined, []uint64{1, 1}, true)
	requireFact(t, joined, []uint64{2, 3}, false)

	if _, err = left.Join(right, []uint32{0}, nil, joinManager); !errors.Is(err, ErrDOC) {
		t.Fatal("join column arity", err)
	}
}

func TestUDocCyclicJoinThenProject(t *testing.T) {
	leftManager, _ := newTestDocManager(t, 4)
	rightManager, _ := newTestDocManager(t, 4)
	productManager, _ := newTestDocManager(t, 8)
	projectedManager, _ := newTestDocManager(t, 4)
	left, err := NewUDocRelation(leftManager, []uint32{2, 2})
	if err != nil {
		t.Fatal(err)
	}
	right, err := NewUDocRelation(rightManager, []uint32{2, 2})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, left, right)
	if _, err = left.AddFact([]uint64{3, 1}); err != nil {
		t.Fatal(err)
	}
	for _, fact := range [][]uint64{{0, 3}, {1, 3}, {1, 2}} {
		if _, err = right.AddFact(fact); err != nil {
			t.Fatal(err)
		}
	}
	joined, err := left.Join(right, []uint32{0, 1}, []uint32{1, 0}, productManager)
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, joined)
	projected, err := joined.Project([]uint32{0, 1}, projectedManager)
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, projected)
	requireFact(t, projected, []uint64{1, 3}, true)
	requireFact(t, projected, []uint64{0, 3}, false)
	if projected.NumRecords() != 1 {
		t.Fatalf("cyclic join projection records: %d", projected.NumRecords())
	}
}

func TestUDocFilterEqualRetainsMatchingRows(t *testing.T) {
	manager, _ := newTestDocManager(t, 4)
	relation, err := NewUDocRelation(manager, []uint32{2, 2})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, relation)
	for _, fact := range [][]uint64{{0, 1}, {1, 2}, {1, 3}} {
		if inserted, x := relation.AddFact(fact); x != nil || !inserted {
			t.Fatal("add row", fact, inserted, x)
		}
	}
	if err = relation.FilterEqual(0, 1); err != nil {
		t.Fatal(err)
	}
	if relation.NumRecords() != 2 {
		t.Fatal("filtered row count", relation.NumRecords())
	}
	requireFact(t, relation, []uint64{0, 1}, false)
	requireFact(t, relation, []uint64{1, 2}, true)
	requireFact(t, relation, []uint64{1, 3}, true)
	if err = relation.FilterEqual(2, 0); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("out-of-range column", err)
	}
	if err = relation.FilterEqual(0, 4); !errors.Is(err, ErrDOC) {
		t.Fatal("out-of-domain value", err)
	}
	if relation.NumRecords() != 2 {
		t.Fatal("invalid filters changed relation", relation.NumRecords())
	}
}

func TestUDocFilterEqualBigPreservesWideValues(t *testing.T) {
	manager, _ := newTestDocManager(t, 70)
	relation, err := NewUDocRelation(manager, []uint32{70})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, relation)
	wanted := new(big.Int).Lsh(big.NewInt(1), 69)
	wanted.Add(wanted, big.NewInt(7))
	other := new(big.Int).Sub(new(big.Int).Set(wanted), big.NewInt(1))
	for _, value := range []*big.Int{other, wanted} {
		if inserted, x := relation.AddBigFact([]*big.Int{value}); x != nil || !inserted {
			t.Fatal("add wide row", value, inserted, x)
		}
	}
	if err = relation.FilterEqualBig(0, wanted); err != nil {
		t.Fatal(err)
	}
	if relation.NumRecords() != 1 {
		t.Fatal("wide filtered row count", relation.NumRecords())
	}
	expected, err := manager.vectors.AllocateBigInt(wanted)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = manager.vectors.Deallocate(expected) }()
	equal, err := manager.vectors.Equals(relation.elems.values[0].positive, expected)
	if err != nil || !equal {
		t.Fatal("wide filter selected wrong value", equal, err)
	}
}

func TestUDocFilterIdenticalColumns(t *testing.T) {
	manager, _ := newTestDocManager(t, 4)
	relation, err := NewUDocRelation(manager, []uint32{2, 2})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, relation)
	for _, fact := range [][]uint64{{0, 0}, {1, 1}, {0, 1}, {2, 3}, {3, 3}} {
		if _, e := relation.AddFact(fact); e != nil {
			t.Fatal(e)
		}
	}
	if err = relation.FilterIdentical([]uint32{0, 1}); err != nil {
		t.Fatal(err)
	}
	for _, fact := range [][]uint64{{0, 0}, {1, 1}, {0, 1}, {2, 3}, {3, 3}} {
		requireFact(t, relation, fact, fact[0] == fact[1])
	}
	count := relation.NumRecords()
	if err = relation.FilterIdentical([]uint32{0, 2}); err == nil {
		t.Fatal("out-of-range column accepted")
	}
	if relation.NumRecords() != count {
		t.Fatal("invalid filter mutated relation")
	}

	unequalManager, _ := newTestDocManager(t, 3)
	unequal, err := NewUDocRelation(unequalManager, []uint32{2, 1})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, unequal)
	if _, err = unequal.AddFact([]uint64{1, 1}); err != nil {
		t.Fatal(err)
	}
	if err = unequal.FilterIdentical([]uint32{0, 1}); err == nil {
		t.Fatal("different-width columns accepted")
	}
	requireFact(t, unequal, []uint64{1, 1}, true)
}

func TestUDocFilterByNegationFastAndJoinProjectPaths(t *testing.T) {
	manager, _ := newTestDocManager(t, 2)
	target, err := NewUDocRelation(manager, []uint32{2})
	if err != nil {
		t.Fatal(err)
	}
	negative, err := NewUDocRelation(manager, []uint32{2})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, target, negative)
	for value := uint64(0); value < 4; value++ {
		_, _ = target.AddFact([]uint64{value})
	}
	_, _ = negative.AddFact([]uint64{1})
	_, _ = negative.AddFact([]uint64{3})
	if err = target.FilterByNegation(negative, []uint32{0}, []uint32{0}, nil); err != nil {
		t.Fatal("direct subtraction path", err)
	}
	if target.NumRecords() != 2 {
		t.Fatal("direct subtraction count", target.NumRecords())
	}
	requireFact(t, target, []uint64{0}, true)
	requireFact(t, target, []uint64{1}, false)
	requireFact(t, target, []uint64{2}, true)
	requireFact(t, target, []uint64{3}, false)

	targetManager, _ := newTestDocManager(t, 4)
	negativeManager, _ := newTestDocManager(t, 2)
	productManager, _ := newTestDocManager(t, 6)
	wideTarget, err := NewUDocRelation(targetManager, []uint32{2, 2})
	if err != nil {
		t.Fatal(err)
	}
	wideNegative, err := NewUDocRelation(negativeManager, []uint32{2})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, wideTarget, wideNegative)
	for _, fact := range [][]uint64{{0, 0}, {0, 1}, {1, 0}, {1, 1}} {
		_, _ = wideTarget.AddFact(fact)
	}
	_, _ = wideNegative.AddFact([]uint64{1})
	if err = wideTarget.FilterByNegation(wideNegative, []uint32{1}, []uint32{0}, productManager); err != nil {
		t.Fatal("join/project subtraction path", err)
	}
	if wideTarget.NumRecords() != 2 {
		t.Fatal("join/project subtraction count", wideTarget.NumRecords())
	}
	requireFact(t, wideTarget, []uint64{0, 0}, true)
	requireFact(t, wideTarget, []uint64{1, 0}, true)
	requireFact(t, wideTarget, []uint64{0, 1}, false)
	requireFact(t, wideTarget, []uint64{1, 1}, false)
}
