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
