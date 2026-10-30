package rel

import "testing"

func TestSparseIndexedSelectUsesFullNonfunctionalSignature(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{4, 8}, 1, Fact{1, 6}, Fact{2, 7})
	plan, err := NewSparseIndexedSelectPlan(table, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	signature, functional, err := plan.ResultSignature()
	if err != nil || signature.Len() != 1 || functional != 0 {
		t.Fatalf("signature=%v functional=%d err=%v", signature.Domains(), functional, err)
	}
	result, stats, err := plan.Execute(table)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	if !stats.FullSignature {
		t.Fatalf("stats=%+v", stats)
	}
	facts, _ := result.Facts()
	assertSparseKeyFacts(t, facts, []Fact{{6}})
}
