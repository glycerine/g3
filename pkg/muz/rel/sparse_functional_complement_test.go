package rel

import "testing"

func TestSparseFunctionalComplementUsesSuppliedSuffix(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{3, 8}, 1, Fact{0, 2}, Fact{1, 7})
	complement, err := table.ComplementFunctional(Fact{2})
	if err != nil {
		t.Fatal(err)
	}
	defer complement.Close()
	if functional, _ := complement.FunctionalColumns(); functional != 1 {
		t.Fatalf("functional=%d", functional)
	}
	for _, fact := range []Fact{{1, 2}, {2, 2}} {
		if exact, _ := complement.ContainsExact(fact); !exact {
			t.Fatalf("complement missing %v", fact)
		}
	}
	if exact, _ := complement.ContainsExact(Fact{0, 2}); exact {
		t.Fatal("present exact fact survived complement")
	}
	constant := newSparseFunctionalFixture(t, []uint64{8}, 1)
	full, err := constant.ComplementFunctional(Fact{4})
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	if exact, _ := full.ContainsExact(Fact{4}); !exact {
		t.Fatal("zero-key complement missing supplied value")
	}
}
