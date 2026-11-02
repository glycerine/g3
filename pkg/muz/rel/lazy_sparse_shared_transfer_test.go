package rel

import "testing"

func TestLazySparseMutatingForceClonesSharedSourceOwner(t *testing.T) {
	engine := NewLazySparseEngine()
	defer engine.Close()
	source, err := engine.New(lazySignature(t, 2), 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = source.Add(Fact{0})
	_, _ = source.Add(Fact{1})
	filtered := engine.adopt(&lazySparseNode{
		kind: LazyTableFilterEqual, signature: Signature{domains: source.root.signature.Domains()},
		left: source.root, value: 1, column: 0,
	})
	filteredFacts, err := filtered.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, filteredFacts, Fact{1})
	sourceFacts, err := source.Facts()
	if err != nil {
		t.Fatal("shared source lost its packed owner", err)
	}
	instructionWantFacts(t, sourceFacts, Fact{0}, Fact{1})
}
