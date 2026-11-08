package smtcore

import (
	"github.com/glycerine/g3/pkg/util/statistics"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTTacticParameterSnapshotsLogicAndLiveProgress(t *testing.T) {
	m, evaluator := smtModelGeneratorFixture(t)
	c := evaluator.Context
	settings, err := c.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, c.Del(&settings)) })
	key := "max_conflicts"
	astTestMust(t, c.SetUIntName(&settings, &key, 0))
	created, err := NewSMTTactic(m, &settings, SMTSolverConfig{Params: c, Globals: evaluator.Globals, ModelEvaluator: evaluator})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, created.Close()) })
	// The native constructor stores params_ref but leaves fparams at its
	// defaults. using_params subsequently calls updt_params explicitly.
	if created.fullParams.state.MaxConflicts != ^uint32(0) {
		t.Fatal("constructor applied a full fparams update")
	}
	astTestMust(t, created.UpdateParams(&settings))
	if created.fullParams.state.MaxConflicts != 0 {
		t.Fatal("full parameter update was ignored")
	}
	empty, err := c.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, c.Del(&empty)) })
	astTestMust(t, created.UpdateParams(&empty))
	old, err := c.GetUIntName(&created.params, &key, 42)
	astTestMust(t, err)
	if old != 0 || created.fullParams.state.MaxConflicts != ^uint32(0) {
		t.Fatal("parameter snapshot and fparams were incorrectly conflated")
	}
	logic := astTestSymbol(t, m, "QF_UF")
	astTestMust(t, created.SetLogic(logic))
	goal, err := tactic.NewGoal(m, false, false, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	a := smtRelevancyBool(t, m, "smt-tactic-progress-a")
	b := smtRelevancyBool(t, m, "smt-tactic-progress-b")
	clause, err := m.MkOr(a, b)
	astTestMust(t, err)
	astTestMust(t, goal.Assert(clause))
	var result tactic.TacticResult
	t.Cleanup(func() { astTestMust(t, result.Close()) })
	samples := 0
	astTestMust(t, created.SetProgressCallback(func(kind uint32) {
		if kind != 0 {
			return
		}
		samples++
		if created.active == nil || created.active.kernel.setupLogic != "QF_UF" {
			t.Fatal("SetLogic was not applied to the new SMT context")
		}
		if created.active.kernel.smtState.MaxConflicts != ^uint32(0) {
			t.Fatal("context rebuilt fparams from stale params_ref keys")
		}
		if stats := statistics.Sums(created.CollectStatistics); stats["num checks"] != 0 || stats["mk bool var"] < 2 {
			t.Fatal("progress callback cannot inspect the running context's statistics")
		}
	}))
	astTestMust(t, created.Apply(goal, &result))
	if samples == 0 || !goal.IsDecidedSat() {
		t.Fatal("fresh context did not deliver progress and SAT", samples)
	}
	astTestMust(t, created.SetProgressCallback(nil))
	descrs, err := uparams.NewParamDescrs(m.Symbols, uparams.ParamDescrsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, descrs.Close()) })
	astTestMust(t, created.CollectParamDescrs(descrs))
	for _, name := range []string{"candidate_models", "fail_if_inconclusive", "max_conflicts", "rep_freq", "simplex_strategy"} {
		found, err := descrs.ContainsName(name)
		astTestMust(t, err)
		if !found {
			t.Fatal("missing native SMT/LP parameter description", name)
		}
	}
}

