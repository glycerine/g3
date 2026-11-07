package smtcore

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/lp"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestSMTLRALPSettingsLocalGlobalAndFrozenInitialization(t *testing.T) {
	for _, mode := range []string{"default", "global", "local", "wrong-kind", "costs", "lu", "undecided"} {
		t.Run(mode, func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			params, err := cfg.Context.NewValue()
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
			want, pivoted := lp.StrategyTableauRows, true
			if mode == "global" || mode == "local" || mode == "wrong-kind" {
				astTestMust(t, cfg.Globals.SourceSetName("lp.simplex_strategy", "1"))
				astTestMust(t, cfg.Globals.SourceSetName("lp.bprop_on_pivoted_rows", "false"))
				want, pivoted = lp.StrategyTableauCosts, false
			}
			key := "simplex_strategy"
			switch mode {
			case "local":
				astTestMust(t, cfg.Context.SetUIntName(&params, &key, 0))
				want, pivoted = lp.StrategyTableauRows, true
			case "wrong-kind":
				astTestMust(t, cfg.Context.SetBoolName(&params, &key, true))
			case "costs":
				astTestMust(t, cfg.Context.SetUIntName(&params, &key, 1))
				want = lp.StrategyTableauCosts
			case "lu":
				astTestMust(t, cfg.Context.SetUIntName(&params, &key, 2))
				want = lp.StrategyLU
			case "undecided":
				astTestMust(t, cfg.Context.SetUIntName(&params, &key, 3))
				want = lp.StrategyUndecided
			}
			key = "bprop_on_pivoted_rows"
			if mode == "local" {
				astTestMust(t, cfg.Context.SetBoolName(&params, &key, true))
			}
			if mode == "wrong-kind" {
				astTestMust(t, cfg.Context.SetUIntName(&params, &key, 7))
			}
			k := smtTestKernel(t, m)
			astTestMust(t, k.ConfigurePreprocessing(cfg.Context, cfg.Globals, &params))
			family, err := m.ArithmeticFamilyID()
			astTestMust(t, err)
			a, err := NewSMTLRATheory(family)
			astTestMust(t, err)
			astTestMust(t, k.RegisterTheory(a))
			if a.initParams != nil {
				t.Fatal("lexical LP helper retained after initialization")
			}
			check := func() {
				t.Helper()
				if a.numeric.solver.Settings.SimplexStrategy != want || (a.numeric.solver.Core.RPrimal.PivotedRows != nil) != pivoted {
					t.Fatal("LP initialization settings", a.numeric.solver.Settings.SimplexStrategy, a.numeric.solver.Core.RPrimal.PivotedRows != nil, want, pivoted)
				}
			}
			check()
			key = "simplex_strategy"
			astTestMust(t, cfg.Context.SetUIntName(&params, &key, 0))
			key = "bprop_on_pivoted_rows"
			astTestMust(t, cfg.Context.SetBoolName(&params, &key, !pivoted))
			astTestMust(t, k.UpdateParams(&params))
			check() // Source init_solver is a no-op once its LAR solver exists.
			sort, err := m.MkFamilySort(family, 0, nil)
			astTestMust(t, err)
			f := &lraFrontendFixture{t, m, k.internalizer, a, sort}
			x, y := f.variable("lra-config-x"), f.variable("lra-config-y")
			f.assert(f.app(smtArithOpGE, f.app(smtArithOpAdd, x, y), f.number(5)), true)
			f.assert(f.app(smtArithOpLE, y, f.number(2)), true)
			f.assert(f.app(smtArithOpLE, x, f.number(4)), true)
			f.propagate(true)
			status, err := a.FinalCheck()
			astTestMust(t, err)
			if status != SMTFinalCheckDone {
				t.Fatal("configured actual LAR did not finish", status)
			}
		})
	}
}

func TestSMTLRALPSettingsRejectInvalidStrategyAndRetireHelper(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	params, err := cfg.Context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
	key := "simplex_strategy"
	astTestMust(t, cfg.Context.SetUIntName(&params, &key, 4))
	k := smtTestKernel(t, m)
	astTestMust(t, k.ConfigurePreprocessing(cfg.Context, cfg.Globals, &params))
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	a, err := NewSMTLRATheory(family)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, a.Close()) })
	if err = k.RegisterTheory(a); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("invalid strategy accepted", err)
	}
	if a.initParams != nil || a.numeric == nil || !a.numeric.stopped || k.internalizer.Theory(family) != nil {
		t.Fatal("failed initialization ownership/registration")
	}
	if _, err = a.FinalCheck(); err != lp.ErrExactVectorPending {
		t.Fatal("failed initialization was usable", err)
	}
}

