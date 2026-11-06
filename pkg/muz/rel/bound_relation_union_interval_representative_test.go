package rel

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/smtcore"
)

func TestBoundRelationUnionIntervalUsesRepresentativePayload(t *testing.T) {
	newRat := func(value int64) *big.Rat { return new(big.Rat).SetInt64(value) }
	for _, tc := range []struct {
		name   string
		strict bool
		upper  int64
	}{
		{name: "strict", strict: true, upper: 8},
		{name: "weak", strict: false, upper: 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			intervals := NewIntervalFull(3)
			intervals.values.columns[0] = smtcore.NewRelationInterval(newRat(0), false, newRat(10), false)
			intervals.values.columns[1] = smtcore.NewRelationInterval(newRat(5), false, newRat(tc.upper), false)
			intervals.values.columns[2] = smtcore.NewRelationInterval(newRat(9), false, newRat(12), false)
			if err := intervals.values.equate(0, 1); err != nil {
				t.Fatal(err)
			}
			if intervals.values.find(0) != 1 {
				t.Fatal("equal-size merge must select column 1 as representative")
			}

			bounds := NewBoundRelation(3, false)
			if tc.strict {
				if err := bounds.AddLess(0, 2); err != nil {
					t.Fatal(err)
				}
			} else if err := bounds.AddLessEqual(0, 2); err != nil {
				t.Fatal(err)
			}
			if err := bounds.UnionInterval(intervals); err != nil {
				t.Fatal(err)
			}
			strict, weak, err := bounds.Bounds(0)
			if err != nil {
				t.Fatal(err)
			}
			if tc.strict && (len(strict) != 1 || strict[0] != 2) || !tc.strict && (len(weak) != 1 || weak[0] != 2) {
				t.Fatalf("representative endpoint should preserve edge: strict=%v weak=%v", strict, weak)
			}
		})
	}
}