func TestSMTTacticTranslationSurvivesSourceClosure(t *testing.T) {
	m, evaluator := smtModelGeneratorFixture(t)
	c := evaluator.Context
	settings, err := c.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, c.Del(&settings)) })
	key := "max_conflicts"
	astTestMust(t, c.SetUIntName(&settings, &key, 0))
	source, err := NewSMTTactic(m, &settings, SMTSolverConfig{Params: c})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, source.Close()) })
	astTestMust(t, source.UpdateParams(&settings))
	astTestMust(t, source.SetLogic(astTestSymbol(t, m, "QF_UF")))
	astTestMust(t, source.SetProgressCallback(func(uint32) { t.Fatal("translation copied the source progress callback") }))
	target, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: m.Symbols, Numerals: m.NumeralService(), Budget: m.Budget()})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, target.Close()) })
	translated, err := source.Translate(target)
	astTestMust(t, err)
	astTestMust(t, translated.IncRef())
	t.Cleanup(func() { astTestMust(t, translated.DecRef()) })
	child := translated.(*SMTTactic)
	if child.logic != (usymbol.Symbol{}) || child.fullParams.state.MaxConflicts != ^uint32(0) {
		t.Fatal("translation did not use the source constructor's reset semantics")
	}
	astTestMust(t, c.SetUIntName(&settings, &key, 42))
	astTestMust(t, source.UpdateParams(&settings))
	astTestMust(t, source.Close())
	stored, err := c.GetUIntName(&child.params, &key, 43)
	astTestMust(t, err)
	if stored != 0 {
		t.Fatal("source update changed the translated parameter snapshot", stored)
	}
	goal, err := tactic.NewGoal(target, false, true, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	astTestMust(t, goal.Assert(smtRelevancyBool(t, target, "smt-tactic-translation-a")))
	var result tactic.TacticResult
	t.Cleanup(func() { astTestMust(t, result.Close()) })
	astTestMust(t, translated.Apply(goal, &result))
	if !goal.IsDecidedSat() || result.ModelConverter == nil {
		t.Fatal("translated tactic borrowed the destroyed source's services")
	}
}

func TestSMTTacticThroughTacticSolverScopesAndModel(t *testing.T) {
	m, evaluator := smtModelGeneratorFixture(t)
	child, err := NewSMTTacticUsing(m, nil, "QF_UF", false, SMTSolverConfig{Params: evaluator.Context, Globals: evaluator.Globals, ModelEvaluator: evaluator})
	astTestMust(t, err)
	backend, err := solver.NewTacticSolver(m, child, nil, false, true, false, "QF_UF", solver.TacticSolverConfig{Params: evaluator.Context})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, backend.Close()) })
	a := smtRelevancyBool(t, m, "smt-tactic-solver-a")
	astTestMust(t, backend.Assert(a))
	status, err := backend.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("tactic solver did not use the SMT backend", status)
	}
	model, err := backend.AcquireModel()
	astTestMust(t, err)
	if model == nil {
		t.Fatal("SMT tactic model did not reach the solver result")
	}
	t.Cleanup(func() { astTestMust(t, model.DecRef()) })
	decl, err := m.GetDecl(a)
	astTestMust(t, err)
	value, err := model.ConstantInterpretation(decl)
	astTestMust(t, err)
	if !m.IsTrue(value) {
		t.Fatal("tactic solver returned the wrong interpretation")
	}
	astTestMust(t, backend.Push())
	notA, err := m.MkNot(a)
	astTestMust(t, err)
	astTestMust(t, backend.Assert(notA))
	status, err = backend.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("scoped tactic solver did not refute a contradiction", status)
	}
	astTestMust(t, backend.Pop(1))
	status, err = backend.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("tactic solver did not recover after pop", status)
	}
	value, err = model.ConstantInterpretation(decl)
	astTestMust(t, err)
	if !m.IsTrue(value) {
		t.Fatal("later solver checks invalidated the previously acquired model")
	}
}

