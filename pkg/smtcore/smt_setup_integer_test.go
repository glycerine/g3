package smtcore

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTQFLIAAutomaticSetupUsesFeaturesAtFirstBoundary(t *testing.T) {
	for _, profile := range []string{"units", "binary", "large-binary", "deep-ite", "first-push", "disabled"} {
		t.Run(profile, func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			solver, err := NewSMTSolver(m, nil, "QF_LIA", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			if profile == "disabled" {
				smtDisableAutoConfig(t, solver)
			}
			zero, err := m.ArithmeticIntegerNumeralText("0")
			astTestMust(t, err)
			integer, err := m.GetSort(zero)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "setup-integer-x"), integer)
			astTestMust(t, err)
			family, err := m.ArithmeticFamilyID()
			astTestMust(t, err)
			right := zero
			if profile == "large-binary" {
				right, err = m.ArithmeticIntegerNumeralText("100001")
				astTestMust(t, err)
			}
			if profile == "deep-ite" {
				boolean, err := m.MkBoolSort()
				astTestMust(t, err)
				for index := uint32(0); index < 52; index++ {
					condition, err := m.MkConst(symbol.Numerical(2800+index), boolean)
					astTestMust(t, err)
					right, err = m.MkIte(condition, x, right)
					astTestMust(t, err)
				}
			}
			formula, err := m.MkFamilyApp(family, smtArithOpGE, nil, []ast.ASTHandle{x, right})
			astTestMust(t, err)
			if profile == "binary" || profile == "large-binary" {
				condition := smtRelevancyBool(t, m, "setup-clause-condition")
				formula, err = m.MkOr(condition, formula)
				astTestMust(t, err)
			}
			ref, err := m.NewRef(formula)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, m.DelRef(&ref)) })
			astTestMust(t, solver.Assert(formula))
			if profile == "first-push" {
				astTestMust(t, solver.Push())
			}
			status, err := smtSetupProfileCheck(solver)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("setup changed satisfiability", status)
			}
			k := solver.kernel
			owner, err := k.smtParams.ArithmeticState()
			astTestMust(t, err)
			theory := k.internalizer.theories[family].(*SMTTheoryArith)
			if profile == "disabled" {
				if owner.Eq2Ineq || !owner.Reflect || !owner.PropagateEqs || k.smtState.RelevancyLvl != 2 {
					t.Fatal("disabled auto configuration applied a logic profile", owner, k.smtState)
				}
				return
			}
			if owner != k.arithState || owner.Reflect || k.preprocessState.NNFCNF || k.smtParams.preprocess.state.NNFCNF {
				t.Fatal("setup owner and retained caches diverged")
			}
			if theory.params.GCDTest != owner.GCDTest || theory.params.PropagateEqs != owner.PropagateEqs || theory.params.BranchCutRatio != owner.BranchCutRatio || theory.params.StrongerLemmas != owner.StrongerLemmas {
				t.Fatal("arithmetic theory did not receive setup policy")
			}
			if k.relevancy.enabled != (k.smtState.RelevancyLvl > 0) || k.internalizer.relevancyLevel != k.smtState.RelevancyLvl || k.smtParams.state.RelevancyLvl != k.smtState.RelevancyLvl {
				t.Fatal("relevancy policy owner and consumers diverged")
			}
			switch profile {
			case "units":
				if owner.GCDTest || owner.BranchCutRatio != 4 || !owner.Eq2Ineq || owner.PropagateEqs || k.smtState.RelevancyLvl != 2 || !k.preprocessState.EliminateTermITE {
					t.Fatal("unit-only profile", owner, k.smtState)
				}
			case "first-push":
				if !owner.GCDTest || owner.BranchCutRatio != 2 || !owner.Eq2Ineq || owner.PropagateEqs || k.smtState.RelevancyLvl != 0 {
					t.Fatal("first push did not preserve the non-feature logic profile", owner)
				}
				astTestMust(t, solver.Pop(1))
			case "deep-ite":
				if owner.Eq2Ineq || !owner.PropagateEqs || !owner.GCDTest || !k.preprocessState.PullCheapITE || k.smtState.RelevancyLvl != 2 {
					t.Fatal("deep ITE profile", owner)
				}
			case "binary", "large-binary":
				if !owner.Eq2Ineq || owner.PropagateEqs || k.smtState.RelevancyLvl != 0 || k.smtState.PhaseSelection != 2 || k.smtState.RestartAdaptive || k.smtState.RestartStrategy != 0 || k.smtState.RestartFactor != 1.5 {
					t.Fatal("clausal profile", owner, k.smtState)
				}
				if profile == "large-binary" && (owner.BoundProp != 0 || owner.StrongerLemmas) {
					t.Fatal("large-coefficient binary profile", owner)
				}
			}
			// Setup is permanent across checks and user scopes. The next
			// unit assertion must not reclassify a previously clausal input.
			before := k.smtState.RelevancyLvl
			astTestMust(t, solver.Push())
			astTestMust(t, solver.Assert(formula))
			status, err = smtSetupProfileCheck(solver)
			astTestMust(t, err)
			astTestMust(t, solver.Pop(1))
			if status != lbool.True || k.smtState.RelevancyLvl != before {
				t.Fatal("later scope reran automatic setup", status)
			}
		})
	}
}

func TestSMTQFLIAStaticSetupRejectsUninterpretedFunctions(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	solver, err := NewSMTSolver(m, nil, "QF_LIA", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	integer, err := m.GetSort(zero)
	astTestMust(t, err)
	predicate := smtQuickPredicate(t, m, "setup-disallowed-predicate", integer, boolean)
	formula, err := m.MkApp(predicate, []ast.ASTHandle{zero})
	astTestMust(t, err)
	astTestMust(t, solver.Assert(formula))
	_, err = smtSetupProfileCheck(solver)
	if err == nil || !strings.Contains(err.Error(), "uninterpreted function symbols") {
		t.Fatal("static QF_LIA validation accepted an uninterpreted function", err)
	}
	if solver.kernel.preprocessingSetupDone {
		t.Fatal("failed feature validation committed setup")
	}
}
