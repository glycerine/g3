package sat

// Source: src/sat/sat_solver.cpp:805-896.

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func satParallelPortfolioFixture(t *testing.T, lanes uint32) *Solver {
	t.Helper()
	s, context, params := satSolverCheckFixture(t)
	name := "parallel_threads"
	satOK(t, context.SetUIntName(&params, &name, lanes))
	// sat_config names the source field num_parallel through the public
	// threads parameter.
	satOK(t, s.UpdateParams(&params))
	state, err := s.config.State()
	satOK(t, err)
	if state.NumParallel != lanes {
		t.Fatal("parallel source parameter", state.NumParallel, lanes)
	}
	return s
}

func satParallelAssertModel(t *testing.T, s *Solver, variable BoolVar, want lbool.LBool) {
	t.Helper()
	model, err := s.AppendModel(nil)
	satOK(t, err)
	if uint32(variable) >= uint32(len(model)) || model[variable] != want {
		t.Fatal("parallel winner model", model, variable, want)
	}
}

func satParallelPhase(t *testing.T, s *Solver) usymbol.Symbol {
	t.Helper()
	name := "phase"
	phase, err := s.context.GetSymbolName(&s.params, &name, s.config.cached[2])
	satOK(t, err)
	return phase
}
