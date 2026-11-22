package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func shortPremiseExample(facts, universals int, conflict bool, goal string) string {
	var b strings.Builder
	b.WriteString("(declare-sort A 0)(declare-const a A)")
	for i := 0; i < max(facts, universals); i++ {
		fmt.Fprintf(&b, "(declare-fun p%d (A) Bool)", i)
	}
	b.WriteString("(assert (and")
	for i := 0; i < facts; i++ {
		if conflict && i == 0 {
			b.WriteString(" (not (p0 a))")
		} else {
			fmt.Fprintf(&b, " (p%d a)", i)
		}
	}
	for i := 0; i < universals; i++ {
		fmt.Fprintf(&b, " (forall ((x A)) (p%d x))", i)
	}
	// Selecting a disjunct here would refute the satisfiable example.
	b.WriteString(" (or (not (p0 a)) (forall ((v A)) (p0 v)))))")
	fmt.Fprintf(&b, "(assert %s)", goal)
	return b.String()
}

func TestShortPremisesCheck(t *testing.T) {
	for _, tc := range []struct {
		name, goal         string
		facts, universals  int
		conflict, admitted bool
	}{
		{"universal conflict", "(and (not false))", 8, 8, true, true},
		{"satisfiable", "(or (not (not true)))", 8, 8, false, true},
		{"too few universals", "true", 8, 7, false, false},
		{"too few facts", "true", 7, 8, false, false},
		{"symbolic goal", "(p0 a)", 8, 8, false, false},
		{"constant false", "(and (not true))", 8, 8, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, s := orderSolver(t, shortPremiseExample(tc.facts, tc.universals, tc.conflict, tc.goal))
			m, err := ctx.Manager()
			astTestMust(t, err)
			roots, err := solver.Assertions(s.solver)
			astTestMust(t, err)
			state := orderProofState{source: s}
			defer func() { astTestMust(t, state.release()) }()
			plan, err := state.shortPremises(m, roots)
			astTestMust(t, err)
			if (plan != nil) != tc.admitted {
				t.Fatalf("plan admitted = %t", plan != nil)
			}
			if plan == nil {
				return
			}
			if len(plan.selected) != tc.facts+tc.universals {
				t.Fatalf("unexpected selection of %d conjuncts", len(plan.selected))
			}
			got, err := state.checkProofPlan(m, plan, 80_000)
			if err != nil || got != tc.conflict {
				t.Fatalf("private proof = %t, %v", got, err)
			}
			if got, err := state.checkProofPlan(m, plan, 1); err != nil || got || len(m.ResourceLimit().Children) != 0 {
				t.Fatalf("step limit = %t, %v", got, err)
			}
			result, err := s.Check()
			astTestMust(t, err)
			if tc.conflict {
				if result != lbool.False {
					t.Fatalf("concrete result %v", result)
				}
				return
			}
			if result != lbool.True {
				t.Fatalf("concrete result %v", result)
			}
			if yes, err := s.ProveUnsatByOrderAbstraction(); yes || err != nil {
				t.Fatalf("private proof after SAT = %t, %v", yes, err)
			}
			model, err := s.Model()
			astTestMust(t, err)
			if model == nil {
				t.Fatal("source lost its concrete model")
			}
			astTestMust(t, model.Close())
		})
	}
}

func TestShortPremisesValidatesOmittedInput(t *testing.T) {
	ctx, s := orderSolver(t, shortPremiseExample(8, 8, true, "true"))
	m, err := ctx.Manager()
	astTestMust(t, err)
	roots, err := solver.Assertions(s.solver)
	astTestMust(t, err)
	state := orderProofState{source: s}
	defer func() { astTestMust(t, state.release()) }()
	bs, err := m.MkBoolSort()
	astTestMust(t, err)
	free, err := m.MkVar(0, bs)
	astTestMust(t, err)
	astTestMust(t, m.IncRef(free))
	defer func() { astTestMust(t, m.DecRef(free)) }()
	withFree := append([]ast.ASTHandle{free}, roots...)
	plan, err := state.shortPremises(m, withFree)
	astTestMust(t, err)
	if plan == nil {
		t.Fatal("missing contradictory closed subset")
	}
	if yes, err := state.checkProofPlan(m, plan, 80_000); err != nil || yes {
		t.Fatalf("proof ignored an open source term: %t, %v", yes, err)
	}
}

func TestShortPremisesRefuteRfn2Check26(t *testing.T) {
	_, s := orderSolver(t, appleBenchmarkInput(t, "rfn2-26"))
	if yes, err := s.ProveUnsatByOrderAbstraction(); err != nil || !yes {
		t.Fatalf("private proof on captured input = %t, %v", yes, err)
	}
}

func BenchmarkAppleShortPremises(b *testing.B) {
	input := appleBenchmarkInput(b, "rfn2-26")
	for _, mode := range []string{"Concrete", "ShortPremises"} {
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if err := benchmarkGoalInstances(b, input, mode == "ShortPremises"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
