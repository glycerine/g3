package rel

import (
	"math/big"
	"testing"
)

// Both numeric plugins accept the zero-column signature. The source vector
// relation represents its full relation with the single empty tuple; its
// project transformer also starts from full and does not copy the source
// empty flag.
func TestNumericRelationNullarySourceBehavior(t *testing.T) {
	t.Run("bound", func(t *testing.T) {
		full, empty := NewBoundRelation(0, false), NewBoundRelation(0, true)
		if full.Empty() || !full.ContainsFact(nil) || !empty.Empty() || empty.ContainsFact(nil) {
			t.Fatal("nullary full/empty fact semantics", full.Empty(), empty.Empty())
		}
		projected, err := empty.Project(nil)
		astTestMust(t, err)
		if projected.Empty() || !projected.ContainsFact(nil) {
			t.Fatal("source nullary projection did not initialize a full destination")
		}
		if err = empty.AddFact(nil); err != nil || empty.Empty() || !empty.ContainsFact(nil) {
			t.Fatal("nullary fact did not make the empty relation full", err, empty.Empty())
		}
		joined, err := JoinBoundRelations(full, empty, nil, nil)
		astTestMust(t, err)
		if joined.Empty() {
			t.Fatal("full nullary join")
		}
		joined, err = JoinBoundRelations(full, NewBoundRelation(0, true), nil, nil)
		astTestMust(t, err)
		if !joined.Empty() {
			t.Fatal("empty nullary join")
		}
	})

	t.Run("interval", func(t *testing.T) {
		full, empty := NewIntervalFull(0), NewIntervalEmpty(0)
		if full.Empty() || !full.ContainsFact(nil) || !empty.Empty() || !empty.ContainsFact(nil) {
			t.Fatal("nullary full/empty fact semantics", full.Empty(), empty.Empty())
		}
		projected, err := empty.Project(nil)
		astTestMust(t, err)
		if projected.Empty() || !projected.ContainsFact(nil) {
			t.Fatal("source nullary projection did not initialize a full destination")
		}
		if err = empty.AddFact([]*big.Rat{}); err != nil || empty.Empty() || !empty.ContainsFact(nil) {
			t.Fatal("nullary fact did not make the empty relation full", err, empty.Empty())
		}
		joined, err := JoinIntervalRelations(full, empty, nil, nil)
		astTestMust(t, err)
		if joined.Empty() {
			t.Fatal("full nullary join")
		}
		joined, err = JoinIntervalRelations(full, NewIntervalEmpty(0), nil, nil)
		astTestMust(t, err)
		if !joined.Empty() {
			t.Fatal("empty nullary join")
		}
	})
}