func TestSMTTacticPreservesDerivedInputProof(t *testing.T) {
	m, evaluator := smtModelGeneratorFixture(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	created, err := NewSMTTactic(m, nil, SMTSolverConfig{Params: evaluator.Context, Globals: evaluator.Globals, ModelEvaluator: evaluator})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, created.Close()) })
	a := smtRelevancyBool(t, m, "smt-tactic-proof-a")
	notA, err := m.MkNot(a)
	astTestMust(t, err)
	pa, err := m.MkProofAsserted(a)
	astTestMust(t, err)
	pn, err := m.MkProofAsserted(notA)
	astTestMust(t, err)
	derived, err := m.MkProofUnitResolution([]ast.ASTHandle{pa, pn})
	astTestMust(t, err)
	astTestPin(t, m, derived)
	falsehood, err := m.MkFalse()
	astTestMust(t, err)
	goal, err := tactic.NewGoal(m, true, false, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	astTestMust(t, goal.AssertWithProof(falsehood, derived, ast.ASTDependency{}))
	var result tactic.TacticResult
	t.Cleanup(func() { astTestMust(t, result.Close()) })
	astTestMust(t, created.Apply(goal, &result))
	proof, err := goal.Proof(0)
	astTestMust(t, err)
	if !goal.IsDecidedUnsat() || proof != derived {
		t.Fatal("SMT tactic discarded its input derivation", proof, derived)
	}
	if result.ProofConverter != nil || result.Core != (ast.ASTDependency{}) {
		t.Fatal("SMT tactic published solver2tactic outputs instead of goal-local proof/core")
	}
	if created.active != nil {
		t.Fatal("SMT context survived its application")
	}
}

func TestSMTTacticProofCoreGuardAndGoalDependency(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(map[bool]string{false: "core", true: "proof-and-core"}[proofs], func(t *testing.T) {
			m, evaluator := smtModelGeneratorFixture(t)
			if proofs {
				astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			}
			created, err := NewSMTTactic(m, nil, SMTSolverConfig{Params: evaluator.Context, Globals: evaluator.Globals, ModelEvaluator: evaluator})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, created.Close()) })
			goal, err := tactic.NewGoal(m, proofs, false, true)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, goal.Close()) })
			a := smtRelevancyBool(t, m, "smt-tactic-core-a")
			b := smtRelevancyBool(t, m, "smt-tactic-core-b")
			for i, formula := range []ast.ASTHandle{a, b} {
				if i == 1 {
					formula, err = m.MkNot(a)
					astTestMust(t, err)
				}
				var proof ast.ASTHandle
				if proofs {
					proof, err = m.MkProofAsserted(formula)
					astTestMust(t, err)
				}
				dep, e := m.DependencyLeaf([]ast.ASTHandle{a, b}[i])
				astTestMust(t, e)
				astTestMust(t, goal.AssertWithProof(formula, proof, dep))
			}
			var result tactic.TacticResult
			t.Cleanup(func() { astTestMust(t, result.Close()) })
			err = created.Apply(goal, &result)
			if proofs {
				if err == nil || err.Error() != "smt tactic does not support simultaneous generation of proofs and unsat cores" {
					t.Fatal("missing native proof/core refusal", err)
				}
				if len(result.Goals) != 0 || goal.IsDecidedUnsat() {
					t.Fatal("proof/core refusal changed the goal")
				}
				// Both flags are allowed when no dependency assumptions were
				// extracted; a rejected application must leave the tactic reusable.
				independent, e := tactic.NewGoal(m, true, false, true)
				astTestMust(t, e)
				t.Cleanup(func() { astTestMust(t, independent.Close()) })
				falsehood, e := m.MkFalse()
				astTestMust(t, e)
				proof, e := m.MkProofAsserted(falsehood)
				astTestMust(t, e)
				astTestMust(t, independent.AssertWithProof(falsehood, proof, ast.ASTDependency{}))
				astTestMust(t, created.Apply(independent, &result))
				published, e := independent.Proof(0)
				astTestMust(t, e)
				if !independent.IsDecidedUnsat() || published.IsNull() {
					t.Fatal("proof/core guard rejected a goal without dependency assumptions")
				}
				return
			}
			astTestMust(t, err)
			if !goal.IsDecidedUnsat() {
				t.Fatal("dependent contradiction was not refuted")
			}
			dep, err := goal.Dependency(0)
			astTestMust(t, err)
			leaves, err := m.DependencyLinearize(dep, nil)
			astTestMust(t, err)
			if len(leaves) != 2 || !(leaves[0] == a && leaves[1] == b || leaves[0] == b && leaves[1] == a) {
				t.Fatal("SMT tactic lost the original core labels", leaves)
			}
			if result.Core != (ast.ASTDependency{}) || result.ProofConverter != nil {
				t.Fatal("unexpected separate proof/core outputs")
			}
		})
	}
}

