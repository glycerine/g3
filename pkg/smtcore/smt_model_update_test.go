package smtcore

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func smtModelUpdateFixture(t *testing.T, integer bool) (*SMTSolver, *arithmeticObjectiveFixture) {
	t.Helper()
	m, cfg := smtModelGeneratorFixture(t)
	p, err := cfg.Context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&p)) })
	key := "auto_config"
	astTestMust(t, cfg.Context.SetBoolName(&p, &key, false))
	key = "arith.solver"
	astTestMust(t, cfg.Context.SetUIntName(&p, &key, 2))
	logic := "QF_UFLRA"
	if integer {
		logic = "QF_LIA"
	}
	s, err := NewSMTSolver(m, &p, logic, SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	s.kernel.preprocess = false
	astTestMust(t, s.SetProduceModels(true))
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	kind := int32(0)
	if integer {
		kind = 1
	}
	sort, err := m.MkFamilySort(family, kind, nil)
	astTestMust(t, err)
	return s, &arithmeticObjectiveFixture{t: t, m: m, s: s.kernel.internalizer, sort: sort, integer: integer}
}

func startSMTModelUpdate(t *testing.T, s *SMTSolver, f *arithmeticObjectiveFixture) {
	t.Helper()
	astTestMust(t, s.Push())
	family, err := f.m.ArithmeticFamilyID()
	astTestMust(t, err)
	var ok bool
	f.a, ok = s.kernel.internalizer.theories[family].(*SMTTheoryArith)
	if !ok {
		t.Fatal("expected retained arithmetic mode 2")
	}
}

func modelUpdateNumber(t *testing.T, m *ast.ASTManager, md *model.ModelCore, x ast.ASTHandle) string {
	t.Helper()
	decl, err := m.GetDecl(x)
	astTestMust(t, err)
	h, err := md.ConstantInterpretation(decl)
	astTestMust(t, err)
	text, _, ok, err := m.ArithmeticNumeralText(h)
	astTestMust(t, err)
	if !ok {
		t.Fatal("missing numeric model interpretation", h)
	}
	return text
}

func acquireUpdateModel(t *testing.T, s *SMTSolver) *model.ModelCore {
	t.Helper()
	md, err := s.AcquireModel()
	astTestMust(t, err)
	if md == nil {
		t.Fatal("missing published model")
	}
	t.Cleanup(func() { astTestMust(t, md.DecRef()) })
	return md
}

