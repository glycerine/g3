package realclosure

import "testing"

func TestAlgebraicMetadataSignConditionPolynomial(t *testing.T) {
	f := newMultiRootExactFixture(t, true)
	value := f.values[0]
	condition := value.root.signs.conditions[value.root.signIndex]
	for conditionIndex := 0; condition != nil; conditionIndex++ {
		expected := value.root.signs.qs[condition.polynomial].coefficients
		count, err := f.f.store.numSignConditionCoefficients(value, conditionIndex)
		if err != nil || count != len(expected) {
			t.Fatal(conditionIndex, count, len(expected), err)
		}
		for coefficientIndex, want := range expected {
			got, err := f.f.store.signConditionCoefficient(value, conditionIndex, coefficientIndex)
			if err != nil || got != want {
				t.Fatal(conditionIndex, coefficientIndex, got, want, err)
			}
			if err := f.f.store.release(got); err != nil {
				t.Fatal(err)
			}
		}
		condition = condition.previous
	}
}
