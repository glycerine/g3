package rel_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/muz/rel"
)

func TestHashTableJoinProjectMatchesSourceCompositionAndZeroSignature(t *testing.T) {
	left := rel.NewHashTable(newSignature(t, []uint64{4, 3}))
	right := rel.NewHashTable(newSignature(t, []uint64{3, 2}))
	add(t, left, 1, 0)
	add(t, left, 2, 1)
	add(t, right, 1, 0)
	add(t, right, 1, 1)

	got, err := left.JoinProject(right, []int{1}, []int{0}, []int{0, 1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if signature, err := got.Signature(); err != nil || signature.Len() != 0 {
		t.Fatalf("zero-column join/project signature = %v, %v", signature.Domains(), err)
	}
	if rows := facts(t, got); fmt.Sprint(rows) != "[[]]" {
		t.Fatalf("zero-column join/project rows = %v, want one empty fact", rows)
	}
	joined := join(t, left, right, []int{1}, []int{0})
	composed := project(t, joined, []int{0, 1, 2, 3})
	if err = joined.Close(); err != nil {
		t.Fatal(err)
	}
	if rows := facts(t, composed); fmt.Sprint(rows) != fmt.Sprint(facts(t, got)) {
		t.Fatalf("source composition differs: got=%v composed=%v", facts(t, got), rows)
	}
	if _, err = left.JoinProject(right, []int{1}, []int{0}, []int{4}); !errors.Is(err, rel.ErrTable) {
		t.Fatalf("invalid projection error = %v", err)
	}
}
