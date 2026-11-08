package ackermannization_test

import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/util/statistics"
	"math"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ackermannization"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestAckermannTacticAppendFallbackAndStatistics(t *testing.T) {
	c := ackContext(t, `(set-logic ALL)(declare-const x Bool)(declare-const y Bool)
(declare-fun f (Bool) Bool)(assert (not (= (f x) (f y))))`)
	m, err := c.Manager()
	ackMust(t, err)
	r, err := ackermannization.NewTactic(m, nil, ackServices(t, c))
	ackMust(t, err)
	t.Cleanup(func() { ackMust(t, r.Close()) })
	ackMust(t, r.UpdateParams(nil))
	g, err := tactic.NewGoal(m, false, true, false)
	ackMust(t, err)
	t.Cleanup(func() { ackMust(t, g.Close()) })
	ackMust(t, c.AssertGoal(g))
	var result tactic.TacticResult
	t.Cleanup(func() { ackMust(t, result.Close()) })
	for i := 1; i <= 2; i++ {
		ackMust(t, r.Apply(g, &result))
		if len(result.Goals) != i {
			t.Fatal("successful application did not append its goal")
		}
		if got := statistics.Sums(r.CollectStatistics)["ackr-constraints"]; got != float64(i) {
			t.Fatalf("constraints = %v, want %d", got, i)
		}
	}
	globals, err := c.GlobalParameters()
	ackMust(t, err)
	p, err := globals.Context.NewValue()
	ackMust(t, err)
	t.Cleanup(func() { ackMust(t, globals.Context.Del(&p)) })
	key := "div0_ackermann_limit"
	ackMust(t, globals.Context.SetUIntName(&p, &key, 0))
	ackMust(t, r.UpdateParams(&p))
	ackMust(t, r.Apply(g, &result))
	if len(result.Goals) != 1 || result.Goals[0] != g {
		t.Fatal("limit fallback did not replace result with original goal")
	}
	if result.ModelConverter != nil {
		t.Fatal("fallback retained a model converter")
	}
	if got := statistics.Sums(r.CollectStatistics)["ackr-constraints"]; got != 2 {
		t.Fatal("fallback changed statistics", got)
	}
	ackMust(t, r.ResetStatistics())
	if got := statistics.Sums(r.CollectStatistics)["ackr-constraints"]; got != 0 {
		t.Fatal("statistics not reset", got)
	}
}

func TestAckermannLemmaBoundSourceOverflow(t *testing.T) {
	for _, tc := range []struct {
		n    uint32
		want float64
	}{
		{0, 0}, {1, 0}, {3, 3}, {65535, 2147385345},
		{65536, math.Inf(1)}, {131071, math.Inf(1)}, {131072, 4294901760},
		{math.MaxUint32, math.Inf(1)},
	} {
		if got := ackermannization.NChoose2Bound(tc.n); got != tc.want {
			t.Fatalf("source bound(%d) = %v, want %v", tc.n, got, tc.want)
		}
	}
}

func TestAckermannConstructorSnapshotAndRefusalOrder(t *testing.T) {
	c := ackContext(t, `(set-option :produce-proofs true)(set-logic ALL)(declare-const x Bool)(declare-const y Bool)
(declare-fun f (Bool) Bool)(assert (not (= (f x) (f y))))`)
	m, err := c.Manager()
	ackMust(t, err)
	globals, err := c.GlobalParameters()
	ackMust(t, err)
	p, err := globals.Context.NewValue()
	ackMust(t, err)
	t.Cleanup(func() { ackMust(t, globals.Context.Del(&p)) })
	key := "max_steps"
	ackMust(t, globals.Context.SetUIntName(&p, &key, 0))
	r, err := ackermannization.NewTactic(m, &p, ackServices(t, c))
	ackMust(t, err)
	t.Cleanup(func() { ackMust(t, r.Close()) })
	// Updates set only the lemma limit. They leave the constructor's
	// simplifier parameters unchanged, including through translation.
	ackMust(t, globals.Context.SetUIntName(&p, &key, 10000))
	ackMust(t, r.UpdateParams(&p))
	g, err := tactic.NewGoal(m, false, true, false)
	ackMust(t, err)
	t.Cleanup(func() { ackMust(t, g.Close()) })
	ackMust(t, c.AssertGoal(g))
	var result tactic.TacticResult
	t.Cleanup(func() { ackMust(t, result.Close()) })
	if err = r.Apply(g, &result); err == nil || !strings.Contains(err.Error(), "max") {
		t.Fatal("constructor max_steps snapshot was replaced", err)
	}
	guarded, err := tactic.NewGoal(m, true, false, true)
	ackMust(t, err)
	t.Cleanup(func() { ackMust(t, guarded.Close()) })
	ackMust(t, result.AddGoal(g))
	if err = r.Apply(guarded, &result); err == nil || err.Error() != "ackermannize does not support unsat core production" {
		t.Fatal("unsat core refusal must precede proof refusal", err)
	}
	if len(result.Goals) != 1 || result.Goals[0] != g {
		t.Fatal("refusal changed previous goals")
	}
}

// Every injected allocation failure must leave its real owners reachable so
// that disabling injection and closing the tactic/result drains all storage.
func TestAckermannTacticAllocationFrontiers(t *testing.T) {
	completed := false
	for frontier := int64(0); frontier < 180 && !completed; frontier++ {
		t.Run(fmt.Sprint(frontier), func(t *testing.T) {
			c := ackContext(t, `(set-logic ALL)(declare-const x (_ BitVec 3))
(declare-const y (_ BitVec 3))(declare-fun f ((_ BitVec 3)) (_ BitVec 3))
(assert (not (= (f x) (f y))))`)
			m, err := c.Manager()
			ackMust(t, err)
			r, err := ackermannization.NewTactic(m, nil, ackServices(t, c))
			ackMust(t, err)
			t.Cleanup(func() { ackMust(t, r.Close()) })
			ackMust(t, r.UpdateParams(nil))
			g, err := tactic.NewGoal(m, false, true, false)
			ackMust(t, err)
			t.Cleanup(func() { ackMust(t, g.Close()) })
			ackMust(t, c.AssertGoal(g))
			var result tactic.TacticResult
			t.Cleanup(func() { ackMust(t, result.Close()) })
			m.Budget().FailAfter(frontier)
			err = r.Apply(g, &result)
			m.Budget().FailAfter(-1)
			if err == nil {
				completed = true
				return
			}
			if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatalf("unexpected refusal: %v", err)
			}
			var failure memory.ConstructionFailure
			if errors.As(err, &failure) {
				ackMust(t, failure.Cleanup())
			}
		})
	}
	if !completed {
		t.Fatal("allocation campaign never reached successful application")
	}
}