func TestSMTUpdateModelPublishesOptimizedAssignmentAndKeepsOldOwner(t *testing.T) {
	for _, integer := range []bool{false, true} {
		for _, refinalize := range []bool{false, true} {
			t.Run(fmt.Sprint(integer, "/", refinalize), func(t *testing.T) {
				s, f := smtModelUpdateFixture(t, integer)
				startSMTModelUpdate(t, s, f)
				x := f.variable("update-model-x")
				obj := f.app(smtArithOpAdd, f.app(smtArithOpMul, f.number("2"), x), f.number("1"))
				// Preprocessing is off: assert the rewriter's normal form of x < 5.
				bound := f.pin(f.m.MkNot(f.app(smtArithOpGE, x, f.number("5"))))
				if integer {
					bound = f.app(smtArithOpLE, f.app(smtArithOpMul, f.number("2"), x), f.number("7"))
				}
				astTestMust(t, s.Assert(bound))
				v, err := f.a.AddObjective(obj)
				astTestMust(t, err)
				status, err := s.CheckSat(nil)
				astTestMust(t, err)
				if status != lbool.True {
					t.Fatal(status)
				}
				old := acquireUpdateModel(t, s)
				oldValue := modelUpdateNumber(t, f.m, old, x)
				_, blocker, shared, err := f.a.MaximizeObjective(v)
				astTestMust(t, err)
				f.pin(blocker, nil)
				if shared {
					t.Fatal("unshared optimization requests combination checking")
				}
				beforeChecks, beforeFinal := f.s.statistics.NumChecks, f.s.statistics.NumFinalChecks
				result := s.result
				updated, err := s.UpdateModel(refinalize)
				astTestMust(t, err)
				if !updated {
					t.Fatal("optimized model was not refreshed")
				}
				fresh := acquireUpdateModel(t, s)
				want := "4"
				if integer {
					want = "3"
				}
				if fresh == old || modelUpdateNumber(t, f.m, fresh, x) != want || modelUpdateNumber(t, f.m, old, x) != oldValue {
					t.Fatal("model refresh reused or mutated previous model")
				}
				ev, err := model.NewModelEvaluator(fresh, s.kernel.modelConfig)
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, ev.Close()) })
				r, err := ev.EvaluateValue(obj)
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, f.m.DelRef(&r)) })
				h, err := f.m.RefGet(r)
				astTestMust(t, err)
				text, _, numeric, err := f.m.ArithmeticNumeralText(h)
				astTestMust(t, err)
				wantObjective := "9"
				if integer {
					wantObjective = "7"
				}
				if !numeric || text != wantObjective {
					t.Fatal("refreshed compound objective evaluation", text, wantObjective)
				}
				if s.kernel.GeneratedModel() != fresh || s.result != result || f.s.statistics.NumChecks != beforeChecks {
					t.Fatal("model refresh replaced check result or ran a search")
				}
				delta := uint32(0)
				if refinalize {
					delta = 1
				}
				if f.s.statistics.NumFinalChecks != beforeFinal+delta {
					t.Fatal("wrong final-check count", f.s.statistics.NumFinalChecks, beforeFinal)
				}
			})
		}
	}
}

func TestSMTUpdateModelBypassesStaleAcceptedCandidate(t *testing.T) {
	s, f := smtModelUpdateFixture(t, false)
	startSMTModelUpdate(t, s, f)
	x := f.variable("update-candidate-x")
	astTestMust(t, s.Assert(f.app(smtArithOpLE, x, f.number("5"))))
	v, err := f.a.AddObjective(x)
	astTestMust(t, err)
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	q := f.s.quantifierManager
	if q == nil {
		t.Fatal("missing candidate owner")
	}
	stale, err := s.kernel.buildCandidateModelGenerator()
	astTestMust(t, err)
	q.acceptedModel = stale
	_, _, _, err = f.a.MaximizeObjective(v)
	astTestMust(t, err)
	updated, err := s.UpdateModel(false)
	astTestMust(t, err)
	if !updated || q.acceptedModel != nil || !stale.closed {
		t.Fatal("stale accepted candidate survived refresh")
	}
	md := acquireUpdateModel(t, s)
	if modelUpdateNumber(t, f.m, md, x) != "5" {
		t.Fatal("old candidate published after optimization")
	}
}

func TestSMTUpdateModelConstructionRefusalPreservesPublishedModel(t *testing.T) {
	succeeded := false
	for stop := int64(0); stop < 128 && !succeeded; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			s, f := smtModelUpdateFixture(t, false)
			startSMTModelUpdate(t, s, f)
			x := f.variable("update-refusal-x")
			astTestMust(t, s.Assert(f.app(smtArithOpLE, x, f.number("5"))))
			v, err := f.a.AddObjective(x)
			astTestMust(t, err)
			status, err := s.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal(status)
			}
			old := acquireUpdateModel(t, s)
			oldValue := modelUpdateNumber(t, f.m, old, x)
			_, _, _, err = f.a.MaximizeObjective(v)
			astTestMust(t, err)
			// Cross actual model construction/publication reservations; the first
			// is NewProtoModel's model.NewModel owner, not a warmed AST operation.
			f.m.Budget().FailAfter(stop)
			updated, err := s.UpdateModel(false)
			f.m.Budget().FailAfter(-1)
			still := acquireUpdateModel(t, s)
			if err != nil {
				if updated || !errors.Is(err, arena.ErrInjectedFailure) {
					t.Fatal("unexpected model refresh refusal", updated, err)
				}
				if still != old || s.kernel.GeneratedModel() != old || modelUpdateNumber(t, f.m, still, x) != oldValue {
					t.Fatal("refused model refresh lost previous publication")
				}
			} else {
				succeeded = true
				if !updated || still == old || modelUpdateNumber(t, f.m, still, x) != "5" {
					t.Fatal("successful refresh did not publish optimized value")
				}
			}
			if modelUpdateNumber(t, f.m, old, x) != oldValue {
				t.Fatal("old model owner changed during refresh")
			}
			updated, err = s.UpdateModel(false)
			astTestMust(t, err)
			if !updated || modelUpdateNumber(t, f.m, acquireUpdateModel(t, s), x) != "5" {
				t.Fatal("model refresh did not recover")
			}
		})
		if t.Failed() {
			t.Fatalf("model refresh allocation frontier %d", stop)
		}
	}
	if !succeeded {
		t.Fatal("model refresh never reached a complete allocation frontier")
	}
}

