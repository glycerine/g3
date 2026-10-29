package realclosure

import "testing"

func TestAlgebraicMetadataSignConditionChain(t *testing.T) {
	f := newMultiRootExactFixture(t, true)
	value := f.values[0]
	head := value.root.signs.conditions[value.root.signIndex]
	wantCount := 0
	for current := head; current != nil; current = current.previous {
		wantCount++
	}
	count, err := f.f.store.numSignConditions(value)
	if err != nil || count != wantCount {
		t.Fatal(count, wantCount, err)
	}
	current := head
	for i := 0; i < count; i++ {
		sign, err := f.f.store.signConditionSign(value, i)
		if err != nil || sign != current.sign {
			t.Fatal(i, sign, current.sign, err)
		}
		current = current.previous
	}
	if sign, err := f.f.store.signConditionSign(value, count); err != nil || sign != 0 {
		t.Fatal(sign, err)
	}
}
