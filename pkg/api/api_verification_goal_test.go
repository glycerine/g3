package api

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func relevantPremiseExample(last string, padding int) string {
	var b strings.Builder
	b.WriteString("(declare-sort U 0)(declare-const k U)(declare-fun alpha (U) Bool)(declare-fun beta (U) Bool)(declare-fun gamma (U) Bool)")
	for i := 0; i < padding; i++ {
		fmt.Fprintf(&b, "(declare-fun pad%d (U) Bool)", i)
	}
	b.WriteString("(assert (and (forall ((v U)) (= (alpha v) (beta v))) (forall ((v U)) (= (beta v) (gamma v))) ")
	b.WriteString(last)
	for i := 0; i < padding; i++ {
		fmt.Fprintf(&b, " (forall ((v U)) (pad%d v))", i)
	}
	b.WriteString("))(assert (not (alpha k)))")
	return b.String()
}

func TestVerificationDefinitionSelection(t *testing.T) {
	for _, tc := range []struct {
		name, last string
		proved     bool
	}{
		{"universal fact", "(forall ((v U)) (gamma v))", true},
		{"ground fact", "(gamma k)", true},
		{"cycle", "(forall ((v U)) (= (gamma v) (alpha v)))", false},
		{"disjunction", "(or (gamma k) (not (beta k)))", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, s := orderSolver(t, relevantPremiseExample(tc.last, 20))
			m, err := ctx.Manager()
			astTestMust(t, err)
			roots, err := solver.Assertions(s.solver)
			astTestMust(t, err)
			state := orderProofState{source: s}
			defer func() { astTestMust(t, state.release()) }()
			got, err := state.proveRelevantPremises(m, roots)
			if err != nil || got != tc.proved {
				t.Fatalf("selected proof %v, want %v, error %v", got, tc.proved, err)
			}
			result, err := s.Check()
			astTestMust(t, err)
			if result == lbool.Undef || (result == lbool.False) != tc.proved {
				t.Fatal("concrete check", result)
			}
			if tc.proved {
				sort, err := m.MkBoolSort()
				astTestMust(t, err)
				v, err := m.MkVar(0, sort)
				astTestMust(t, err)
				astTestMust(t, m.IncRef(v))
				defer func() { astTestMust(t, m.DecRef(v)) }()
				open := append([]ast.ASTHandle{v}, roots...)
				if yes, err := state.proveRelevantPremises(m, open); yes || err != nil {
					t.Fatalf("omitted free variable %v, %v", yes, err)
				}
			}
		})
	}
}

func TestVerificationWrappedGoalWidth(t *testing.T) {
	for _, width := range []int{5, 6} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			var decls, conjuncts strings.Builder
			for i := 0; i < width; i++ {
				fmt.Fprintf(&decls, "(declare-const k%d U)", i)
				fmt.Fprintf(&conjuncts, " (alpha k%d)", i)
			}
			input := relevantPremiseExample("(forall ((v U)) (gamma v))", 20)
			input = strings.TrimSuffix(input, "(assert (not (alpha k)))") + decls.String() +
				"(assert (or (not (and" + conjuncts.String() + "))))"
			ctx, s := orderSolver(t, input)
			m, err := ctx.Manager()
			astTestMust(t, err)
			roots, err := solver.Assertions(s.solver)
			astTestMust(t, err)
			state := orderProofState{source: s}
			defer func() { astTestMust(t, state.release()) }()
			proved, err := state.proveRelevantPremises(m, roots)
			astTestMust(t, err)
			if proved != (width == 5) {
				t.Fatalf("width %d private proof returned %t", width, proved)
			}
			result, err := s.Check()
			astTestMust(t, err)
			if result != lbool.False {
				t.Fatalf("width %d concrete check returned %v", width, result)
			}
		})
	}
}

func TestVerificationWidePremiseSavings(t *testing.T) {
	for _, omitted := range []int{20, maxWidePremises - 1, maxWidePremises} {
		t.Run(fmt.Sprint(omitted), func(t *testing.T) {
			// The two definitions and ground fact suffice. All other
			// conjuncts are independent universal predicates.
			ctx, s := orderSolver(t, relevantPremiseExample("(gamma k)", omitted))
			m, err := ctx.Manager()
			astTestMust(t, err)
			roots, err := solver.Assertions(s.solver)
			astTestMust(t, err)
			batch := s.BeginOrderProofBatch()
			t.Cleanup(func() { astTestMust(t, batch.Close()) })
			want := omitted >= maxWidePremises || !batch.state.plans.reusable(m)
			proved, err := batch.state.proveRelevantPremises(m, roots)
			if err != nil || proved != want {
				t.Fatalf("omitting %d conjuncts gave %t, want %t, error %v", omitted, proved, want, err)
			}
			result, err := s.Check()
			if err != nil || result != lbool.False {
				t.Fatalf("concrete result %v, error %v", result, err)
			}
		})
	}
}