func TestSMTUpdateModelSharedArithmeticRequestsContinuation(t *testing.T) {
	s, f := smtModelUpdateFixture(t, false)
	startSMTModelUpdate(t, s, f)
	x := f.variable("update-shared-x")
	five := f.number("5")
	decl, err := f.m.MkFuncDecl(astTestSymbol(t, f.m, "update-shared-f"), []ast.ASTHandle{f.sort}, f.sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	fx := f.pin(f.m.MkApp(decl, []ast.ASTHandle{x}))
	f5 := f.pin(f.m.MkApp(decl, []ast.ASTHandle{five}))
	eq := f.pin(f.m.MkEq(fx, f5))
	astTestMust(t, s.Assert(f.pin(f.m.MkNot(eq))))
	astTestMust(t, s.Assert(f.app(smtArithOpLE, x, five)))
	v, err := f.a.AddObjective(x)
	astTestMust(t, err)
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	old := acquireUpdateModel(t, s)
	oldValue := modelUpdateNumber(t, f.m, old, x)
	value, _, shared, err := f.a.MaximizeObjective(v)
	astTestMust(t, err)
	checkArithmeticObjective(t, value, "5", "0")
	if !shared {
		t.Fatal("missing shared arithmetic movement")
	}
	checks := f.s.statistics.NumChecks
	// update_model(refinalize) runs final_check: theory_arith's liberal
	// final check mutates the shared assignment (mutate_assignment and
	// random_update move x below its bound) and assume_eqs splits on any
	// remaining equal shared values. Either the check continues and nothing
	// is published, or the published model satisfies f(x) != f(5).
	updated, err := s.UpdateModel(shared)
	astTestMust(t, err)
	if f.s.statistics.NumChecks != checks {
		t.Fatal("update_model ran another search")
	}
	still := acquireUpdateModel(t, s)
	if !updated {
		if old != still || s.kernel.GeneratedModel() != old || modelUpdateNumber(t, f.m, still, x) != oldValue {
			t.Fatal("continuation changed publication")
		}
		return
	}
	published, ok := new(big.Rat).SetString(modelUpdateNumber(t, f.m, still, x))
	if !ok || published.Cmp(big.NewRat(5, 1)) >= 0 {
		t.Fatal("published model violates x <= 5 and f(x) != f(5)", published)
	}
}

type smtUpdateErrorTheory struct {
	*smtInternalizerTestTheory
	finalError, modelError error
}

func (a *smtUpdateErrorTheory) FinalCheck() (SMTFinalCheckStatus, error) {
	a.finalCheckCalls++
	return a.finalStatus, a.finalError
}

func (a *smtUpdateErrorTheory) FinalizeModel(*SMTModelGenerator) error {
	a.finalizeModelCalls++
	return a.modelError
}

func TestSMTUpdateModelFinalCheckStatusesAndErrors(t *testing.T) {
	for _, mode := range []string{"continue", "giveup", "final-error", "model-error", "skip-final"} {
		t.Run(mode, func(t *testing.T) {
			s, f := smtModelUpdateFixture(t, false)
			base, err := NewSMTTheoryBase(ast.FamilyID(73))
			astTestMust(t, err)
			hook := &smtUpdateErrorTheory{smtInternalizerTestTheory: &smtInternalizerTestTheory{SMTTheoryBase: base}}
			astTestMust(t, s.kernel.RegisterTheory(hook))
			startSMTModelUpdate(t, s, f)
			x := f.variable("update-status-x")
			astTestMust(t, s.Assert(f.app(smtArithOpLE, x, f.number("5"))))
			v, err := f.a.AddObjective(x)
			astTestMust(t, err)
			status, err := s.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal(status)
			}
			old := acquireUpdateModel(t, s)
			oldValue := modelUpdateNumber(t, f.m, old, x)
			_, _, _, err = f.a.MaximizeObjective(v)
			astTestMust(t, err)
			failure := errors.New("selected model refresh failure")
			switch mode {
			case "continue":
				hook.finalStatus = SMTFinalCheckContinue
			case "giveup":
				hook.finalStatus = SMTFinalCheckGiveUp
			case "final-error", "skip-final":
				hook.finalError = failure
			case "model-error":
				hook.modelError = failure
			}
			calls, models := hook.finalCheckCalls, hook.finalizeModelCalls
			updated, err := s.UpdateModel(mode != "skip-final")
			if mode == "final-error" || mode == "model-error" {
				if !errors.Is(err, failure) {
					t.Fatal("model refresh error was lost", err)
				}
			} else {
				astTestMust(t, err)
			}
			if mode == "skip-final" {
				if !updated || hook.finalCheckCalls != calls || modelUpdateNumber(t, f.m, acquireUpdateModel(t, s), x) != "5" {
					t.Fatal("disabled final check still called the theory or retained old publication")
				}
			} else {
				if updated || hook.finalCheckCalls != calls+1 || s.kernel.GeneratedModel() != old || acquireUpdateModel(t, s) != old || modelUpdateNumber(t, f.m, old, x) != oldValue {
					t.Fatal("non-DONE/error refresh changed model ownership")
				}
				if mode != "model-error" && hook.finalizeModelCalls != models {
					t.Fatal("non-DONE/error final check entered model construction")
				}
			}
			if mode == "giveup" {
				failure, err := s.kernel.LastFailure()
				astTestMust(t, err)
				if failure != SMTFailureTheory || len(f.s.incompleteTheories) != 1 {
					t.Fatal("theory give-up did not preserve native failure state", failure)
				}
			}
		})
	}
}

