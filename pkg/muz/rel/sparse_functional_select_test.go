package rel

import "testing"

func TestSparseFunctionalSelectProjectTracksBoundary(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{4, 4, 8}, 1, Fact{1, 2, 3}, Fact{0, 2, 5})
	keyProjection, err := table.SelectEqualProjectFunctional(1, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer keyProjection.Close()
	if functional, _ := keyProjection.FunctionalColumns(); functional != 0 {
		t.Fatalf("key projection functional=%d", functional)
	}
	if exact, _ := keyProjection.ContainsExact(Fact{2, 3}); !exact {
		t.Fatal("selected key row missing")
	}
	valueProjection, err := table.SelectEqualProjectFunctional(3, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer valueProjection.Close()
	if functional, _ := valueProjection.FunctionalColumns(); functional != 0 {
		t.Fatalf("value projection functional=%d", functional)
	}
	if exact, _ := valueProjection.ContainsExact(Fact{1, 2}); !exact {
		t.Fatal("selected value row missing")
	}
}
