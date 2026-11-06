package rel

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/smtcore"
)

func TestBoundRelationUnionIntervalReadsAliasedColumnPayload(t *testing.T) {
	newRat := func(value int64) *big.Rat { return new(big.Rat).SetInt64(value) }
	for _, tc := range []struct {
		name   string
		strict bool
	}{
		{name: "strict", strict: true},
		{name: "weak", strict: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			intervals := NewIntervalFull(3)
			intervals.values.columns[0] = smtcore.NewRelationInterval(newRat(0), false, newRat(10), false)
			upper := int64(9)
			if tc.strict {
				upper = 8
			}
			intervals.values.columns[1] = smtcore.NewRelationInterval(newRat(5), false, newRat(upper), false)
			intervals.values.columns[2] = smtcore.NewRelationInterval(newRat(9), false, newRat(12), false)
			if err := intervals.values.equate(0, 1); err != nil {
				t.Fatal(err)
			}

			bounds := NewBoundRelation(3, false)
			var err error
			if tc.strict {
				err = bounds.AddLess(0, 2)
			} else {
				err = bounds.AddLessEqual(0, 2)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := bounds.UnionInterval(intervals); err != nil {
				t.Fatal(err)
			}
			strict, weak, err := bounds.Bounds(0)
			if err != nil {
				t.Fatal(err)
			}
			if len(strict) != 0 || len(weak) != 0 {
				t.Fatalf("source-indexed interval payload should prune edge, got strict=%v weak=%v", strict, weak)
			}
		})
	}
}
