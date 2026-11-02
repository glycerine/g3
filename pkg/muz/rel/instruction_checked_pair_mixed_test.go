package rel

import "testing"

func TestFiniteCheckedPairMixedInstructionDispatchPreservesChecking(t *testing.T) {
	signature := lazySignature(t, 2)
	pair := checkedPairForInstruction(t, signature, Fact{1})
	defer pair.Close()
	plain := NewHashTable(signature)
	defer plain.Close()
	_, _ = plain.Add(Fact{1})

	for _, pairFirst := range []bool{true, false} {
		first, second := FiniteTable(pair), FiniteTable(plain)
		if !pairFirst {
			first, second = second, first
		}
		joined, err := finiteTableJoin(first, second, []int{0}, []int{0})
		if err != nil {
			t.Fatal(err)
		}
		checked, ok := joined.(*FiniteCheckedPair)
		if !ok {
			t.Fatalf("mixed join lost checking: %T", joined)
		}
		facts, err := checked.Facts()
		if err != nil {
			t.Fatal(err)
		}
		instructionWantFacts(t, facts, Fact{1, 1})
		if err = checked.Close(); err != nil {
			t.Fatal(err)
		}
	}

	nullary, err := finiteTableJoinProject(pair, plain, []int{0}, []int{0}, []int{0, 1})
	if err != nil {
		t.Fatal(err)
	}
	checkedNullary, ok := nullary.(*FiniteCheckedPair)
	if !ok {
		t.Fatalf("mixed nullary join-project lost checking: %T", nullary)
	}
	facts, err := checkedNullary.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, facts, Fact{})
	primary, checker, err := checkedNullary.ConcreteKinds()
	if err != nil || primary != "hashtable" || checker != "hashtable" {
		t.Fatal("nullary Sparse fallback", primary, checker, err)
	}
	if err = checkedNullary.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestFiniteCheckedPairMixedUnionAndNegationLiftPlainTables(t *testing.T) {
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
