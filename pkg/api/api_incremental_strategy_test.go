package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func checkDeferredTactic(t *testing.T, s *APISimpleSolver, want bool) {
	t.Helper()
	want = want && !diagnostic.DebugBuild && !diagnostic.TraceCompiled()
	c, ok := s.solver.(*solver.CombinedSolver)
	if !ok || c.HasDeferredTactic() != want {
		t.Fatalf("deferred tactic for %T, want %v", s.solver, want)
	}
}

func TestIncrementalStrategyChecksAndReset(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	s, err := f.c.NewSolverForIncrementalChecks()
	astTestMust(t, err)
	defer func() { astTestMust(t, s.Close()) }()
	options, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, options.SetBool("smt.macro_finder", true))
	astTestMust(t, s.SetParameters(options))
	astTestMust(t, options.Close())
	yes := f.pin(f.m.MkTrue())
	no := f.pin(f.m.MkFalse())
	astTestMust(t, s.Assert(yes))
	checkDeferredTactic(t, s, true)
	astTestMust(t, s.Push())
	astTestMust(t, s.Assert(no))
	status, err := s.Check()
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("expected scoped UNSAT", status)
	}
	astTestMust(t, s.Pop(1))
	status, err = s.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("expected SAT after pop", status)
	}
	md, err := s.Model()
	astTestMust(t, err)
	astTestMust(t, md.Close())
	checkDeferredTactic(t, s, true)
	astTestMust(t, s.Reset())
	astTestMust(t, s.Assert(yes))
	checkDeferredTactic(t, s, true)
	status, err = s.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("expected fresh SAT", status)
	}
	checkDeferredTactic(t, s, false)
}

func TestIncrementalStrategyParameterCatalog(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	s, err := f.c.NewSolverForIncrementalChecks()
	astTestMust(t, err)
	defer func() { astTestMust(t, s.Close()) }()
	astTestMust(t, s.Assert(f.pin(f.m.MkTrue())))
	combined := s.solver.(*solver.CombinedSolver)
	partial, err := s.getSolverDescriptions(combined.CollectIncrementalParamDescrs)
	astTestMust(t, err)
	defer func() { astTestMust(t, partial.Close()) }()
	checkDeferredTactic(t, s, true)
	full, err := s.ParameterDescriptions()
	astTestMust(t, err)
	defer func() { astTestMust(t, full.Close()) }()
	checkDeferredTactic(t, s, false)
	n, err := partial.Size()
	astTestMust(t, err)
	for i := uint32(0); i < n; i++ {
		name, err := partial.Name(i)
		astTestMust(t, err)
		pk, err := partial.value.KindName(name)
		astTestMust(t, err)
		fk, err := full.value.KindName(name)
		astTestMust(t, err)
		pm, pok, err := partial.value.ModuleName(name)
		astTestMust(t, err)
		fm, fok, err := full.value.ModuleName(name)
		astTestMust(t, err)
		if pk != fk || pm != fm || pok != fok {
			t.Fatalf("descriptor for %q differs in partial and full catalogs", name)
		}
	}
	ordinary, err := f.c.NewSolver()
	astTestMust(t, err)
	defer func() { astTestMust(t, ordinary.Close()) }()
	want, err := ordinary.ParameterDescriptions()
	astTestMust(t, err)
	defer func() { astTestMust(t, want.Close()) }()
	size, err := want.Size()
	astTestMust(t, err)
	have, err := full.Size()
	astTestMust(t, err)
	if have != size {
		t.Fatalf("catalog size %d, want %d", have, size)
	}
	for i := uint32(0); i < size; i++ {
		a, err := want.Name(i)
		astTestMust(t, err)
		b, err := full.Name(i)
		astTestMust(t, err)
		if a != b {
			t.Fatalf("catalog position %d is %q, want %q", i, b, a)
		}
	}
}

func TestIncrementalStrategyValidation(t *testing.T) {
	for _, option := range []string{"smt.random_seed", "rewriter.flat", "missing_strategy_option"} {
		t.Run(option, func(t *testing.T) {
			f := newSMTAPIFixture(t, "ALL", nil)
			var messages [2]string
			for i, newSolver := range []func() (*APISimpleSolver, error){f.c.NewSolver, f.c.NewSolverForIncrementalChecks} {
				s, err := newSolver()
				astTestMust(t, err)
				astTestMust(t, s.Assert(f.pin(f.m.MkTrue())))
				params, err := f.c.NewParameterSet()
				astTestMust(t, err)
				astTestMust(t, params.SetBool(option, true))
				if err = s.SetParameters(params); err != nil {
					messages[i] = err.Error()
				}
				if i == 1 {
					checkDeferredTactic(t, s, option == "smt.random_seed")
				}
				astTestMust(t, params.Close())
				astTestMust(t, s.Close())
			}
			if messages[0] != messages[1] || (messages[0] == "") != (option == "rewriter.flat") {
				t.Fatalf("unequal or unexpected validation for %q: %q, %q", option, messages[0], messages[1])
			}
		})
	}
}
