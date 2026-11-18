package smtcore

// Source: src/muz/pdr/pdr_context.cpp pre-search arithmetic fparams tuning.

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTSolverArithmeticStateOverrideSelectsBackendAndFreezesAtSetup(t *testing.T) {
	manager, services := smtModelGeneratorFixture(t)
	params, err := services.Context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, services.Context.Del(&params)) })
	automatic := "auto_config"
	astTestMust(t, services.Context.SetBoolName(&params, &automatic, false))
	solver, err := NewSMTSolver(manager, &params, "ALL", SMTSolverConfig{Params: services.Context, ModelEvaluator: services})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })

	state, err := solver.ArithmeticState()
	astTestMust(t, err)
	if state.Mode != SMTArithmeticSimplex {
		t.Fatal("unexpected initial arithmetic mode", state.Mode)
	}
	state.Mode = SMTArithmeticUTVPI
	state.BoundProp = 0
	state.AutoConfigSimplex = true
	state.PropagateEqs = false
	state.EagerEqAxioms = false
	state.Eq2Ineq = true
	astTestMust(t, solver.SetArithmeticState(state))
	got, err := solver.ArithmeticState()
	astTestMust(t, err)
	if got != state || solver.kernel.smtParams.arith.state != state {
		t.Fatal("arithmetic override was not installed in both setup snapshots")
	}

	family, err := manager.RegisterArithmeticPlugin()
	astTestMust(t, err)
	sort, err := manager.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	x, err := manager.MkConst(astTestSymbol(t, manager, "state-override-x"), sort)
	astTestMust(t, err)
	zero, err := bigTestArithNumeral(manager, new(big.Rat), true)
	astTestMust(t, err)
	bound, err := manager.MkFamilyApp(family, smtArithOpGE, nil, []ast.ASTHandle{x, zero})
	astTestMust(t, err)
	astTestMust(t, solver.Assert(bound))
	status, err := smtSetupProfileCheck(solver)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("UTVPI override check", status)
	}
	if _, ok := solver.kernel.internalizer.theories[family].(*SMTUTVPITheory); !ok {
		t.Fatal("arithmetic override did not select UTVPI")
	}
	late := state
	late.Mode = SMTArithmeticDifferenceLogic
	if err = solver.SetArithmeticState(late); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("late arithmetic override", err)
	}
	got, err = solver.ArithmeticState()
	astTestMust(t, err)
	if got != state {
		t.Fatal("refused late override changed state")
	}
}

func TestSMTSolverArithmeticStateRejectsInvalidModeWithoutMutation(t *testing.T) {
	manager, services := smtModelGeneratorFixture(t)
	solver, err := NewSMTSolver(manager, nil, "", SMTSolverConfig{Params: services.Context, ModelEvaluator: services})
	astTestMust(t, err)
	original, err := solver.ArithmeticState()
	astTestMust(t, err)
	invalid := original
	invalid.Mode = SMTArithmeticOptInf + 1
	if err = solver.SetArithmeticState(invalid); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("invalid arithmetic mode", err)
	}
	got, err := solver.ArithmeticState()
	astTestMust(t, err)
	if got != original {
		t.Fatal("invalid arithmetic mode changed state")
	}
	astTestMust(t, solver.Close())
	if _, err = solver.ArithmeticState(); err == nil {
		t.Fatal("closed arithmetic state getter succeeded")
	}
	if err = solver.SetArithmeticState(original); err == nil {
		t.Fatal("closed arithmetic state setter succeeded")
	}
}
