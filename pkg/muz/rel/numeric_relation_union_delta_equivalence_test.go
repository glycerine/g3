package rel

import (
	"math/big"
	"testing"
)

func TestNumericRelationUnionDeltaPublishesEqualitySplit(t *testing.T) {
	t.Run("bound", func(t *testing.T) {
		target, source, delta := NewBoundRelation(2, false), NewBoundRelation(2, false), NewBoundRelation(2, true)
		astTestMust(t, target.Equate(0, 1))
		changed, err := target.UnionWithDelta(source, delta, false)
		astTestMust(t, err)
		if !changed || target.find(0) == target.find(1) || delta.Empty() || delta.find(0) == delta.find(1) {
			t.Fatal("union equality split was not published to delta", changed, target.find(0), target.find(1), delta.Empty(), delta.find(0), delta.find(1))
		}
		changed, err = target.UnionWithDelta(source, delta, false)
		astTestMust(t, err)
		if changed || delta.Empty() || delta.find(0) == delta.find(1) {
			t.Fatal("stable equality split changed or cleared delta", changed)
		}
	})

	t.Run("interval", func(t *testing.T) {
		target, source, delta := NewIntervalFull(2), NewIntervalFull(2), NewIntervalEmpty(2)
		astTestMust(t, target.FilterIdentical(0, 1))
		astTestMust(t, source.FilterEqual(0, big.NewRat(1, 1)))
		astTestMust(t, source.FilterEqual(1, big.NewRat(2, 1)))
		changed, err := target.UnionWithDelta(source, delta, false)
		astTestMust(t, err)
		if !changed || target.values.find(0) == target.values.find(1) || delta.Empty() || delta.values.find(0) == delta.values.find(1) {
			t.Fatal("union equality split was not published to delta", changed, target.values.find(0), target.values.find(1), delta.Empty(), delta.values.find(0), delta.values.find(1))
		}
		if !delta.ContainsFact([]*big.Rat{big.NewRat(3, 1), big.NewRat(4, 1)}) {
			t.Fatal("delta copy did not preserve the split relation's full interval payload")
		}
		changed, err = target.UnionWithDelta(source, delta, false)
		astTestMust(t, err)
		if changed || delta.Empty() || delta.values.find(0) == delta.values.find(1) {
			t.Fatal("stable equality split changed or cleared delta", changed)
		}
	})
}
