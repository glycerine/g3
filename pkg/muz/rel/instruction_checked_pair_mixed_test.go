package rel

import "testing"

// The relation manager's default union and negation filter of a check
// table and a table of another plugin.
func TestFiniteCheckedPairMixedDefaultUnionAndNegation(t *testing.T) {
	signature := lazySignature(t, 2)
	template := checkedPairForInstruction(t, signature)
	defer template.Close()
	target, err := template.EmptyLike()
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	delta, err := template.EmptyLike()
	if err != nil {
		t.Fatal(err)
	}
	defer delta.Close()
	source := NewHashTable(signature)
	defer source.Close()
	_, _ = source.Add(Fact{0})
	_, _ = source.Add(Fact{1})
	if err = finiteTableUnion(target, source, delta); err != nil {
		t.Fatal(err)
	}
	targetFacts, err := target.Facts()
	if err != nil {
		t.Fatal(err)
	}
	deltaFacts, err := delta.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, targetFacts, Fact{0}, Fact{1})
	instructionWantFacts(t, deltaFacts, Fact{0}, Fact{1})

	negative := NewHashTable(signature)
	defer negative.Close()
	_, _ = negative.Add(Fact{1})
	if err = finiteTableFilterNegation(target, negative, []int{0}, []int{0}); err != nil {
		t.Fatal(err)
	}
	targetFacts, err = target.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, targetFacts, Fact{0})
}