func TestSMTTacticSnapshotsRelevantModelLabels(t *testing.T) {
	m, evaluator := smtModelGeneratorFixture(t)
	created, err := NewSMTTactic(m, nil, SMTSolverConfig{Params: evaluator.Context, Globals: evaluator.Globals, ModelEvaluator: evaluator})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, created.Close()) })
	first := astTestSymbol(t, m, "smt-tactic-label-first")
	second := astTestSymbol(t, m, "smt-tactic-label-second")
	literal, err := m.MkLabelLit([]usymbol.Symbol{first, second})
	astTestMust(t, err)
	goal, err := tactic.NewGoal(m, false, true, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	astTestMust(t, goal.Assert(literal))
	var result tactic.TacticResult
	t.Cleanup(func() { astTestMust(t, result.Close()) })
	astTestMust(t, created.Apply(goal, &result))
	astTestMust(t, created.Close())
	var labels tactic.ModelLabels
	defer labels.Close()
	astTestMust(t, result.ModelConverter.ApplyLabels(&labels, 0))
	if len(labels.Values) != 2 || labels.Values[0] != first || labels.Values[1] != second {
		t.Fatal("model converter lost its relevant-label snapshot", labels.Values)
	}
}

func TestSMTTacticKeepsStatisticsOnUnknownAndApproximationFailure(t *testing.T) {
	m, evaluator := smtModelGeneratorFixture(t)
	settings, err := evaluator.Context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, evaluator.Context.Del(&settings)) })
	key := "max_conflicts"
	astTestMust(t, evaluator.Context.SetUIntName(&settings, &key, 0))
	created, err := NewSMTTactic(m, &settings, SMTSolverConfig{Params: evaluator.Context, Globals: evaluator.Globals, ModelEvaluator: evaluator})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, created.Close()) })
	astTestMust(t, created.UpdateParams(&settings))
	goal, err := tactic.NewGoal(m, false, false, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	a := smtRelevancyBool(t, m, "smt-tactic-stat-a")
	astTestPin(t, m, a)
	b := smtRelevancyBool(t, m, "smt-tactic-stat-b")
	notA, err := m.MkNot(a)
	astTestMust(t, err)
	notB, err := m.MkNot(b)
	astTestMust(t, err)
	for _, pair := range [][2]ast.ASTHandle{{a, b}, {notA, b}, {a, notB}, {notA, notB}} {
		clause, err := m.MkOr(pair[0], pair[1])
		astTestMust(t, err)
		astTestMust(t, goal.Assert(clause))
	}
	var result tactic.TacticResult
	t.Cleanup(func() { astTestMust(t, result.Close()) })
	err = created.Apply(goal, &result)
	if err == nil || !strings.Contains(err.Error(), "max-conflicts-reached") {
		t.Fatal("expected conflict-limit failure", err)
	}
	if stats := statistics.Sums(created.CollectStatistics); stats["num checks"] != 0 || stats["mk bool var"] != 3 || stats["conflicts"] == 0 {
		t.Fatal("UNKNOWN lost check statistics", stats)
	}
	astTestMust(t, created.ResetStatistics())
	astTestMust(t, evaluator.Context.SetUIntName(&settings, &key, ^uint32(0)))
	astTestMust(t, created.UpdateParams(&settings))
	astTestMust(t, goal.Reset())
	astTestMust(t, goal.Assert(a))
	astTestMust(t, goal.SetPrecision(tactic.GoalOver))
	err = created.Apply(goal, &result)
	if err == nil || err.Error() != "over-approximated goal found to be sat" {
		t.Fatal("expected approximation failure", err)
	}
	if stats := statistics.Sums(created.CollectStatistics); stats["num checks"] != 0 || stats["mk bool var"] != 2 {
		t.Fatal("approximation exception lost check statistics", stats)
	}
	astTestMust(t, created.Cleanup())
	if stats := statistics.Sums(created.CollectStatistics); stats["num checks"] != 0 || stats["mk bool var"] != 2 {
		t.Fatal("cleanup erased stored statistics")
	}
	astTestMust(t, created.ResetStatistics())
	if len(statistics.Sums(created.CollectStatistics)) != 0 {
		t.Fatal("statistics did not reset")
	}
}

