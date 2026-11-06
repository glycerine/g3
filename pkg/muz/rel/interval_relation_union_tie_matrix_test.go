package rel

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/smtcore"
)

func TestIntervalRelationUnionAndWidenEqualEndpointTieMatrix(t *testing.T) {
	newRelation := func(lowerOpen, upperOpen bool) *IntervalRelation {
		relation := NewIntervalFull(1)
		relation.values.columns[0] = smtcore.NewRelationInterval(big.NewRat(0, 1), lowerOpen, big.NewRat(2, 1), upperOpen)
		return relation
	}
	for targetMask := 0; targetMask < 4; targetMask++ {
		targetLowerOpen, targetUpperOpen := targetMask&1 != 0, targetMask&2 != 0
		for sourceMask := 0; sourceMask < 4; sourceMask++ {
			sourceLowerOpen, sourceUpperOpen := sourceMask&1 != 0, sourceMask&2 != 0
			name := fmt.Sprintf("target=%d/source=%d", targetMask, sourceMask)
			t.Run(name, func(t *testing.T) {
				target := newRelation(targetLowerOpen, targetUpperOpen)
				if _, err := target.Union(newRelation(sourceLowerOpen, sourceUpperOpen), false); err != nil {
					t.Fatal(err)
				}
				lower, lowerOpen, upper, upperOpen, err := target.Bounds(0)
				if err != nil || lower == nil || lower.Sign() != 0 || upper == nil || upper.Cmp(big.NewRat(2, 1)) != 0 {
					t.Fatalf("union hull bounds: [%v,%v], err=%v", lower, upper, err)
				}
				if lowerOpen != (targetLowerOpen && sourceLowerOpen) || upperOpen != (targetUpperOpen && sourceUpperOpen) {
					t.Fatalf("union openness=(%v,%v), want (%v,%v)", lowerOpen, upperOpen, targetLowerOpen && sourceLowerOpen, targetUpperOpen && sourceUpperOpen)
				}

				widened := newRelation(targetLowerOpen, targetUpperOpen)
				if _, err := widened.Union(newRelation(sourceLowerOpen, sourceUpperOpen), true); err != nil {
					t.Fatal(err)
				}
				widenLow, widenLowerOpen, widenHigh, widenUpperOpen, err := widened.Bounds(0)
				if err != nil {
					t.Fatal(err)
				}
				wantMinusInfinity := targetLowerOpen && !sourceLowerOpen
				wantPlusInfinity := !targetUpperOpen && sourceUpperOpen
				if wantMinusInfinity != (widenLow == nil) || wantPlusInfinity != (widenHigh == nil) {
					t.Fatalf("widen endpoints=(%v,%v), want infinite=(%v,%v)", widenLow, widenHigh, wantMinusInfinity, wantPlusInfinity)
				}
				if !wantMinusInfinity && (widenLow.Sign() != 0 || widenLowerOpen != targetLowerOpen) {
					t.Fatalf("widen lower=(%v,%v), want (0,%v)", widenLow, widenLowerOpen, targetLowerOpen)
				}
				if !wantPlusInfinity && (widenHigh.Cmp(big.NewRat(2, 1)) != 0 || widenUpperOpen != targetUpperOpen) {
					t.Fatalf("widen upper=(%v,%v), want (2,%v)", widenHigh, widenUpperOpen, targetUpperOpen)
				}
			})
		}
	}
}
