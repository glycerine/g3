package smtcore

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTModelCheckerRetainsBindingsAcrossBacktrackingUntilRestart(t *testing.T) {
	for _, quantifierGeneration := range []uint32{4, 20} {
		t.Run(fmt.Sprint(quantifierGeneration), func(t *testing.T) {
			m, state, registry, _ := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{})
			q, _, sort := smtManagedQuantifier(t, m, "pinned-instance")
			astTestMust(t, state.SetGeneration(quantifierGeneration))
			astTestMust(t, state.Internalize(q, true))
			checker, err := NewSMTModelChecker(state, 1)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, checker.Close()) })
			astTestMust(t, state.PushScope())
			term, err := m.MkConst(astTestSymbol(t, m, "pinned-instance-binding"), sort)
			astTestMust(t, err)
			astTestMust(t, state.SetGeneration(9))
			astTestMust(t, state.Internalize(term, false))
			oldNode := state.context.enodes.get(term)
			for i := 0; i < 2; i++ {
				added, err := checker.enqueueCounterexample(q, []ast.ASTHandle{term}, true, 11)
				astTestMust(t, err)
				if !added {
					t.Fatal("source capture must retain duplicate candidates until queue insertion")
				}
			}
			if len(checker.newInstances) != 2 || registry.schedule.HasWork() || registry.numInstances != 0 {
				t.Fatal("candidate capture prematurely entered the quantifier queue")
			}
			astTestMust(t, state.PopScopes(1))
			if state.context.enodes.get(term) != nil {
				t.Fatal("decision enode survived backtracking")
			}
			if _, err := m.GetKind(term); err != nil {
				t.Fatal("pinned binding expression did not survive its enode", err)
			}
			astTestMust(t, state.SetGeneration(77))
			astTestMust(t, checker.Restart())
			wantGeneration := uint32(11)
			if quantifierGeneration > wantGeneration {
				wantGeneration = quantifierGeneration
			}
			node := state.context.enodes.get(term)
			if node == nil || node == oldNode || node.Generation() != wantGeneration || state.generation != 77 {
				t.Fatal("restart did not reinternalize the pinned binding with source generation", node, state.generation)
			}
			if checker.HasNewInstances() || registry.scopeLevel != 0 || len(registry.schedule.newEntries) != 1 || registry.schedule.newEntries[0].Generation != wantGeneration {
				t.Fatal("restart did not publish once at the base scope", checker.HasNewInstances(), registry.scopeLevel, registry.schedule.newEntries)
			}
		})
	}
}

func TestSMTModelCheckerRestartSkipsQuantifiersRemovedByPop(t *testing.T) {
	m, state, registry, _ := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{})
	checker, err := NewSMTModelChecker(state, 1)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, checker.Close()) })
	astTestMust(t, state.PushScope())
	q, _, sort := smtManagedQuantifier(t, m, "popped-instance")
	astTestMust(t, state.Internalize(q, true))
	term, err := m.MkConst(astTestSymbol(t, m, "popped-instance-binding"), sort)
	astTestMust(t, err)
	added, err := checker.addCounterexample(q, []ast.ASTHandle{term})
	astTestMust(t, err)
	if !added || state.context.enodes.get(term) != nil || registry.schedule.HasWork() {
		t.Fatal("capture must not internalize a new binding")
	}
	astTestMust(t, state.PopScopes(1))
	if _, present := state.BoolVar(q); present {
		t.Fatal("fixture quantifier was not removed")
	}
	astTestMust(t, checker.Restart())
	if checker.HasNewInstances() || registry.schedule.HasWork() || state.context.enodes.get(term) != nil {
		t.Fatal("restart revived an obsolete quantifier or binding")
	}
}

func TestSMTModelCheckerCaptureUsesInverseGenerationRatherThanEnodeGeneration(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	s, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	state := s.kernel.internalizer
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	x, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{astTestSymbol(t, m, "inverse-generation-b")}, Body: x})
	astTestMust(t, err)
	astTestMust(t, state.Internalize(q, true))
	falsity, err := m.MkFalse()
	astTestMust(t, err)
	astTestMust(t, state.SetGeneration(1))
	astTestMust(t, state.Internalize(falsity, false))
	finder := state.quantifierManager.modelFinder
	_, err = finder.InsertInstantiationTerm(q, 0, falsity, 13)
	astTestMust(t, err)
	set, err := finder.instantiationSet(q, 0, false)
	astTestMust(t, err)
	proto, err := protomodel.NewProtoModel(m, cfg)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, proto.Close()) })
	astTestMust(t, set.MakeInverse(proto))
	checker := state.quantifierManager.modelChecker
	added, err := checker.addFiniteCounterexample(q, []ast.ASTHandle{falsity})
	astTestMust(t, err)
	if !added || len(checker.newInstances) != 1 || checker.newInstances[0].generation != 13 || state.context.enodes.get(falsity).Generation() == 13 {
		t.Fatal("capture substituted enode generation for inverse generation", added, checker.newInstances)
	}
	astTestMust(t, checker.Restart())
	if len(state.quantifiers.schedule.newEntries) != 1 || state.quantifiers.schedule.newEntries[0].Generation != 13 {
		t.Fatal("inverse generation was lost at restart publication")
	}
}

