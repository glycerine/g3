package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func checkedPairForInstruction(t *testing.T, signature Signature, facts ...Fact) *FiniteCheckedPair {
	t.Helper()
	primary := NewHashTable(signature)
	for _, fact := range facts {
		if _, err := primary.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	pair, err := NewFiniteCheckedPair(primary, "sparse")
	if err != nil {
		t.Fatal(err)
	}
	if err = primary.Close(); err != nil {
		t.Fatal(err)
	}
	return pair
}

func TestFiniteCheckedPairCompiledInstructionDispatch(t *testing.T) {
	f := newTableRelationFixture(t)
	context, err := NewExecutionContextWithRelations(f.relations, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = context.Close() })
	left := checkedPairForInstruction(t, lazySignature(t, 2, 2), Fact{0, 0}, Fact{1, 1})
	right := checkedPairForInstruction(t, lazySignature(t, 2), Fact{1})
	if err = context.SetFiniteRegister(0, left); err != nil {
		t.Fatal(err)
	}
	if err = context.SetFiniteRegister(1, right); err != nil {
		t.Fatal(err)
	}
	joinProject := NewJoinProjectInstruction(0, 1, []int{1}, []int{0}, []int{1, 2}, 2)
	t.Cleanup(func() { _ = joinProject.Close() })
	complete, err := joinProject.Perform(context)
	if err != nil || !complete {
		t.Fatal("checked join-project", complete, err)
	}
	result, err := context.FiniteRegister(2)
	if err != nil {
		t.Fatal(err)
	}
	pair, ok := result.(*FiniteCheckedPair)
	if !ok {
		t.Fatalf("compiled checked result type %T", result)
	}
	facts, err := pair.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, facts, Fact{1})

	empty, err := pair.EmptyLike()
	if err != nil {
		t.Fatal(err)
	}
	if err = setNonemptyFiniteRegister(context, 3, empty); err != nil {
		t.Fatal(err)
	}
	storedEmpty, err := context.FiniteRegister(3)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok = storedEmpty.(*FiniteCheckedPair); !ok {
		t.Fatalf("empty checked owner discarded: %T", storedEmpty)
	}

	filtered := checkedPairForInstruction(t, lazySignature(t, 2), Fact{1})
	if err = context.SetFiniteRegister(4, filtered); err != nil {
		t.Fatal(err)
	}
	filter, err := NewFilterInterpretedInstruction(f.ast, f.relations, tableRelationTheoryRewriter(t, f.ast), 4, []ast.ASTHandle{f.falsehood}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = filter.Close() })
	complete, err = filter.Perform(context)
	if err != nil || !complete {
		t.Fatal("checked interpreted filter", complete, err)
	}
	filteredEmpty, err := context.FiniteRegister(4)
	if err != nil {
		t.Fatal(err)
	}
	if checked, ok := filteredEmpty.(*FiniteCheckedPair); !ok {
		t.Fatalf("interpreted filter discarded checked owner: %T", filteredEmpty)
	} else if count, countErr := checked.Len(); countErr != nil || count != 0 {
		t.Fatal("interpreted checked empty", count, countErr)
	}

	rows, bytes, err := finiteTableSizeEstimates(pair)
	if err != nil || rows != 1 || bytes == 0 {
		t.Fatal("checked size estimates", rows, bytes, err)
	}
}
