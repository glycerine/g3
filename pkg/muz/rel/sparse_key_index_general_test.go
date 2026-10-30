package rel

import "testing"

func TestSparseKeyIndexGeneralGroupsInPhysicalOrder(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{4, 4}, 0,
		Fact{0, 1}, Fact{0, 2}, Fact{1, 2})
	plan, err := NewSparseKeyLookupPlan(table, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, stats, err := plan.Build(table)
	if err != nil {
		t.Fatal(err)
	}
	if stats != (SparseKeyIndexStats{Rows: 3, Groups: 2, KeyColumns: 1}) {
		t.Fatalf("stats=%+v", stats)
	}
	rows, err := snapshot.Lookup(Fact{0})
	if err != nil {
		t.Fatal(err)
	}
	assertSparseKeyFacts(t, rows, []Fact{{0, 1}, {0, 2}})
	groups := snapshot.Groups()
	if len(groups) != 2 || groups[0].Key[0] != 0 || groups[1].Key[0] != 1 {
		t.Fatalf("groups=%v", groups)
	}
}

func TestSparseKeyIndexZeroAndRepeatedColumns(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{4, 4}, 0,
		Fact{0, 1}, Fact{1, 2})
	zero, err := NewSparseKeyLookupPlan(table, nil)
	if err != nil {
		t.Fatal(err)
	}
	zeroSnapshot, stats, err := zero.Build(table)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Groups != 1 || stats.FullSignature {
		t.Fatalf("zero stats=%+v", stats)
	}
	rows, err := zeroSnapshot.Lookup(Fact{})
	if err != nil {
		t.Fatal(err)
	}
	assertSparseKeyFacts(t, rows, []Fact{{0, 1}, {1, 2}})

	repeated, err := NewSparseKeyLookupPlan(table, []int{0, 0})
	if err != nil {
		t.Fatal(err)
	}
	repeatedSnapshot, repeatedStats, err := repeated.Build(table)
	if err != nil {
		t.Fatal(err)
	}
	if repeatedStats.FullSignature {
		t.Fatalf("repeated columns selected full specialization: %+v", repeatedStats)
	}
	found, err := repeatedSnapshot.Contains(Fact{1, 1})
	if err != nil || !found {
		t.Fatalf("repeated lookup=%v, %v", found, err)
	}
}

func assertSparseKeyFacts(t *testing.T, got, want []Fact) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("facts=%v want=%v", got, want)
	}
	for i := range want {
		if factKey(got[i]) != factKey(want[i]) {
			t.Fatalf("facts=%v want=%v", got, want)
		}
	}
}