func TestVerificationWideSearchAfterDefinitions(t *testing.T) {
	for _, size := range []int{maxNarrowPremises, maxNarrowPremises + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			// The definitions connect alpha to gamma. The ground gamma fact
			// finishes the proof, and each ground pad fact adds a wide row.
			var facts strings.Builder
			facts.WriteString("(gamma k)")
			for i := 0; i < size-3; i++ {
				fmt.Fprintf(&facts, " (pad%d k)", i)
			}
			ctx, s := orderSolver(t, relevantPremiseExample(facts.String(), maxWidePremises))
			m, err := ctx.Manager()
			astTestMust(t, err)
			roots, err := solver.Assertions(s.solver)
			astTestMust(t, err)
			batch := s.BeginOrderProofBatch()
			t.Cleanup(func() { astTestMust(t, batch.Close()) })
			want := size <= maxNarrowPremises || !batch.state.plans.reusable(m)
			got, err := batch.state.proveRelevantPremises(m, roots)
			if err != nil || got != want {
				t.Fatalf("wide search with %d rows gave %t, want %t, error %v", size, got, want, err)
			}
			result, err := s.Check()
			if err != nil || result != lbool.False {
				t.Fatalf("complete check gave %v, error %v", result, err)
			}
		})
	}
}

func TestVerificationRarePremisesAfterSatisfiableDefinitions(t *testing.T) {
	for _, tc := range []struct{ name, premises string }{
		{"ground fact", "(alpha k)"},
		{"complementary clauses", "(or (alpha k) (gamma k)) (or (alpha k) (not (gamma k)))"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The definitions admit alpha(k) = beta(k) = gamma(k) = false.
			// Each additional premise set entails alpha(k), refuting the goal.
			ctx, s := orderSolver(t, relevantPremiseExample(tc.premises, 20))
			m, err := ctx.Manager()
			astTestMust(t, err)
			roots, err := solver.Assertions(s.solver)
			astTestMust(t, err)
			batch := s.BeginOrderProofBatch()
			t.Cleanup(func() { astTestMust(t, batch.Close()) })
			proved, err := batch.state.proveRelevantPremises(m, roots)
			if err != nil || !proved {
				t.Fatalf("private premise search returned %t, %v", proved, err)
			}
			result, err := s.Check()
			if err != nil || result != lbool.False {
				t.Fatalf("complete check returned %v, %v", result, err)
			}
		})
	}
}

func BenchmarkVerificationPremiseSearch(b *testing.B) {
	for _, name := range []string{"ticket_wide_goal", "rfn2-5", "cf_live_overlapping_premises", "tlb_overlapping_premises"} {
		b.Run(name, func(b *testing.B) {
			ctx, source := orderSolver(b, appleBenchmarkInput(b, name))
			m, err := ctx.Manager()
			astTestMust(b, err)
			roots, err := solver.Assertions(source.solver)
			astTestMust(b, err)
			var proofs int
			b.ReportAllocs()
			for b.Loop() {
				batch := source.BeginOrderProofBatch()
				proved, err := batch.state.proveRelevantPremises(m, roots)
				astTestMust(b, errors.Join(err, batch.Close()))
				if proved {
					proofs++
				}
			}
			b.ReportMetric(float64(proofs)/float64(b.N), "refutations/op")
		})
	}
}

