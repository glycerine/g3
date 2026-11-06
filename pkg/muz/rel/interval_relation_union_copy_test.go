package rel

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/smtcore"
)

func TestIntervalRelationUnionIntoEmptyUsesSourceVectorCopy(t *testing.T) {
	source := NewIntervalFull(2)
	source.values.columns[0] = smtcore.NewRelationInterval(big.NewRat(1, 1), false, big.NewRat(5, 1), false)
	source.values.columns[1] = smtcore.NewRelationInterval(big.NewRat(3, 1), false, big.NewRat(8, 1), false)
	if err := source.values.equate(0, 1); err != nil {
		t.Fatal(err)
	}

	target := NewIntervalEmpty(2)
	changed, err := target.Union(source, false)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || target.Empty() || target.values.find(0) == target.values.find(1) {
		t.Fatal("source vector copy should fill the target with independent columns", changed, target.Empty())
	}
	for column := uint32(0); column < 2; column++ {
		lower, lowerOpen, upper, upperOpen, err := target.Bounds(column)
		if err != nil || lower == nil || lower.RatString() != "3" || lowerOpen || upper == nil || upper.RatString() != "5" || upperOpen {
			t.Fatalf("copied column %d bounds=(%v,%v,%v,%v), err=%v", column, lower, lowerOpen, upper, upperOpen, err)
		}
	}
	if source.values.find(0) != source.values.find(1) {
		t.Fatal("union changed the source equality class")
	}

	beforeLow, beforeLowerOpen, beforeHigh, beforeUpperOpen, err := target.Bounds(0)
	if err != nil {
		t.Fatal(err)
	}
	unchanged, err := target.Union(NewIntervalEmpty(2), false)
	if err != nil || unchanged || target.Empty() {
		t.Fatalf("empty source changed target: changed=%v empty=%v err=%v", unchanged, target.Empty(), err)
	}
	afterLow, afterLowerOpen, afterHigh, afterUpperOpen, err := target.Bounds(0)
	if err != nil || beforeLow.Cmp(afterLow) != 0 || beforeHigh.Cmp(afterHigh) != 0 || beforeLowerOpen != afterLowerOpen || beforeUpperOpen != afterUpperOpen {
		t.Fatalf("empty source changed target bounds: before=(%v,%v,%v,%v) after=(%v,%v,%v,%v) err=%v", beforeLow, beforeLowerOpen, beforeHigh, beforeUpperOpen, afterLow, afterLowerOpen, afterHigh, afterUpperOpen, err)
	}
}
