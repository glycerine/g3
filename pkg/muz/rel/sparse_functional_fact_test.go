package rel

import "testing"

func TestSparseFunctionalFetchSuggestEnsureSourceSemantics(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{4, 8}, 1, Fact{1, 2})
	if exact, err := table.ContainsExact(Fact{1, 3}); err != nil || exact {
		t.Fatalf("exact=%v err=%v", exact, err)
	}
	probe := Fact{1, 7}
	if found, err := table.FetchFact(probe); err != nil || !found || probe[1] != 2 {
		t.Fatalf("fetch=%v fact=%v err=%v", found, probe, err)
	}
	suggestion := Fact{1, 6}
	if added, err := table.SuggestFact(suggestion); err != nil || added || suggestion[1] != 2 {
		t.Fatalf("suggest=%v fact=%v err=%v", added, suggestion, err)
	}
	if err := table.EnsureFact(Fact{1, 5}); err != nil {
		t.Fatal(err)
	}
	if exact, _ := table.ContainsExact(Fact{1, 5}); !exact {
		t.Fatal("ensured functional value missing")
	}
	if exact, _ := table.ContainsExact(Fact{1, 2}); exact {
		t.Fatal("old functional value survived")
	}
}

func TestSparseFunctionalEnsurePreservesPhysicalRowOrder(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{8, 16}, 1,
		Fact{1, 2}, Fact{2, 3}, Fact{3, 4})
	if err := table.EnsureFact(Fact{1, 9}); err != nil {
		t.Fatal(err)
	}
	facts, err := table.Facts()
	if err != nil {
		t.Fatal(err)
	}
	want := []Fact{{1, 9}, {2, 3}, {3, 4}}
	if len(facts) != len(want) {
		t.Fatalf("fact count=%d, want %d: %v", len(facts), len(want), facts)
	}
	for i := range want {
		if len(facts[i]) != len(want[i]) || facts[i][0] != want[i][0] || facts[i][1] != want[i][1] {
			t.Fatalf("physical facts=%v, want %v", facts, want)
		}
	}
}
