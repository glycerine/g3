package rel

import "testing"

func newJoinProjectUDoc(t *testing.T, manager *DocManager, widths []uint32) *UDocRelation {
	t.Helper()
	r, err := NewUDocRelation(manager, widths)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func addJoinProjectFact(t *testing.T, r *UDocRelation, fact []uint64) {
	t.Helper()
	if _, err := r.AddFact(fact); err != nil {
		t.Fatal(err)
	}
}

func TestUDocJoinProjectMatchesComposition(t *testing.T) {
	leftDM, _ := newTestDocManager(t, 3)
	rightDM, _ := newTestDocManager(t, 3)
	productDM, _ := newTestDocManager(t, 6)
	resultDM, _ := newTestDocManager(t, 3)
	left := newJoinProjectUDoc(t, leftDM, []uint32{1, 2})
	right := newJoinProjectUDoc(t, rightDM, []uint32{2, 1})
	closeUDoc(t, left, right)
	for _, fact := range [][]uint64{{0, 1}, {1, 2}, {1, 3}} {
		addJoinProjectFact(t, left, fact)
	}
	for _, fact := range [][]uint64{{1, 0}, {2, 1}, {3, 0}} {
		addJoinProjectFact(t, right, fact)
	}
	fused, err := left.JoinProject(right, []uint32{1}, []uint32{0}, []uint32{0, 2}, productDM, resultDM)
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, fused)
	joined, err := left.Join(right, []uint32{1}, []uint32{0}, productDM)
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, joined)
	composed, err := joined.Project([]uint32{0, 2}, resultDM)
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, composed)
	for x := uint64(0); x < 4; x++ {
		for y := uint64(0); y < 2; y++ {
			fact := []uint64{x, y}
			got, e := fused.ContainsFact(fact)
			if e != nil {
				t.Fatal(e)
			}
			want, e := composed.ContainsFact(fact)
			if e != nil || got != want {
				t.Fatalf("fact %v: fused %v composed %v err %v", fact, got, want, e)
			}
		}
	}
	for _, fact := range [][]uint64{{1, 0}, {2, 1}, {3, 0}} {
		requireFact(t, fused, fact, true)
	}
}

func TestUDocJoinProjectRepeatedKeysAndNullary(t *testing.T) {
	leftDM, _ := newTestDocManager(t, 2)
	rightDM, _ := newTestDocManager(t, 4)
	productDM, _ := newTestDocManager(t, 6)
	resultDM, _ := newTestDocManager(t, 2)
	left := newJoinProjectUDoc(t, leftDM, []uint32{2})
	right := newJoinProjectUDoc(t, rightDM, []uint32{2, 2})
	closeUDoc(t, left, right)
	addJoinProjectFact(t, left, []uint64{1})
	for _, fact := range [][]uint64{{1, 1}, {1, 2}, {2, 2}} {
		addJoinProjectFact(t, right, fact)
	}
	projected, err := left.JoinProject(right, []uint32{0, 0}, []uint32{0, 1}, []uint32{0, 1}, productDM, resultDM)
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, projected)
	requireFact(t, projected, []uint64{1}, true)
	requireFact(t, projected, []uint64{2}, false)

	nullDM, _ := newTestDocManager(t, 0)
	nullary, err := left.JoinProject(right, nil, nil, []uint32{0, 1, 2}, productDM, nullDM)
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, nullary)
	requireFact(t, nullary, nil, true)
	if nullary.NumRecords() != 1 {
		t.Fatal("nullary Cartesian projection must deduplicate", nullary.NumRecords())
	}
}

func TestUDocJoinProjectIntersectionAndDuplicateGuard(t *testing.T) {
	manager, _ := newTestDocManager(t, 4)
	productDM, _ := newTestDocManager(t, 8)
	left := newJoinProjectUDoc(t, manager, []uint32{2, 2})
	right := newJoinProjectUDoc(t, manager, []uint32{2, 2})
	closeUDoc(t, left, right)
	for _, fact := range [][]uint64{{0, 1}, {1, 2}} {
		addJoinProjectFact(t, left, fact)
	}
	for _, fact := range [][]uint64{{1, 2}, {1, 3}} {
		addJoinProjectFact(t, right, fact)
	}
	intersection, err := left.JoinProject(right, []uint32{0, 1}, []uint32{0, 1}, []uint32{0, 1}, productDM, manager)
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, intersection)
	requireFact(t, intersection, []uint64{1, 2}, true)
	requireFact(t, intersection, []uint64{1, 3}, false)

	// The source shortcut's syntactic check also accepts duplicate join columns;
	// use the general streamed path so the unconstrained column remains free.
	duplicate, err := left.JoinProject(right, []uint32{0, 0}, []uint32{0, 0}, []uint32{0, 1}, productDM, manager)
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, duplicate)
	requireFact(t, duplicate, []uint64{1, 2}, true)
	requireFact(t, duplicate, []uint64{1, 3}, true)
	if left.NumRecords() != 2 || right.NumRecords() != 2 {
		t.Fatal("source mutation", left.NumRecords(), right.NumRecords())
	}
}
