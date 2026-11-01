package rel

import "testing"

func TestSparseFunctionalProjectBoundaryRules(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{4, 4, 4}, 1, Fact{1, 2, 3})
	removedFunctional, err := table.ProjectFunctional([]int{2})
	if err != nil {
		t.Fatal(err)
	}
	defer removedFunctional.Close()
	if functional, _ := removedFunctional.FunctionalColumns(); functional != 0 {
		t.Fatalf("functional after functional removal=%d", functional)
	}
	removedKey, err := table.ProjectFunctional([]int{0})
	if err != nil {
		t.Fatal(err)
	}
	defer removedKey.Close()
	if functional, _ := removedKey.FunctionalColumns(); functional != 0 {
		t.Fatalf("functional after key removal=%d", functional)
	}
	identity, err := table.ProjectFunctional(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer identity.Close()
	if functional, _ := identity.FunctionalColumns(); functional != 1 {
		t.Fatalf("identity functional=%d", functional)
	}
}

func TestSparseFunctionalProjectWithReduceReportsEveryCollision(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{3, 2, 2}, 2,
		Fact{0, 0, 0}, Fact{1, 1, 1}, Fact{2, 0, 1})
	var reductions [][2][]Element
	projected, err := table.ProjectWithReduce([]int{0}, func(existing, incoming []Element) {
		reductions = append(reductions, [2][]Element{
			append([]Element(nil), existing...),
			append([]Element(nil), incoming...),
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	defer projected.Close()
	if functional, err := projected.FunctionalColumns(); err != nil || functional != 2 {
		t.Fatalf("functional columns=%d err=%v", functional, err)
	}
	if rows, err := projected.SizeEstimateRows(); err != nil || rows != 1 {
		t.Fatalf("projected rows=%d err=%v", rows, err)
	}
	facts, err := projected.Facts()
	if err != nil || len(facts) != 1 || facts[0][0] != 0 || facts[0][1] != 0 {
		t.Fatalf("first projected row=%v err=%v", facts, err)
	}
	want := [][2][]Element{
		{{0, 0}, {1, 1}},
		{{0, 0}, {0, 1}},
	}
	if len(reductions) != len(want) {
		t.Fatalf("reducer calls=%v, want %v", reductions, want)
	}
	for i := range want {
		for side := range want[i] {
			got, expected := reductions[i][side], want[i][side]
			if len(got) != len(expected) {
				t.Fatalf("reduction %d side %d=%v, want %v", i, side, got, expected)
			}
			for j := range expected {
				if got[j] != expected[j] {
					t.Fatalf("reduction %d side %d=%v, want %v", i, side, got, expected)
				}
			}
		}
	}
}

func TestSparseFunctionalProjectWithReduceRejectsMissingReducer(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{2, 2}, 1, Fact{0, 0})
	if _, err := table.ProjectWithReduce([]int{0}, nil); err != ErrTable {
		t.Fatalf("nil reducer error=%v, want %v", err, ErrTable)
	}
	if _, err := table.ProjectWithReduce([]int{2}, func(_, _ []Element) {}); err == nil {
		t.Fatal("out-of-range projection column was accepted")
	}
}

func TestSparseFunctionalProjectWithReducePublishesMergedSuffix(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{3, 3}, 1,
		Fact{0, 0}, Fact{1, 2}, Fact{2, 1})
	projected, err := table.ProjectWithReduce([]int{0}, func(existing, incoming []Element) {
		if incoming[0] > existing[0] {
			existing[0] = incoming[0]
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer projected.Close()
	facts, err := projected.Facts()
	if err != nil || len(facts) != 1 || facts[0][0] != 2 {
		t.Fatalf("reduced suffix=%v err=%v, want [[2]]", facts, err)
	}
}
