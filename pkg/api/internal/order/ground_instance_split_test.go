package order

import (
	"math/rand"
	"slices"
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
)

func TestGroundSplitRestoresBothBranches(t *testing.T) {
	limit, err := control.NewReslimit(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer limit.Close()
	// Under goal 1, all four assignments to 2 and 3 are excluded.
	q := GroundInstanceQuery{
		Variables: 4,
		Clauses: []BooleanClause{
			{4},
			{-1, 2, 3}, {-1, 2, -3}, {-1, -2, 3}, {-1, -2, -3},
		},
		units: new(instanceUnits),
	}
	for _, x := range []struct {
		goal        int32
		split, want bool
	}{{1, false, false}, {1, true, true}, {-1, true, false}, {2, true, false}, {1, true, true}} {
		q.Goal = x.goal
		if got, ok := q.Reduce(limit, x.split); !ok || got != x.want || !slices.Equal(q.PermanentUnits(), []int32{4}) {
			t.Fatalf("goal %d split %t: conflict %t complete %t units %v", x.goal, x.split, got, ok, q.PermanentUnits())
		}
	}
	q.Goal = 1
	for budget := uint32(1); budget <= 30; budget++ {
		if err := limit.Push(budget); err != nil {
			t.Fatal(err)
		}
		conflict, complete := q.Reduce(limit, true)
		limit.Pop()
		if conflict != complete || !slices.Equal(q.PermanentUnits(), []int32{4}) {
			t.Fatalf("budget %d conflict %t complete %t units %v", budget, conflict, complete, q.PermanentUnits())
		}
		if cf, ok := q.Reduce(limit, true); !cf || !ok {
			t.Fatalf("retry failed after budget %d", budget)
		}
	}
}

func TestGroundSplitBooleanModels(t *testing.T) {
	rng := rand.New(rand.NewSource(1141))
	limit, e := control.NewReslimit(nil)
	if e != nil {
		t.Fatal(e)
	}
	defer limit.Close()
	const variables = 6
	value := func(lit int32, mask int) bool {
		v := lit
		if v < 0 {
			v = -v
		}
		return (mask&(1<<(v-1)) != 0) == (lit > 0)
	}
	for trial := 0; trial < 512; trial++ {
		q := GroundInstanceQuery{Variables: variables, units: new(instanceUnits)}
		for j := 0; j < 24; j++ {
			var row BooleanClause
			for k := range row {
				lit := int32(1 + rng.Intn(variables))
				if rng.Intn(2) == 0 {
					lit = -lit
				}
				row[k] = lit
			}
			q.Clauses = append(q.Clauses, row)
			if j%4 != 3 {
				continue
			}
			for g := int32(-variables); g <= variables; g++ {
				if g == 0 {
					continue
				}
				q.Goal = g
				conflict, ok := q.Reduce(limit, true)
				if !ok {
					t.Fatal("unexpected interruption")
				}
				if !conflict {
					continue
				}
				for mask := 0; mask < 1<<variables; mask++ {
					if !value(g, mask) {
						continue
					}
					model := true
					for _, clause := range q.Clauses {
						if !value(clause[0], mask) && !value(clause[1], mask) && !value(clause[2], mask) {
							model = false
							break
						}
					}
					if model {
						t.Fatalf("trial %d goal %d: refuted model %06b for %v", trial, g, mask, q.Clauses)
					}
				}
			}
		}
	}
}
