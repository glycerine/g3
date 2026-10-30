package rel

import (
	"errors"
	"testing"
)

func TestSparseIndexedSelectRejectsUnsupportedIndexesAndDrift(t *testing.T) {
	one := newSparseFunctionalFixture(t, []uint64{4}, 0)
	if _, err := NewSparseIndexedSelectPlan(one, 1, 0); !errors.Is(err, ErrTable) {
		t.Fatalf("unary table: %v", err)
	}
	table := newSparseFunctionalFixture(t, []uint64{4, 8}, 1)
	for _, tc := range []struct {
		value  Element
		column int
	}{{0, -1}, {0, 1}, {0, 2}} {
		if _, err := NewSparseIndexedSelectPlan(table, tc.value, tc.column); !errors.Is(err, ErrTable) {
			t.Fatalf("case=%+v err=%v", tc, err)
		}
	}
	outOfDomain, err := NewSparseIndexedSelectPlan(table, 4, 0)
	if err != nil {
		t.Fatal(err)
	}
	result, stats, err := outOfDomain.Execute(table)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	if count, _ := result.Len(); count != 0 || stats.Matches != 0 {
		t.Fatalf("out-of-domain select count=%d stats=%+v", count, stats)
	}
	plan, err := NewSparseIndexedSelectPlan(table, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	wrong := newSparseFunctionalFixture(t, []uint64{5, 8}, 1)
	if _, _, err := plan.Execute(wrong); !errors.Is(err, ErrTable) {
		t.Fatalf("schema drift: %v", err)
	}
	if _, _, err := (*SparseIndexedSelectPlan)(nil).Execute(table); !errors.Is(err, ErrTable) {
		t.Fatalf("nil plan: %v", err)
	}
}

func TestSparseIndexedSelectCloneOwnsSignatures(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{4, 4}, 0)
	plan, err := NewSparseIndexedSelectPlan(table, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	clone := plan.Clone()
	signature, _, err := clone.ResultSignature()
	if err != nil {
		t.Fatal(err)
	}
	signature.domains[0] = 99
	unchanged, _, _ := plan.ResultSignature()
	value, column, err := clone.Selection()
	if err != nil || value != 2 || column != 1 || unchanged.domains[0] != 4 {
		t.Fatalf("selection=%d/%d signature=%v err=%v", value, column, unchanged.Domains(), err)
	}
}