func TestSMTTacticPublishesIndependentModelsAcrossApplications(t *testing.T) {
	m, evaluator := smtModelGeneratorFixture(t)
	created, err := NewSMTTacticUsing(m, nil, "QF_UF", false, SMTSolverConfig{Params: evaluator.Context, Globals: evaluator.Globals, ModelEvaluator: evaluator})
	astTestMust(t, err)
	astTestMust(t, created.IncRef())
	t.Cleanup(func() { astTestMust(t, created.DecRef()) })
	atom := smtRelevancyBool(t, m, "smt-tactic-model-atom")
	astTestPin(t, m, atom)
	declaration, err := m.GetDecl(atom)
	astTestMust(t, err)
	var saved tactic.ModelRef
	t.Cleanup(func() { astTestMust(t, saved.Close()) })
	for _, positive := range []bool{true, false} {
		goal, err := tactic.NewGoal(m, false, true, false)
		astTestMust(t, err)
		formula := atom
		if !positive {
			formula, err = m.MkNot(atom)
			astTestMust(t, err)
		}
		astTestMust(t, goal.Assert(formula))
		var result tactic.TacticResult
		func() {
			defer func() { astTestMust(t, result.Close()); astTestMust(t, goal.Close()) }()
			astTestMust(t, created.Apply(goal, &result))
			if !goal.IsDecidedSat() || result.ModelConverter == nil {
				t.Fatal("SMT tactic did not publish a model for its satisfiable goal", positive)
			}
			var current tactic.ModelRef
			defer func() { astTestMust(t, current.Close()) }()
			astTestMust(t, result.ModelConverter.ApplyUnindexed(&current))
			model, err := current.Get()
			astTestMust(t, err)
			if model == nil {
				t.Fatal("SMT tactic converter returned an empty model")
			}
			value, err := model.ConstantInterpretation(declaration)
			astTestMust(t, err)
			if positive && !m.IsTrue(value) || !positive && !m.IsFalse(value) {
				t.Fatal("SMT tactic model lost the asserted Boolean value", positive, value)
			}
			if positive {
				astTestMust(t, saved.SetRef(&current))
			}
		}()
	}
	previous, err := saved.Get()
	astTestMust(t, err)
	value, err := previous.ConstantInterpretation(declaration)
	astTestMust(t, err)
	if !m.IsTrue(value) {
		t.Fatal("later tactic application changed the previously published model")
	}
	// The original term remains independently owned after both local SMT
	// contexts and their result converters have been destroyed.
	if sort, err := m.GetSort(atom); err != nil || sort == (ast.ASTHandle{}) {
		t.Fatal("tactic teardown invalidated its caller's AST", err)
	}
}

func TestNewSMTTacticUsingCopiesAndFixesAutoConfig(t *testing.T) {
	m := astTestManager(t)
	params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, params.Close()) })
	input, err := params.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, params.Del(&input)) })
	created, err := NewSMTTacticUsing(m, &input, "QF_UF", true, SMTSolverConfig{Params: params})
	astTestMust(t, err)
	astTestMust(t, created.IncRef())
	t.Cleanup(func() { astTestMust(t, created.DecRef()) })
	autoName := "auto_config"
	present, err := params.ContainsName(&input, &autoName)
	astTestMust(t, err)
	if present {
		t.Fatal("factory mutated caller parameters")
	}
	goal, err := tactic.NewGoal(m, false, true, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	var result tactic.TacticResult
	astTestMust(t, created.Apply(goal, &result))
	t.Cleanup(func() { astTestMust(t, result.Close()) })
	if len(result.Goals) != 1 || !goal.IsDecidedSat() {
		t.Fatal("SMT tactic factory result")
	}
	if result.ModelConverter == nil {
		t.Fatal("SMT tactic with default evaluator services did not publish a model")
	}
}
