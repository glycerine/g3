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

func TestSparseFunctionalUnionUsesZeroColumnKey(t *testing.T) {
	target := newSparseFunctionalFixture(t, []uint64{8}, 1)
	source := newSparseFunctionalFixture(t, []uint64{8}, 1, Fact{4})
	delta := newSparseFunctionalFixture(t, []uint64{8}, 1)
	inserted, err := target.UnionFunctional(source, delta)
	if err != nil || inserted != 1 {
		t.Fatalf("first zero-key union inserted=%d err=%v", inserted, err)
	}
	if exact, cause := target.ContainsExact(Fact{4}); cause != nil || !exact {
		t.Fatal("zero-key target row", exact, cause)
	}
	if exact, cause := delta.ContainsExact(Fact{4}); cause != nil || !exact {
		t.Fatal("zero-key delta row", exact, cause)
	}

	conflict := newSparseFunctionalFixture(t, []uint64{8}, 1, Fact{7})
	emptyDelta := newSparseFunctionalFixture(t, []uint64{8}, 1)
	inserted, err = target.UnionFunctional(conflict, emptyDelta)
	if err != nil || inserted != 0 {
		t.Fatalf("conflicting zero-key union inserted=%d err=%v", inserted, err)
	}
	if exact, cause := target.ContainsExact(Fact{4}); cause != nil || !exact {
		t.Fatal("conflicting union replaced target suffix", exact, cause)
	}
	if count, cause := emptyDelta.Len(); cause != nil || count != 0 {
		t.Fatal("conflicting zero-key row entered delta", count, cause)
	}
}
