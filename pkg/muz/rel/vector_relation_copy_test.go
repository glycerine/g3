package rel

import (
	"math/big"
	"testing"
)

// vector_relation::copy writes through the destination's own m_eqs and keeps
// them: a union into an empty target whose equalities were merged before it
// became empty keeps the class, holding the last column's value.
func TestVectorRelationCopyKeepsDestinationEqualities(t *testing.T) {
	t.Run("interval empty target", func(t *testing.T) {
		m := IntervalTestManager()
		target := NewIntervalFull(m, 3)
		astTestMust(t, target.FilterIdentical(0, 1))
		astTestMust(t, target.FilterEqualRat(0, big.NewRat(1, 1)))
		astTestMust(t, target.FilterEqualRat(2, big.NewRat(5, 1)))
		astTestMust(t, target.FilterIdentical(0, 2))
		if !target.Empty() || target.values.find(0) != target.values.find(1) {
			t.Fatal("setup: equate did not empty the merged relation", target.Empty())
		}
		source := NewIntervalFull(m, 3)
		astTestMust(t, source.FilterEqualRat(0, big.NewRat(7, 1)))
		astTestMust(t, source.FilterEqualRat(1, big.NewRat(8, 1)))
		delta := NewIntervalEmpty(m, 3)
		_, err := target.UnionWithDelta(source, delta, false)
		astTestMust(t, err)
		if target.Empty() || target.values.find(0) != target.values.find(1) {
			t.Fatal("copy into the empty target split its equalities")
		}
		if !target.ContainsFact([]*big.Rat{big.NewRat(8, 1), big.NewRat(8, 1), big.NewRat(0, 1)}) || target.ContainsFact([]*big.Rat{big.NewRat(7, 1), big.NewRat(8, 1), big.NewRat(0, 1)}) {
			t.Fatal("the target class does not hold the last column's value")
		}
		// delta->copy(src), not a copy of the target.
		if delta.Empty() || delta.values.find(0) == delta.values.find(1) || !delta.ContainsFact([]*big.Rat{big.NewRat(7, 1), big.NewRat(8, 1), big.NewRat(0, 1)}) {
			t.Fatal("delta is not the source's copy")
		}
	})
	t.Run("interval merged delta", func(t *testing.T) {
		m := IntervalTestManager()
		delta := NewIntervalFull(m, 2)
		astTestMust(t, delta.FilterIdentical(0, 1))
		target, source := NewIntervalFull(m, 2), NewIntervalFull(m, 2)
		astTestMust(t, target.FilterEqualRat(0, big.NewRat(1, 1)))
		astTestMust(t, target.FilterEqualRat(1, big.NewRat(2, 1)))
		astTestMust(t, source.FilterEqualRat(0, big.NewRat(3, 1)))
		astTestMust(t, source.FilterEqualRat(1, big.NewRat(4, 1)))
		changed, err := target.UnionWithDelta(source, delta, false)
		astTestMust(t, err)
		if !changed || delta.Empty() || delta.values.find(0) != delta.values.find(1) {
			t.Fatal("delta copy did not keep delta's equalities", changed)
		}
		// The class holds column 1's hull [2, 4].
		if !delta.ContainsFact([]*big.Rat{big.NewRat(4, 1), big.NewRat(4, 1)}) || delta.ContainsFact([]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1)}) {
			t.Fatal("delta class does not hold the last column's hull")
		}
	})
	t.Run("bound empty target", func(t *testing.T) {
		target := NewBoundRelation(3, false)
		astTestMust(t, target.Equate(0, 1))
		astTestMust(t, target.AddLess(0, 1))
		if !target.Empty() || target.find(0) != target.find(1) {
			t.Fatal("setup: strict self-bound did not empty the merged relation", target.Empty())
		}
		source := NewBoundRelation(3, false)
		astTestMust(t, source.AddLess(1, 2))
		delta := NewBoundRelation(3, true)
		_, err := target.UnionWithDelta(source, delta, false)
		astTestMust(t, err)
		if target.Empty() || target.find(0) != target.find(1) {
			t.Fatal("copy into the empty target split its equalities")
		}
		if strict, _, err := target.Bounds(0); err != nil || len(strict) != 1 || strict[0] != 2 {
			t.Fatal("the target class does not hold the last column's bounds", strict, err)
		}
		if delta.Empty() || delta.find(0) == delta.find(1) {
			t.Fatal("delta is not the source's copy")
		}
		if strict, _, err := delta.Bounds(0); err != nil || len(strict) != 0 {
			t.Fatal("delta column 0 is not the source's", strict, err)
		}
	})
	t.Run("bound merged delta", func(t *testing.T) {
		delta := NewBoundRelation(2, false)
		astTestMust(t, delta.Equate(0, 1))
		target, source := NewBoundRelation(2, false), NewBoundRelation(2, false)
		astTestMust(t, target.Equate(0, 1))
		changed, err := target.UnionWithDelta(source, delta, false)
		astTestMust(t, err)
		if !changed || delta.Empty() || delta.find(0) != delta.find(1) {
			t.Fatal("delta copy did not keep delta's equalities", changed)
		}
	})
}