func TestSMTModelCheckerPinnedInstanceAllocationFrontiers(t *testing.T) {
	for _, phase := range []string{"capture", "restart"} {
		t.Run(phase, func(t *testing.T) {
			succeeded := false
			for stop := int64(0); stop < 512 && !succeeded; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					m, state, registry, _ := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{})
					q, _, sort := smtManagedQuantifier(t, m, "stopped-instance")
					astTestMust(t, state.Internalize(q, true))
					term, err := m.MkConst(astTestSymbol(t, m, "stopped-instance-binding"), sort)
					astTestMust(t, err)
					astTestPin(t, m, term)
					checker, err := NewSMTModelChecker(state, 1)
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, checker.Close()) })
					if phase == "restart" {
						_, err = checker.addCounterexample(q, []ast.ASTHandle{term})
						astTestMust(t, err)
					}
					m.Budget().FailAfter(stop)
					if phase == "capture" {
						_, err = checker.addCounterexample(q, []ast.ASTHandle{term})
					} else {
						err = checker.Restart()
					}
					m.Budget().FailAfter(-1)
					if err != nil {
						if !errors.Is(err, arena.ErrInjectedFailure) {
							t.Fatal("unexpected stopped-owner error", err)
						}
						if phase == "capture" && (checker.HasNewInstances() || registry.schedule.HasWork()) {
							t.Fatal("failed capture published a partial instance")
						}
					} else {
						succeeded = true
						if phase == "restart" && (checker.HasNewInstances() || !registry.schedule.HasWork()) {
							t.Fatal("successful restart did not transfer ownership")
						}
					}
				})
				if t.Failed() {
					t.Fatalf("allocation frontier %s/%d", phase, stop)
				}
			}
			if !succeeded {
				t.Fatal("no successful pinned-instance allocation frontier")
			}
		})
	}
}

func TestSMTMBQICounterexamplesPublishAfterForcedBaseRestart(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	params, err := cfg.Context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
	for _, key := range []string{"auto_config", "preprocess", "ematching"} {
		astTestMust(t, cfg.Context.SetBoolName(&params, &key, false))
	}
	s, err := NewSMTSolver(m, &params, "AUFLIA", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	x, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	p := smtQuickPredicate(t, m, "restart-mbqi-p", boolean, boolean)
	body, err := m.MkApp(p, []ast.ASTHandle{x})
	astTestMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{astTestSymbol(t, m, "restart-mbqi-x")}, Body: body})
	astTestMust(t, err)
	astTestMust(t, s.Assert(q))
	a := smtRelevancyBool(t, m, "restart-mbqi-a")
	b := smtRelevancyBool(t, m, "restart-mbqi-b")
	clause, err := m.MkOr(a, b)
	astTestMust(t, err)
	astTestMust(t, s.Assert(clause))
	k := s.kernel
	manager := k.internalizer.quantifierManager
	build := manager.candidateModel
	candidates := 0
	manager.candidateModel = func() (*SMTModelGenerator, error) {
		candidates++
		generator, err := build()
		if err != nil {
			return nil, err
		}
		if candidates == 1 {
			if k.internalizer.scopeLevel == 0 {
				t.Fatal("fixture did not reach a non-base candidate")
			}
			falsehood, err := m.MkFalse()
			if err == nil {
				var interpretation *model.FunctionInterpretation
				interpretation, err = model.NewFunctionInterpretation(m, 1)
				if err == nil {
					err = interpretation.SetElse(falsehood)
				}
				if err == nil {
					err = generator.model.Core().RegisterFunction(p, interpretation)
				} else if interpretation != nil {
					_ = interpretation.Close()
				}
			}
			if err != nil {
				return nil, errors.Join(err, generator.Close())
			}
		} else if k.internalizer.Statistics().NumRestarts == 0 || manager.modelChecker.HasNewInstances() {
			t.Fatal("counterexample was checked again before base restart publication")
		}
		return generator, nil
	}
	// Source model-check restarts are unconditional even when adaptive restarts
	// would normally skip backtracking because search agility is high.
	k.smtState.RestartAdaptive = true
	k.smtState.RestartAgilityThreshold = 0
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True || candidates < 2 || k.internalizer.Statistics().NumRestarts == 0 {
		t.Fatal("MBQI did not refine the candidate through a restart", status, candidates, k.Statistics())
	}
	if manager.modelChecker.HasNewInstances() || manager.acceptedModel != nil {
		t.Fatal("completed search retained temporary model-check owners")
	}
}