func TestSMTLRALPFreshCopyUsesDestinationSettingsAndIndependentNumerics(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	target, targetCfg := smtModelGeneratorFixture(t)
	params, err := cfg.Context.NewValue()
	astTestMust(t, err)
	targetParams, err := targetCfg.Context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)); astTestMust(t, targetCfg.Context.Del(&targetParams)) })
	key := "simplex_strategy"
	astTestMust(t, cfg.Context.SetUIntName(&params, &key, 1))
	astTestMust(t, targetCfg.Context.SetUIntName(&targetParams, &key, 2))
	key = "bprop_on_pivoted_rows"
	astTestMust(t, targetCfg.Context.SetBoolName(&targetParams, &key, false))
	source, err := NewSMTSolver(m, &params, "QF_UFLRA", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg, TargetConfig: func(manager *ast.ASTManager) (SMTSolverConfig, error) {
		if manager != target {
			return SMTSolverConfig{}, ast.ErrASTDomain
		}
		return SMTSolverConfig{Params: targetCfg.Context, ModelEvaluator: targetCfg}, nil
	}})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, source.Close()) })
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	a, err := NewSMTLRATheory(family)
	astTestMust(t, err)
	// Install the still-unselected backend explicitly. Mark its setup stage
	// complete so context::copy_plugins sees the retained installed plugin.
	astTestMust(t, a.ConfigureArithmetic(source.kernel.arithState))
	astTestMust(t, source.kernel.replaceUnusedSetupArithmetic(a))
	source.kernel.preprocessingSetupDone = true
	source.kernel.copiedArithmeticPlugin = true
	sort, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	f := &lraFrontendFixture{t, m, source.kernel.internalizer, a, sort}
	x := f.variable("lra-copy-settings-x")
	astTestMust(t, source.Assert(f.app(smtArithOpGE, x, f.number(1))))
	astTestMust(t, source.Assert(f.app(smtArithOpLE, x, f.number(2))))
	status, err := source.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("source direct-installed LRA", status)
	}
	next, err := source.Translate(target, &targetParams)
	astTestMust(t, err)
	copied := next.(*SMTSolver)
	t.Cleanup(func() { astTestMust(t, copied.Close()) })
	targetFamily, err := target.ArithmeticFamilyID()
	astTestMust(t, err)
	fresh, ok := copied.kernel.internalizer.Theory(targetFamily).(*SMTLRATheory)
	if !ok || fresh == a || fresh.numeric == a.numeric || fresh.numerals == a.numerals || fresh.numeric.solver == a.numeric.solver || fresh.NumVariables() != 0 {
		t.Fatal("copy did not create independent LRA owners", ok)
	}
	if fresh.numeric.solver.Settings.SimplexStrategy != lp.StrategyLU || fresh.numeric.solver.Core.RPrimal.PivotedRows != nil || a.numeric.solver.Settings.SimplexStrategy != lp.StrategyTableauCosts {
		t.Fatal("fresh copy reused source LP settings")
	}
	// Destination arithmetic mode would choose another backend. The copied
	// concrete plugin must survive that setup and replay assertions into LAR.
	status, err = copied.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True || copied.kernel.internalizer.Theory(targetFamily) != fresh {
		t.Fatal("copied LRA replaced during destination setup", status)
	}
	astTestMust(t, source.Close())
	astTestMust(t, copied.Push())
	tsort, err := target.MkFamilySort(targetFamily, 0, nil)
	astTestMust(t, err)
	tf := &lraFrontendFixture{t, target, copied.kernel.internalizer, fresh, tsort}
	tx := tf.variable("lra-copy-settings-x")
	astTestMust(t, copied.Assert(tf.app(smtArithOpLE, tx, tf.number(0))))
	status, err = copied.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("copy lost parent bounds after source Close", status)
	}
	astTestMust(t, copied.Pop(1))
	status, err = copied.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("copied scope recovery", status)
	}
	value, err := fresh.numeric.value(copied.kernel.internalizer.context.enodes.get(tx).TheoryVar(targetFamily))
	astTestMust(t, err)
	if value.Main.Cmp(big.NewRat(1, 1)) < 0 || value.Main.Cmp(big.NewRat(2, 1)) > 0 {
		t.Fatal("copy numeric value outside retained bounds", value)
	}
}

func TestSMTLRALPInitializationAllocationRefusalCleanup(t *testing.T) {
	failures, successes := 0, 0
	for _, stop := range []int64{-1, 0, 1, 2, 3, 5, 8, 12, 24} {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			k := smtTestKernel(t, m)
			astTestMust(t, k.ConfigurePreprocessing(cfg.Context, cfg.Globals, cfg.Params))
			family, err := m.ArithmeticFamilyID()
			astTestMust(t, err)
			a, err := NewSMTLRATheory(family)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, a.Close()) })
			m.Budget().FailAfter(stop)
			err = k.RegisterTheory(a)
			m.Budget().FailAfter(-1)
			if err != nil {
				var failure umemory.Failure
				memout := errors.As(err, &failure) && failure.Kind == umemory.FailureMemout && failure.ErrorCode() == 101 && stop >= 0
				if !errors.Is(err, arena.ErrInjectedFailure) && !memout {
					t.Fatal("unexpected initialization refusal", err)
				}
				failures++
				if k.internalizer.Theory(family) != nil {
					t.Fatal("failed LRA remained registered")
				}
			} else {
				successes++
			}
			astTestMust(t, a.Close())
			if a.initParams != nil || a.numeric != nil || a.numerals != nil || a.constructorInteger != nil {
				t.Fatal("initialization cleanup retained child owners")
			}
		})
	}
	if failures == 0 || successes == 0 {
		t.Fatal("allocation samples need refusal and success", failures, successes)
	}
}
