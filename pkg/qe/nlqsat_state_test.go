package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/nlsat"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func nlqStateFixture(t *testing.T) (*nlsat.Solver, []sat.Literal) {
	t.Helper()
	f := newFMFixture(t)
	return nlqSolverWithFixture(t, f)
}

func nlqSolverWithFixture(t *testing.T, f *fmFixture) (*nlsat.Solver, []sat.Literal) {
	t.Helper()
	solver := nlqBareSolverWithFixture(t, f)
	pm, err := solver.PolynomialManager()
	fmMust(t, err)
	core, err := pm.ArithmeticCore()
	fmMust(t, err)
	literals := make([]sat.Literal, 3)
	for i := uint32(0); i < 3; i++ {
		variable, cause := solver.MkVar(false)
		fmMust(t, cause)
		if variable != i {
			t.Fatal("variable order", variable, i)
		}
		p, cause := core.MkVariable(i, 1)
		fmMust(t, cause)
		fmMust(t, core.IncRef(p))
		t.Cleanup(func() { fmMust(t, core.DecRef(p)) })
		literals[i], cause = solver.MkInequality(nlsat.AtomGT, []polynomial.Polynomial{p}, []bool{false}, false)
		fmMust(t, cause)
		fmMust(t, solver.AddClause([]sat.Literal{literals[i]}, nlsat.Assumption{}))
	}
	return solver, literals
}

func nlqBareSolverWithFixture(t *testing.T, f *fmFixture) *nlsat.Solver {
	t.Helper()
	am, err := f.r.manager.ArithmeticAlgebraicManager()
	fmMust(t, err)
	rationals, err := am.RationalManager()
	fmMust(t, err)
	pm, err := polynomial.NewMultivariatePolynomialManager(polynomial.MultivariatePolynomialManagerConfig{
		Core: polynomial.MultivariatePolynomialConfig{
			Integer: rationals.Integers(), Rationals: rationals,
			Checkpoint:  f.r.manager.Limit.Inc,
			Cooperation: func() error { return f.r.config.Cooperation.Cooperate("nlqsat") },
		},
	})
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, pm.CloseChecked()) })
	solver, err := nlsat.NewSolver(nlsat.SolverConfig{Algebraic: am, Polynomial: pm, Limit: &f.r.manager.Limit, SimplifyCore: true})
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, solver.Close()) })
	return solver
}

func TestNLQStateModelScopesAssumptionsAndPredicateLevels(t *testing.T) {
	solver, literals := nlqStateFixture(t)
	truthVariable, err := solver.MkBoolVar()
	fmMust(t, err)
	truth := sat.MakeLiteral(truthVariable, false)
	fmMust(t, solver.AddClause([]sat.Literal{truth}, nlsat.Assumption{}))
	status, err := solver.Check()
	fmMust(t, err)
	if status != lbool.True {
		t.Fatal("state fixture is not satisfiable", status)
	}
	levels := []MaxLevel{
		{Existential: 0, Universal: NoQuantifierLevel},
		{Existential: NoQuantifierLevel, Universal: 1},
		{Existential: 2, Universal: NoQuantifierLevel},
	}
	state, err := newNLQState(solver, levels)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, state.close()) })
	for i, literal := range literals {
		level, cause := state.getLevel(literal)
		fmMust(t, cause)
		if level != levels[i] {
			t.Fatal("predicate level", i, level, levels[i])
		}
	}
	fmMust(t, state.saveModel())
	fmMust(t, state.push())
	assumptions, err := state.initAssumptions(truth, nil)
	fmMust(t, err)
	if len(assumptions) != 3 || assumptions[0] != truth.Negated() || assumptions[1] != literals[0] || assumptions[2] != literals[2] {
		t.Fatal("universal-level assumptions", assumptions)
	}
	fmMust(t, state.push())
	assumptions, err = state.initAssumptions(truth, nil)
	fmMust(t, err)
	if len(assumptions) != 3 || assumptions[0] != truth || assumptions[1] != literals[0] || assumptions[2] != literals[1] {
		t.Fatal("existential-level assumptions", assumptions)
	}
	fmMust(t, state.pop(1))
	if state.level() != 1 || state.validModel || len(state.cached) != 1 || state.cached[0] != literals[0] {
		t.Fatal("scope pop did not restore cached prefix", state.level(), state.validModel, state.cached)
	}
	if !state.answerValid {
		t.Fatal("existential answer model was cleared by backtracking")
	}
}
