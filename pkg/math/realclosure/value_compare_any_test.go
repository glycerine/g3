package realclosure

import (
	"sort"
	"testing"
)

func TestAnyDeterminedCompareAndPredicates(t *testing.T) {
	fixture := newMultiRootExactFixture(t, false)
	comparisons := make([]int, 0, 3)
	for _, root := range fixture.roots {
		comparison, err := fixture.f.store.compareAnyDeterminedValues(root, fixture.two, false)
		if err != nil {
			t.Fatal(err)
		}
		comparisons = append(comparisons, comparison)
	}
	sort.Ints(comparisons)
	if comparisons[0] != -1 || comparisons[1] != 0 || comparisons[2] != 1 {
		t.Fatal(comparisons)
	}
	lessCount, equalCount, greaterCount := 0, 0, 0
	for _, root := range fixture.roots {
		less, err := fixture.f.store.lessAnyDeterminedValues(root, fixture.two, false)
		if err != nil {
			t.Fatal(err)
		}
		equal, err := fixture.f.store.equalAnyDeterminedValues(root, fixture.two, false)
		if err != nil {
			t.Fatal(err)
		}
		greater, err := fixture.f.store.greaterAnyDeterminedValues(root, fixture.two, false)
		if err != nil {
			t.Fatal(err)
		}
		if less {
			lessCount++
		}
		if equal {
			equalCount++
		}
		if greater {
			greaterCount++
		}
	}
	if lessCount != 1 || equalCount != 1 || greaterCount != 1 {
		t.Fatal(lessCount, equalCount, greaterCount)
	}
}
