package rel

import "testing"

func TestSparseFunctionalUnionUsesKeyAndDeltaSemantics(t *testing.T) {
	target := newSparseFunctionalFixture(t, []uint64{4, 8}, 1, Fact{1, 2})
	source := newSparseFunctionalFixture(t, []uint64{4, 8}, 1, Fact{1, 7}, Fact{2, 3})
	delta := newSparseFunctionalFixture(t, []uint64{4, 8}, 1)
	inserted, err := target.UnionFunctional(source, delta)
	if err != nil || inserted != 1 {
		t.Fatalf("inserted=%d err=%v", inserted, err)
	}
	if exact, _ := target.ContainsExact(Fact{1, 2}); !exact {
		t.Fatal("existing functional value was replaced")
	}
	if exact, _ := target.ContainsExact(Fact{1, 7}); exact {
		t.Fatal("conflicting source value was inserted")
	}
	if exact, _ := delta.ContainsExact(Fact{2, 3}); !exact {
		t.Fatal("new row missing from delta")
	}
}