func TestSMTUpdateModelRequiresModelEnabledSATResult(t *testing.T) {
	s, f := smtModelUpdateFixture(t, false)
	if updated, err := s.UpdateModel(false); updated || !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("unchecked model refresh accepted", updated, err)
	}
	startSMTModelUpdate(t, s, f)
	x := f.variable("update-disabled-x")
	astTestMust(t, s.Assert(f.app(smtArithOpLE, x, f.number("5"))))
	astTestMust(t, s.SetProduceModels(false))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	checks := f.s.statistics.NumFinalChecks
	if updated, err := s.UpdateModel(true); updated || !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("model-disabled refresh accepted", updated, err)
	}
	if f.s.statistics.NumFinalChecks != checks {
		t.Fatal("invalid model refresh ran final checking")
	}
	astTestMust(t, s.SetProduceModels(true))
	astTestMust(t, s.Assert(f.pin(f.m.MkFalse())))
	status, err = s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal(status)
	}
	if updated, err := s.UpdateModel(false); updated || !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("UNSAT model refresh accepted", updated, err)
	}
}

func TestSMTUpdateModelNativePublishedValues(t *testing.T) {
	checkNativeArithmeticObjective(t, "arithmetic_model_refresh.smt2", `sat
((x 4.0) ((+ (* 2 x) 1) 9.0))
sat
((x 3) ((+ (* 2 x) 1) 7))`)
}
