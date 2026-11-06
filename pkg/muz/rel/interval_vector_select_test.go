package rel

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestIntervalVectorSelectEqualProjectUsesSourceCloneSemantics(t *testing.T) {
	source := NewIntervalFull(2)
	if err := source.values.intersect(0, intervalClosed(1, 5)); err != nil {
		t.Fatal(err)
	}
	if err := source.FilterIdentical(0, 1); err != nil {
		t.Fatal(err)
	}
	selected, err := source.SelectEqualAndProject(big.NewRat(3, 1), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected.values.columns) != 1 || selected.Empty() {
		t.Fatalf("selected arity=%d empty=%v", len(selected.values.columns), selected.Empty())
	}
	// vector_relation::copy puts each representative's value into a fresh
	// independent column. Filtering the removed clone column therefore does
	// not narrow the surviving former-equal column.
	intervalIntersectionBound(t, selected, 0, 1, 5)
	if source.values.find(0) != source.values.find(1) {
		t.Fatal("selection mutated source equivalence")
	}
	intervalIntersectionBound(t, source, 0, 1, 5)
}

func TestIntervalVectorSelectEqualProjectEmptyInputFollowsSourceQuirkAndValidation(t *testing.T) {
	empty := NewIntervalEmpty(2)
	selected, err := empty.SelectEqualAndProject(big.NewRat(2, 1), 1)
	if err != nil || selected == nil || selected.Empty() || !selected.IsFull() || len(selected.values.columns) != 1 {
		t.Fatalf("empty selection=%v err=%v", selected, err)
	}

	source := NewIntervalFull(1)
	if _, err := source.SelectEqualAndProject(nil, 0); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatalf("nil value error=%v", err)
	}
	if _, err := source.SelectEqualAndProject(big.NewRat(0, 1), 1); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatalf("invalid column error=%v", err)
	}
	lo, _, hi, _, err := source.Bounds(0)
	if err != nil || lo != nil || hi != nil {
		t.Fatalf("failed selection mutated source: lo=%v hi=%v err=%v", lo, hi, err)
	}
}
