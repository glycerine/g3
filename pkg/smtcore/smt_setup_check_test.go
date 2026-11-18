package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Setup-profile tests deliberately exercise the static overload on a fresh
// context. After a push or an earlier check they use ordinary incremental check.
func smtSetupProfileCheck(s *SMTSolver) (lbool.LBool, error) {
	if s.kernel.preprocessingSetupDone {
		return s.CheckSat(nil)
	}
	return s.checkSatCore(nil, true)
}

func smtSetupCheckBound(t *testing.T, m *ast.ASTManager) ast.ASTHandle {
	t.Helper()
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	sort, err := m.GetSort(zero)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "setup-check-x"), sort)
	astTestMust(t, err)
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	bound, err := m.MkFamilyApp(family, smtArithOpGE, nil, []ast.ASTHandle{x, zero})
	astTestMust(t, err)
	return bound
}

func TestSMTSetupAndCheckSeparatesStaticAndIncrementalModes(t *testing.T) {
	for _, mode := range []string{"static", "ordinary", "assumptions", "push", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			s, err := NewSMTSolver(m, nil, "QF_LIA", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, s.Close()) })
			if mode == "disabled" {
				smtDisableAutoConfig(t, s)
			}
			astTestMust(t, s.Assert(smtSetupCheckBound(t, m)))
			if mode == "push" {
				astTestMust(t, s.Push())
			}
			var status lbool.LBool
			switch mode {
			case "static", "disabled":
				status, err = s.checkSatCore(nil, true)
			case "ordinary":
				status, err = s.kernel.Check()
			default:
				status, err = s.CheckSat(nil)
			}
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("nonnegative integer must be satisfiable", status)
			}
			k := s.kernel
			wantChecks := uint64(1)
			if mode == "static" || mode == "disabled" {
				wantChecks = 0
			}
			if uint64(k.Statistics().NumChecks) != wantChecks {
				t.Fatal("setup_and_check must not increment regular check statistics", k.Statistics())
			}
			wantRelevancy := uint32(0)
			if mode == "static" || mode == "disabled" {
				wantRelevancy = 2
			}
			if k.smtState.RelevancyLvl != wantRelevancy || k.arithState.GCDTest != (mode != "static") {
				t.Fatal("wrong source setup overload", mode, k.smtState.RelevancyLvl, k.arithState.GCDTest)
			}
			// SASSERT(!m_setup.already_configured()) in a debug build; a
			// release setup_and_check skips the configured setup_context.
			if status, err := k.SetupAndCheck(); smtcoreSourceDebug && err != ast.ErrASTDomain || !smtcoreSourceDebug && (err != nil || status != lbool.True) {
				t.Fatal("setup_and_check on an already configured context", status, err)
			}
		})
	}
}

func TestSMTTacticSelectsSetupOnlyWithoutExtractedAssumptions(t *testing.T) {
	for _, core := range []bool{false, true} {
		t.Run(map[bool]string{false: "static", true: "assumptions"}[core], func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			created, err := NewSMTTactic(m, nil, SMTSolverConfig{Params: cfg.Context, Globals: cfg.Globals, ModelEvaluator: cfg})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, created.Close()) })
			astTestMust(t, created.SetLogic(astTestSymbol(t, m, "QF_LIA")))
			goal, err := tactic.NewGoal(m, false, true, core)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, goal.Close()) })
			bound := smtSetupCheckBound(t, m)
			var dep ast.ASTDependency
			if core {
				dep, err = m.DependencyLeaf(smtRelevancyBool(t, m, "setup-check-dependency"))
				astTestMust(t, err)
			}
			astTestMust(t, goal.AssertWithProof(bound, ast.ASTHandle{}, dep))
			calls := 0
			astTestMust(t, created.SetProgressCallback(func(kind uint32) {
				if kind != 0 {
					return
				}
				calls++
				k := created.active.kernel
				want := uint32(2)
				checks := uint32(0)
				if core {
					want, checks = 0, 1
				}
				if k.smtState.RelevancyLvl != want || uint32(k.Statistics().NumChecks) != checks {
					t.Fatal("tactic took wrong source setup/check branch", k.smtState.RelevancyLvl, k.Statistics())
				}
			}))
			var result tactic.TacticResult
			t.Cleanup(func() { astTestMust(t, result.Close()) })
			astTestMust(t, created.Apply(goal, &result))
			if calls == 0 || !goal.IsDecidedSat() || result.ModelConverter == nil {
				t.Fatal("tactic lost SAT/model publication", calls)
			}
		})
	}
}

func TestSMTSetupAndCheckRepeatedEmptyStandaloneContext(t *testing.T) {
	m := astTestManager(t)
	k := smtTestKernel(t, m)
	status, err := k.SetupAndCheck()
	astTestMust(t, err)
	if status != lbool.True || k.Statistics().NumChecks != 0 {
		t.Fatal("empty static setup", status, k.Statistics())
	}
	if status, err := k.SetupAndCheck(); smtcoreSourceDebug && err != ast.ErrASTDomain || !smtcoreSourceDebug && (err != nil || status != lbool.True) {
		t.Fatal("repeated setup without params", status, err)
	}
}

func TestSMTCheckAssumptionValidationFollowsSetupAndPrecedesInternalization(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	s, err := NewSMTSolver(m, nil, "QF_LIA", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	astTestMust(t, s.Assert(smtSetupCheckBound(t, m)))
	k := s.kernel
	before := len(k.internalizer.booleanData)
	a := smtRelevancyBool(t, m, "setup-invalid-assumption-a")
	b := smtRelevancyBool(t, m, "setup-invalid-assumption-b")
	invalid, err := m.MkOr(a, b)
	astTestMust(t, err)
	astTestPin(t, m, invalid)
	status, err := s.CheckSat([]ast.ASTHandle{invalid})
	if err != nil || status != lbool.Undef || !k.preprocessingSetupDone || !k.internalizer.theoriesSetup || k.smtState.RelevancyLvl != 0 {
		t.Fatal("invalid assumption did not follow logic setup", err, k.preprocessingSetupDone, k.smtState.RelevancyLvl)
	}
	if k.assertedHead != 0 || len(k.internalizer.booleanData) != before || k.Statistics().NumChecks != 1 {
		t.Fatal("invalid assumption internalized the pending assertions", k.assertedHead, k.Statistics())
	}
	status, err = s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True || k.Statistics().NumChecks != 2 {
		t.Fatal("valid retry after invalid assumption", status, k.Statistics())
	}
}