func TestOrderProofTemporaryGoal(t *testing.T) {
	c, donor := orderSolver(t, orderPremises+"(assert (not (p a)))")
	m, err := c.Manager()
	astTestMust(t, err)
	roots, err := solver.Assertions(donor.solver)
	astTestMust(t, err)
	s, err := c.NewSolverForIncrementalChecks()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	batch := s.BeginOrderProofBatch()
	t.Cleanup(func() { astTestMust(t, batch.Close()) })
	astTestMust(t, s.Assert(roots[0]))
	for i := 0; i < 2; i++ {
		if yes, err := batch.ProveWithGoal(roots[1]); err != nil || !yes {
			t.Fatalf("private goal %d: %v, %v", i, yes, err)
		}
		level, err := s.ScopeLevel()
		astTestMust(t, err)
		active, err := solver.Assertions(s.solver)
		astTestMust(t, err)
		if level != 0 || len(active) != 1 || active[0] != roots[0] || s.pendingRefutation != nil {
			t.Fatal("temporary proof changed source", level, active)
		}
	}
	positive, err := m.GetArg(roots[1], 0)
	astTestMust(t, err)
	if yes, err := batch.ProveWithGoal(positive); err != nil || yes || s.pendingRefutation != nil {
		t.Fatal("satisfiable counterexample", yes, err)
	}
	// A subsequent ordinary SAT check sees just the base, with its own model.
	got, err := s.Check()
	astTestMust(t, err)
	if got != lbool.True {
		t.Fatal("base", got)
	}
	if yes, err := batch.ProveWithGoal(roots[1]); err != nil || yes {
		t.Fatal("checked source", yes, err)
	}
	model, err := s.Model()
	astTestMust(t, err)
	ref, err := model.Evaluate(positive, true)
	astTestMust(t, err)
	h, err := m.RefGet(ref)
	astTestMust(t, err)
	if !m.IsTrue(h) {
		t.Fatal("model lost ground instance")
	}
	astTestMust(t, m.DelRef(&ref))
	astTestMust(t, model.Close())
	astTestMust(t, s.Reset())
	if yes, err := batch.ProveWithGoal(roots[1]); err != nil || yes {
		t.Fatal("old base after reset", yes, err)
	}
	astTestMust(t, s.Assert(roots[0]))
	limit := m.ResourceLimit()
	astTestMust(t, limit.Push(1))
	yes, err := batch.ProveWithGoal(roots[1])
	limit.Pop()
	if yes || err != nil || len(limit.Children) != 0 {
		t.Fatal("active resource limit", yes, err)
	}
	astTestMust(t, s.Push())
	if yes, err := batch.ProveWithGoal(roots[1]); yes || err != nil {
		t.Fatal("active source scope", yes, err)
	}
	astTestMust(t, s.Assert(roots[1]))
	got, err = s.Check()
	astTestMust(t, err)
	if got != lbool.False {
		t.Fatal("concrete scoped result", got)
	}
	astTestMust(t, s.Pop(1))
	astTestMust(t, batch.Close())
	if _, err := batch.ProveWithGoal(roots[1]); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed batch", err)
	}
}

func TestTemporaryGoalValidatesHandles(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	s, err := f.c.NewSolver()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	b := s.BeginOrderProofBatch()
	t.Cleanup(func() { astTestMust(t, b.Close()) })
	other := newSMTAPIFixture(t, "QF_UF", nil)
	foreign := other.pin(other.m.MkTrue())
	sort := f.pin(f.c.BoolSort())
	stale, err := f.m.MkVar(3001, sort)
	astTestMust(t, err)
	astTestMust(t, f.m.IncRef(stale))
	astTestMust(t, f.m.DecRef(stale))
	for _, h := range []ast.ASTHandle{foreign, stale, {}, sort} {
		if yes, err := b.ProveWithGoal(h); yes || err == nil {
			t.Fatal("accepted invalid goal", h, yes, err)
		}
	}
}

func BenchmarkAppleVerificationGoalPremises(b *testing.B) {
	for _, n := range []string{"5", "9"} {
		b.Run("Check"+n, func(b *testing.B) {
			input := appleBenchmarkInput(b, "rfn2-"+n)
			ctx, err := NewSMTContext()
			if err != nil {
				b.Fatal(err)
			}
			defer ctx.Close()
			s, err := ctx.NewSolver()
			if err != nil {
				b.Fatal(err)
			}
			defer s.Close()
			if err := s.LoadSMT2String(input); err != nil {
				b.Fatal(err)
			}
			m, err := ctx.Manager()
			if err != nil {
				b.Fatal(err)
			}
			roots, err := solver.Assertions(s.solver)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				state := orderProofState{source: s}
				yes, err := state.proveRelevantPremises(m, roots)
				if err := errors.Join(err, state.release()); err != nil || !yes {
					b.Fatalf("selected proof %v, %v", yes, err)
				}
			}
		})
	}
}

func TestVerificationSizedBatchSmallSelection(t *testing.T) {
	for _, tc := range []struct {
		name           string
		padding, goals int
		attempt        bool
	}{
		{"64 premises", 61, 8, true},
		{"65 premises", 62, 8, false},
		{"unsized batch", 62, 0, true},
		{"wide selection eligible", maxWidePremises, 8, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fact := "(forall ((v U)) (gamma v))"
			if tc.padding == maxWidePremises {
				fact = "(gamma k)"
			}
			ctx, s := orderSolver(t, relevantPremiseExample(fact, tc.padding))
			m, err := ctx.Manager()
			astTestMust(t, err)
			roots, err := solver.Assertions(s.solver)
			astTestMust(t, err)
			batch := s.BeginOrderProofBatchForGoals(tc.goals)
			t.Cleanup(func() { astTestMust(t, batch.Close()) })
			proved, err := batch.state.proveRelevantPremises(m, roots)
			astTestMust(t, err)
			if want := tc.attempt || !batch.state.plans.reusable(m); proved != want {
				t.Fatalf("private result %t, want %t", proved, want)
			}
			result, err := s.Check()
			astTestMust(t, err)
			if result != lbool.False {
				t.Fatalf("concrete check returned %v", result)
			}
		})
	}
}
