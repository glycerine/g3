package smtcore

import (
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func smtSolverFixture(t *testing.T) (*ast.ASTManager, *SMTSolver) {
	t.Helper()
	m := astTestManager(t)
	params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	s, err := NewSMTSolver(m, nil, "QF_UF", SMTSolverConfig{Params: params})
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, s.Close())
		astTestMust(t, params.Close())
	})
	return m, s
}

func TestSMTSolverInstallsQuantifierManagerAndEMatches(t *testing.T) {
	m, s := smtSolverFixture(t)
	if s.kernel.internalizer.quantifiers == nil || s.kernel.internalizer.quantifierManager == nil || s.kernel.quantifierCosts == nil {
		t.Fatal("solver setup omitted quantifier components")
	}
	sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "solver-quantifier-sort"), nil)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	predicate, err := m.MkFuncDecl(astTestSymbol(t, m, "solver-quantifier-p"), []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	variable, err := m.MkVar(0, sort)
	astTestMust(t, err)
	body, err := m.MkApp(predicate, []ast.ASTHandle{variable})
	astTestMust(t, err)
	pattern, err := m.MkPattern([]ast.ASTHandle{body})
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{astTestSymbol(t, m, "solver-quantifier-x")}, Body: body, Patterns: []ast.ASTHandle{pattern},
	})
	astTestMust(t, err)
	constant, err := m.MkConst(astTestSymbol(t, m, "solver-quantifier-a"), sort)
	astTestMust(t, err)
	application, err := m.MkApp(predicate, []ast.ASTHandle{constant})
	astTestMust(t, err)
	negated, err := m.MkNot(application)
	astTestMust(t, err)
	astTestMust(t, s.Assert(quantifier))
	astTestMust(t, s.Assert(negated))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("quantifier instance was not produced", status)
	}
}

func TestSMTSolverMBQIFindsPatternlessBooleanCounterexample(t *testing.T) {
	m, s := smtSolverFixture(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	variable, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{astTestSymbol(t, m, "solver-mbqi-x")}, Body: variable,
	})
	astTestMust(t, err)
	astTestMust(t, s.Assert(quantifier))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("model-based counterexample was not instantiated", status)
	}
}

func TestSMTSolverMBQIFindsInfiniteIntegerCounterexample(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	integer, err := m.GetSort(zero)
	astTestMust(t, err)
	variable, err := m.MkVar(0, integer)
	astTestMust(t, err)
	body, err := m.MkEq(variable, zero)
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []usymbol.Symbol{astTestSymbol(t, m, "solver-mbqi-int")}, Body: body,
	})
	astTestMust(t, err)
	params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	s, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: params})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()); astTestMust(t, params.Close()) })
	astTestMust(t, s.Assert(quantifier))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		reason, _ := s.ReasonUnknown()
		t.Fatal("infinite integer counterexample was not instantiated", status, reason)
	}
}

func TestSMTSolverMBQIBlocksAndCollectsMultipleIntegerCounterexamples(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	integer, err := m.GetSort(zero)
	astTestMust(t, err)
	variable, err := m.MkVar(0, integer)
	astTestMust(t, err)
	body, err := m.MkEq(variable, zero)
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []usymbol.Symbol{astTestSymbol(t, m, "solver-mbqi-many-int")}, Body: body,
	})
	astTestMust(t, err)
	params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	local, err := params.NewValue()
	astTestMust(t, err)
	name := "mbqi.max_cexs"
	astTestMust(t, params.SetUIntName(&local, &name, 3))
	s, err := NewSMTSolver(m, &local, "AUFLIA", SMTSolverConfig{Params: params})
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, s.Close())
		astTestMust(t, params.Del(&local))
		astTestMust(t, params.Close())
	})
	astTestMust(t, s.Assert(quantifier))
	// Exercise unrestricted auxiliary enumeration. The ordinary AUF path
	// supplies a singleton for this formula and can refute it in one instance;
	// restriction and inverse recovery have separate production-path tests.
	s.kernel.internalizer.quantifierManager.modelChecker.modelFinder = nil
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("multiple integer counterexamples did not refute quantifier", status)
	}
	stats := s.kernel.internalizer.quantifiers.schedule.Stats()
	if stats.Instances != 3 {
		t.Fatal("MBQI blocking did not collect the configured counterexample count", stats)
	}
}

func TestSMTSolverMBQIRestrictsSkolemToBodyInstantiationSet(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	five, err := m.ArithmeticIntegerNumeralText("5")
	astTestMust(t, err)
	integer, err := m.GetSort(zero)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	predicate := smtQuickPredicate(t, m, "solver-mbqi-inst-set-p", integer, boolean)
	variable, err := m.MkVar(0, integer)
	astTestMust(t, err)
	trigger, err := m.MkApp(predicate, []ast.ASTHandle{variable})
	astTestMust(t, err)
	pattern, err := m.MkPattern([]ast.ASTHandle{trigger})
	astTestMust(t, err)
	body, err := m.MkEq(variable, zero)
	astTestMust(t, err)
	body, err = m.MkAnd(trigger, body)
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []usymbol.Symbol{astTestSymbol(t, m, "solver-mbqi-inst-set-x")},
		Body: body, Patterns: []ast.ASTHandle{pattern},
	})
	astTestMust(t, err)
	ground, err := m.MkApp(predicate, []ast.ASTHandle{five})
	astTestMust(t, err)
	local, err := modelConfig.Context.NewValue()
	astTestMust(t, err)
	ematching := "ematching"
	astTestMust(t, modelConfig.Context.SetBoolName(&local, &ematching, false))
	s, err := NewSMTSolver(m, &local, "AUFLIA", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, s.Close())
		astTestMust(t, modelConfig.Context.Del(&local))
	})
	astTestMust(t, s.Assert(ground))
	astTestMust(t, s.Assert(quantifier))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("restricted integer counterexample did not refute quantifier", status)
	}
	fingerprints := s.kernel.internalizer.quantifiers.fingerprints.fingerprints
	if len(fingerprints) != 1 || len(fingerprints[0].args) != 1 || fingerprints[0].args[0].owner != five {
		t.Fatal("MBQI did not use the body instantiation-set inverse", len(fingerprints))
	}
}

func TestSMTSolverMBQIFallsBackWhenInstantiationSetHasNoCounterexample(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	integer, err := m.GetSort(zero)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	predicate := smtQuickPredicate(t, m, "solver-mbqi-fallback-p", integer, boolean)
	variable, err := m.MkVar(0, integer)
	astTestMust(t, err)
	trigger, err := m.MkApp(predicate, []ast.ASTHandle{variable})
	astTestMust(t, err)
	pattern, err := m.MkPattern([]ast.ASTHandle{trigger})
	astTestMust(t, err)
	body, err := m.MkEq(variable, zero)
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []usymbol.Symbol{astTestSymbol(t, m, "solver-mbqi-fallback-x")},
		Body: body, Patterns: []ast.ASTHandle{pattern},
	})
	astTestMust(t, err)
	ground, err := m.MkApp(predicate, []ast.ASTHandle{zero})
	astTestMust(t, err)
	local, err := modelConfig.Context.NewValue()
	astTestMust(t, err)
	ematching := "ematching"
	astTestMust(t, modelConfig.Context.SetBoolName(&local, &ematching, false))
	s, err := NewSMTSolver(m, &local, "AUFLIA", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, s.Close())
		astTestMust(t, modelConfig.Context.Del(&local))
	})
	astTestMust(t, s.Assert(ground))
	astTestMust(t, s.Assert(quantifier))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("complete auxiliary counterexample was not used", status)
	}
	fingerprints := s.kernel.internalizer.quantifiers.fingerprints.fingerprints
	if len(fingerprints) != 1 || len(fingerprints[0].args) != 1 || fingerprints[0].args[0].owner == zero {
		t.Fatal("MBQI did not fall back from the exhausted instantiation set", len(fingerprints))
	}
}

func TestSMTSolverMBQIFindsUninterpretedUniverseCounterexample(t *testing.T) {
	m, s := smtSolverFixture(t)
	sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "solver-mbqi-sort"), nil)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	predicate, err := m.MkFuncDecl(astTestSymbol(t, m, "solver-mbqi-p"), []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	variable, err := m.MkVar(0, sort)
	astTestMust(t, err)
	body, err := m.MkApp(predicate, []ast.ASTHandle{variable})
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{astTestSymbol(t, m, "solver-mbqi-u")}, Body: body,
	})
	astTestMust(t, err)
	constant, err := m.MkConst(astTestSymbol(t, m, "solver-mbqi-a"), sort)
	astTestMust(t, err)
	application, err := m.MkApp(predicate, []ast.ASTHandle{constant})
	astTestMust(t, err)
	negated, err := m.MkNot(application)
	astTestMust(t, err)
	astTestMust(t, s.Assert(quantifier))
	astTestMust(t, s.Assert(negated))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("uninterpreted-universe counterexample was not instantiated", status)
	}
}

func TestSMTSolverKernelFacadeAssertionsScopesAndResults(t *testing.T) {
	m, s := smtSolverFixture(t)
	a := smtRelevancyBool(t, m, "smt-solver-a")
	b := smtRelevancyBool(t, m, "smt-solver-b")

	astTestMust(t, s.Assert(a))
	if count, err := s.NumAssertions(); err != nil || count != 1 {
		t.Fatal("base assertions", count, err)
	}
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("base status", status)
	}
	astTestMust(t, s.Push())
	notA, err := m.MkNot(a)
	astTestMust(t, err)
	astTestMust(t, s.Assert(notA))
	status, err = s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("scoped contradiction", status)
	}
	astTestMust(t, s.Pop(1))
	status, err = s.Status()
	astTestMust(t, err)
	if status != lbool.Undef {
		t.Fatal("pop did not clear result", status)
	}
	astTestMust(t, s.Assert(b))
	status, err = s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("post-pop status", status)
	}
	var out strings.Builder
	astTestMust(t, s.Display(&out, nil))
	if !strings.Contains(out.String(), "assert") {
		t.Fatal("solver display", out.String())
	}
	stats := statistics.New()
	astTestMust(t, s.CollectStatistics(stats))
	if stats.Size() == 0 {
		t.Fatal("missing cumulative kernel statistics")
	}
}

func TestSMTSolverPreprocessesAssertionsWithOwnedParameterServices(t *testing.T) {
	m := astTestManager(t)
	context, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Close()) })
	a := smtRelevancyBool(t, m, "smt-preprocess-a")
	notA, err := m.MkNot(a)
	astTestMust(t, err)
	double, err := m.MkNot(notA)
	astTestMust(t, err)
	astTestPin(t, m, double) // Reused after the first solver rewrites it away.

	s, err := NewSMTSolver(m, nil, "QF_UF", SMTSolverConfig{Params: context})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	astTestMust(t, s.Assert(double))
	assertion, err := s.Assertion(0)
	astTestMust(t, err)
	if assertion != a {
		t.Fatal("assertion was not theory-rewritten", assertion, a)
	}

	local, err := context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Del(&local)) })
	name := "preprocess"
	astTestMust(t, context.SetBoolName(&local, &name, false))
	raw, err := NewSMTSolver(m, &local, "QF_UF", SMTSolverConfig{Params: context})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, raw.Close()) })
	astTestMust(t, raw.Assert(double))
	assertion, err = raw.Assertion(0)
	astTestMust(t, err)
	if assertion != double {
		t.Fatal("disabled preprocessing changed assertion", assertion, double)
	}
	astTestMust(t, context.SetBoolName(&local, &name, true))
	astTestMust(t, raw.UpdateParams(&local))
	astTestMust(t, raw.Assert(double))
	assertion, err = raw.Assertion(1)
	astTestMust(t, err)
	if assertion != a {
		t.Fatal("updated preprocessing did not rewrite assertion", assertion, a)
	}
}

func TestSMTSolverInstallsAndUpdatesRetainedSMTParameters(t *testing.T) {
	m := astTestManager(t)
	context, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	local, err := context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, context.Del(&local))
		astTestMust(t, context.Close())
	})
	setUInt := func(name string, value uint32) {
		astTestMust(t, context.SetUIntName(&local, &name, value))
	}
	setBool := func(name string, value bool) {
		astTestMust(t, context.SetBoolName(&local, &name, value))
	}
	setDouble := func(name string, value float64) {
		astTestMust(t, context.SetDoubleName(&local, &name, value))
	}
	setString := func(name, value string) {
		astTestMust(t, context.SetStringName(&local, &name, &value))
	}
	setUInt("dack", 2)
	setBool("dack.eq", true)
	setDouble("dack.factor", 0.25)
	setUInt("dack.threshold", 7)
	setUInt("dack.gc", 23)
	setDouble("dack.gc_inv_decay", 0.75)
	setUInt("relevancy", 0)
	setBool("mbqi", true)
	setBool("mbqi.trace", true)
	setString("mbqi.id", "before.")
	setUInt("mbqi.max_cexs", 2)
	setUInt("mbqi.max_iterations", 3)

	solver, err := NewSMTSolver(m, &local, "QF_UF", SMTSolverConfig{Params: context})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	internalizer := solver.kernel.internalizer
	manager := internalizer.dynamicAck
	if !internalizer.binaryClauseOptimization || solver.kernel.relevancy.enabled || manager.strategy != dynAckConflictResolution || !manager.equalities ||
		manager.pairs.threshold != 7 || manager.triples.threshold != 7 || manager.schedule.gcEvery != 23 ||
		manager.schedule.factor != 0.25 || manager.pairs.decay != 0.75 {
		t.Fatal("constructor did not install SMT parameters", internalizer.binaryClauseOptimization, manager.strategy, manager.equalities,
			manager.pairs.threshold, manager.triples.threshold, manager.schedule.gcEvery, manager.schedule.factor, manager.pairs.decay)
	}
	quantifiers := internalizer.quantifierManager
	if quantifiers == nil || !quantifiers.mbqi || quantifiers.mbqiID != "before." || quantifiers.modelChecker == nil ||
		!quantifiers.modelChecker.trace || quantifiers.modelChecker.initialMaxCexs != 2 || quantifiers.modelChecker.maxIterations != 3 {
		t.Fatal("constructor did not install MBQI parameters")
	}
	maintenance := internalizer.clauseMaintenance
	if maintenance != defaultSMTClauseMaintenanceConfig() || internalizer.lemmaGCThreshold != maintenance.Initial {
		t.Fatal("constructor did not install clause-maintenance defaults", maintenance, internalizer.lemmaGCThreshold)
	}

	setUInt("dack", 0)
	setBool("dack.eq", false)
	setDouble("dack.factor", 0.5)
	setUInt("dack.threshold", 11)
	setUInt("dack.gc", 47)
	setDouble("dack.gc_inv_decay", 0.5)
	setUInt("relevancy", 2)
	setBool("mbqi", false)
	setBool("mbqi.trace", false)
	setString("mbqi.id", "after.")
	setUInt("mbqi.max_cexs", 5)
	setUInt("mbqi.max_iterations", 7)
	astTestMust(t, solver.UpdateParams(&local))
	if manager != internalizer.dynamicAck || !solver.kernel.relevancy.enabled || manager.strategy != dynAckDisabled || manager.equalities ||
		manager.pairs.threshold != 11 || manager.triples.threshold != 11 || manager.schedule.gcEvery != 47 ||
		manager.schedule.factor != 0.5 || manager.pairs.decay != 0.5 {
		t.Fatal("parameter update replaced or failed to update dynamic-Ack state")
	}
	if quantifiers != internalizer.quantifierManager || quantifiers.mbqi || quantifiers.mbqiTrace || quantifiers.mbqiID != "after." || quantifiers.modelChecker != nil {
		t.Fatal("parameter update failed to disable or update MBQI state")
	}
	setBool("mbqi", true)
	setBool("mbqi.trace", true)
	astTestMust(t, solver.UpdateParams(&local))
	if !quantifiers.mbqi || quantifiers.modelChecker == nil || !quantifiers.modelChecker.trace || quantifiers.modelChecker.initialMaxCexs != 5 || quantifiers.modelChecker.maxIterations != 7 || quantifiers.modelChecker.auxiliary == nil || quantifiers.modelChecker.modelFinder != quantifiers.modelFinder {
		t.Fatal("parameter update failed to recreate MBQI checker")
	}
	checker := quantifiers.modelChecker
	setBool("mbqi.trace", false)
	astTestMust(t, solver.UpdateParams(&local))
	if quantifiers.modelChecker != checker || checker.trace {
		t.Fatal("live trace update replaced checker or kept stale trace setting")
	}
}

func TestSMTSolverDisablesBinaryClauseOptimizationForProofs(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	context, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "QF_UF", SMTSolverConfig{Params: context})
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, solver.Close())
		astTestMust(t, context.Close())
	})
	if solver.kernel.internalizer.binaryClauseOptimization {
		t.Fatal("proof-producing solver enabled compact binary clauses")
	}
}

func TestSMTSolverStopsAtConfiguredConflictLimit(t *testing.T) {
	m := astTestManager(t)
	context, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	local, err := context.NewValue()
	astTestMust(t, err)
	key := "max_conflicts"
	astTestMust(t, context.SetUIntName(&local, &key, 0))
	solver, err := NewSMTSolver(m, &local, "QF_UF", SMTSolverConfig{Params: context})
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, solver.Close())
		astTestMust(t, context.Del(&local))
		astTestMust(t, context.Close())
	})
	a := smtRelevancyBool(t, m, "conflict-limit-a")
	b := smtRelevancyBool(t, m, "conflict-limit-b")
	notA, err := m.MkNot(a)
	astTestMust(t, err)
	notB, err := m.MkNot(b)
	astTestMust(t, err)
	clauses := make([]ast.ASTHandle, 4)
	clauses[0], err = m.MkOr(a, b)
	astTestMust(t, err)
	clauses[1], err = m.MkOr(notA, b)
	astTestMust(t, err)
	clauses[2], err = m.MkOr(a, notB)
	astTestMust(t, err)
	clauses[3], err = m.MkOr(notA, notB)
	astTestMust(t, err)
	formula, err := m.MkAndArgs(clauses)
	astTestMust(t, err)
	astTestMust(t, solver.Assert(formula))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.Undef || solver.kernel.lastFailure != SMTFailureConflictLimit {
		t.Fatal("conflict limit", status, solver.kernel.lastFailure, solver.kernel.Statistics())
	}
	reason, err := solver.ReasonUnknown()
	astTestMust(t, err)
	if reason != "max-conflicts-reached" {
		t.Fatal("conflict-limit reason", reason)
	}
}

func TestSMTSolverPropagatesValuesAcrossPendingAndCommittedAssertions(t *testing.T) {
	m, s := smtSolverFixture(t)
	a := smtRelevancyBool(t, m, "smt-propagate-a")
	b := smtRelevancyBool(t, m, "smt-propagate-b")
	notA, err := m.MkNot(a)
	astTestMust(t, err)
	clause, err := m.MkOr(notA, b)
	astTestMust(t, err)
	astTestMust(t, s.Assert(a))
	astTestMust(t, s.Assert(clause))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("propagated base check", status)
	}
	assertion, err := s.Assertion(1)
	astTestMust(t, err)
	if assertion != b {
		t.Fatal("pending assertion was not value-propagated", assertion, b)
	}

	astTestMust(t, s.Push())
	notB, err := m.MkNot(b)
	astTestMust(t, err)
	astTestMust(t, s.Assert(notB))
	status, err = s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("committed substitution did not reach scoped assertion", status)
	}
	assertion, err = s.Assertion(2)
	astTestMust(t, err)
	if !m.IsFalse(assertion) {
		t.Fatal("scoped assertion was not reduced to false", assertion)
	}
	astTestMust(t, s.Pop(1))
	if count, err := s.NumAssertions(); err != nil || count != 2 {
		t.Fatal("propagated assertion scope rollback", count, err)
	}
}

func TestSMTSolverValuePropagationComposesProofs(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	context, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	s, err := NewSMTSolver(m, nil, "QF_UF", SMTSolverConfig{Params: context})
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, s.Close())
		astTestMust(t, context.Close())
	})
	a := smtRelevancyBool(t, m, "smt-proof-propagate-a")
	b := smtRelevancyBool(t, m, "smt-proof-propagate-b")
	notA, err := m.MkNot(a)
	astTestMust(t, err)
	clause, err := m.MkOr(notA, b)
	astTestMust(t, err)
	astTestMust(t, s.Assert(a))
	astTestMust(t, s.Assert(clause))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("proof propagation status", status)
	}
	assertions := s.kernel.Assertions()
	if len(assertions) != 2 || assertions[1].Formula != b || assertions[1].Proof.IsNull() {
		t.Fatal("proof propagation result", assertions)
	}
	fact, err := m.GetProofFact(assertions[1].Proof)
	astTestMust(t, err)
	if fact != b {
		t.Fatal("propagation proof fact", fact, b)
	}
}

func TestSMTSolverPublishesGeneratedModel(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	s, err := NewSMTSolver(m, nil, "QF_UF", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	astTestMust(t, s.SetProduceModels(true))
	constant := smtRelevancyBool(t, m, "smt-solver-model-constant")
	astTestMust(t, s.Assert(constant))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("model check status", status)
	}
	generated, err := s.AcquireModel()
	astTestMust(t, err)
	if generated == nil {
		t.Fatal("satisfiable solver omitted generated model")
	}
	t.Cleanup(func() { astTestMust(t, generated.DecRef()) })
	declaration, err := m.GetDecl(constant)
	astTestMust(t, err)
	value, err := generated.ConstantInterpretation(declaration)
	astTestMust(t, err)
	if !m.IsTrue(value) {
		t.Fatal("generated Boolean interpretation", value)
	}
}

func TestSMTSolverPublishesIntegerDifferenceModel(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "solver-model-idl-x"), integer)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "solver-model-idl-y"), integer)
	astTestMust(t, err)
	s, err := NewSMTSolver(m, nil, "QF_IDL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	astTestMust(t, s.SetProduceModels(true))
	fixture := smtIntegerDiffFixture{manager: m, family: family, sort: integer, x: x, y: y}
	astTestMust(t, s.Assert(fixture.bound(t, smtArithOpLE, x, y, 2)))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("integer model status", status)
	}
	generated, err := s.AcquireModel()
	astTestMust(t, err)
	if generated == nil {
		t.Fatal("integer solver omitted generated model")
	}
	t.Cleanup(func() { astTestMust(t, generated.DecRef()) })
	for _, expression := range []ast.ASTHandle{x, y} {
		declaration, declarationErr := m.GetDecl(expression)
		astTestMust(t, declarationErr)
		value, valueErr := generated.ConstantInterpretation(declaration)
		astTestMust(t, valueErr)
		if value.IsNull() {
			t.Fatal("missing integer model interpretation", expression)
		}
	}
}

func TestSMTSolverPublishesStrictLinearRealModel(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	zero, err := m.ArithmeticRealNumeralText("0")
	astTestMust(t, err)
	one, err := m.ArithmeticRealNumeralText("1")
	astTestMust(t, err)
	realSort, err := m.GetSort(zero)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "solver-model-lra-x"), realSort)
	astTestMust(t, err)
	s, err := NewSMTSolver(m, nil, "QF_LRA", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	astTestMust(t, s.SetProduceModels(true))
	for _, bound := range []struct {
		op          int32
		left, right ast.ASTHandle
	}{
		{smtArithOpLT, zero, x},
		{smtArithOpLT, x, one},
	} {
		comparison, comparisonErr := m.MkFamilyApp(family, bound.op, nil, []ast.ASTHandle{bound.left, bound.right})
		astTestMust(t, comparisonErr)
		astTestMust(t, s.Assert(comparison))
	}
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("strict linear model status", status)
	}
	generated, err := s.AcquireModel()
	astTestMust(t, err)
	if generated == nil {
		t.Fatal("linear arithmetic solver omitted generated model")
	}
	t.Cleanup(func() { astTestMust(t, generated.DecRef()) })
	declaration, err := m.GetDecl(x)
	astTestMust(t, err)
	value, err := generated.ConstantInterpretation(declaration)
	astTestMust(t, err)
	text, _, matched, err := m.ArithmeticNumeralText(value)
	astTestMust(t, err)
	rational, valid := new(big.Rat).SetString(text)
	if !matched || !valid || rational.Sign() <= 0 || rational.Cmp(big.NewRat(1, 1)) >= 0 {
		t.Fatal("strict linear model value", text, matched, valid)
	}
}

func TestSMTSolverDerivesBooleanConsequences(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	s, err := NewSMTSolver(m, nil, "QF_UF", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	constant := smtRelevancyBool(t, m, "smt-solver-consequence")
	astTestMust(t, s.Assert(constant))
	status, consequences, err := s.GetConsequences(nil, []ast.ASTHandle{constant})
	astTestMust(t, err)
	if status != lbool.True || len(consequences) != 1 {
		t.Fatal("Boolean consequences", status, consequences)
	}
	if s.produceModels {
		t.Fatal("consequence query did not restore model setting")
	}
}

func TestSMTSolverDerivesArithmeticConsequences(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	if _, err := m.RegisterArithmeticPlugin(); err != nil {
		t.Fatal(err)
	}
	three, err := m.ArithmeticNumeralRat(big.NewRat(3, 1), true)
	astTestMust(t, err)
	integer, err := m.GetSort(three)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "smt-solver-consequence-x"), integer)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "smt-solver-consequence-y"), integer)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	a, err := m.MkConst(astTestSymbol(t, m, "smt-solver-consequence-a"), boolean)
	astTestMust(t, err)
	equality, err := m.MkEq(x, three)
	astTestMust(t, err)
	conditional, err := m.MkImplies(a, equality)
	astTestMust(t, err)
	four, err := m.ArithmeticNumeralRat(big.NewRat(4, 1), true)
	astTestMust(t, err)
	yEquality, err := m.MkEq(y, four)
	astTestMust(t, err)
	s, err := NewSMTSolver(m, nil, "QF_LIA", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	astTestMust(t, s.Assert(conditional))
	astTestMust(t, s.Assert(yEquality))
	status, consequences, err := s.GetConsequences([]ast.ASTHandle{a}, []ast.ASTHandle{a, x, y})
	astTestMust(t, err)
	if status != lbool.True || len(consequences) != 3 {
		t.Fatal("arithmetic consequences", status, consequences)
	}
	for _, consequence := range consequences {
		if _, err = m.GetSort(consequence); err != nil {
			t.Fatal("arithmetic consequence lifetime", err)
		}
	}
}

func TestSMTSolverFindsBinaryClauseMutexes(t *testing.T) {
	m, s := smtSolverFixture(t)
	a := smtRelevancyBool(t, m, "smt-mutex-a")
	b := smtRelevancyBool(t, m, "smt-mutex-b")
	notA, err := m.MkNot(a)
	astTestMust(t, err)
	notB, err := m.MkNot(b)
	astTestMust(t, err)
	clause, err := m.MkOr(notA, notB)
	astTestMust(t, err)
	astTestMust(t, s.Assert(clause))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("mutex setup status", status)
	}
	status, mutexes, err := s.FindMutexes([]ast.ASTHandle{a, b})
	astTestMust(t, err)
	if status != lbool.True || len(mutexes) != 1 || len(mutexes[0]) != 2 {
		t.Fatal("binary mutexes", status, mutexes)
	}
}

func TestSMTSolverPreferredSATReturnsConflictingPreferenceCores(t *testing.T) {
	m, s := smtSolverFixture(t)
	a := smtRelevancyBool(t, m, "smt-preferred-a")
	b := smtRelevancyBool(t, m, "smt-preferred-b")
	both, err := m.MkAnd(a, b)
	astTestMust(t, err)
	notBoth, err := m.MkNot(both)
	astTestMust(t, err)
	astTestMust(t, s.Assert(notBoth))
	status, cores, err := s.PreferredSAT([]ast.ASTHandle{a, b})
	astTestMust(t, err)
	if status != lbool.True || len(cores) != 1 || len(cores[0]) != 2 {
		t.Fatal("preferred SAT result", status, cores)
	}
	seen := map[ast.ASTHandle]bool{}
	for _, expression := range cores[0] {
		seen[expression] = true
	}
	if !seen[a] || !seen[b] {
		t.Fatal("preferred SAT core", cores)
	}
}

func TestSMTSolverTrackedAssertionsCoreDuplicatesAndRollback(t *testing.T) {
	m, s := smtSolverFixture(t)
	a := smtRelevancyBool(t, m, "smt-solver-tracked-a")
	tag := smtRelevancyBool(t, m, "smt-solver-tracked-tag")
	notA, err := m.MkNot(a)
	astTestMust(t, err)

	astTestMust(t, s.Push())
	astTestMust(t, s.AssertTracked(a, tag))
	if err = s.AssertTracked(a, tag); err == nil {
		t.Fatal("duplicate tracked assertion accepted")
	} else {
		var duplicate z3exception.DefaultException
		if !errors.As(err, &duplicate) {
			t.Fatal("duplicate error", err)
		}
	}
	status, err := s.CheckSat([]ast.ASTHandle{notA})
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("tracked assertion status", status)
	}
	core, err := s.UnsatCore()
	astTestMust(t, err)
	seen := make(map[ast.ASTHandle]bool, len(core))
	for _, expression := range core {
		seen[expression] = true
	}
	if len(core) != 2 || !seen[tag] || !seen[notA] {
		t.Fatal("tracked assertion core", core, tag, notA)
	}
	astTestMust(t, s.Pop(1))
	if count, err := s.NumAssumptions(); err != nil || count != 0 {
		t.Fatal("tracked assumptions after pop", count, err)
	}
	// The source duplicate-name table is scope-sensitive.
	astTestMust(t, s.AssertTracked(a, tag))
}

func TestSMTSolverNamedAssertionReferencesReleaseOnPopAndClose(t *testing.T) {
	m, s := smtSolverFixture(t)
	formula := smtRelevancyBool(t, m, "smt-solver-named-release-formula")
	tag := smtRelevancyBool(t, m, "smt-solver-named-release-tag")
	formulaPin, err := m.NewRef(formula)
	astTestMust(t, err)
	tagPin, err := m.NewRef(tag)
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, m.DelRef(&tagPin))
		astTestMust(t, m.DelRef(&formulaPin))
	})
	refCount := func(value ast.ASTHandle) uint32 {
		t.Helper()
		count, err := m.GetRefCount(value)
		astTestMust(t, err)
		return count
	}
	formulaBefore, tagBefore := refCount(formula), refCount(tag)
	astTestMust(t, s.Push())
	astTestMust(t, s.AssertTracked(formula, tag))
	if refCount(formula) <= formulaBefore || refCount(tag) <= tagBefore {
		t.Fatal("tracked assertion did not own its formula and assumption tag")
	}
	astTestMust(t, s.Pop(1))
	if got := refCount(formula); got != formulaBefore {
		t.Fatal("popped tracked formula reference", got, "want", formulaBefore)
	}
	if got := refCount(tag); got != tagBefore {
		t.Fatal("popped tracked tag reference", got, "want", tagBefore)
	}
	astTestMust(t, s.AssertTracked(formula, tag))
	astTestMust(t, s.Close())
	if got := refCount(formula); got != formulaBefore {
		t.Fatal("closed solver formula reference", got, "want", formulaBefore)
	}
	if got := refCount(tag); got != tagBefore {
		t.Fatal("closed solver tag reference", got, "want", tagBefore)
	}
}

func TestSMTSolverExtendsUnsatCoreThroughQuantifierPatterns(t *testing.T) {
	m, s := smtSolverFixture(t)
	sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "core-pattern-sort"), nil)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	function, err := m.MkFuncDecl(astTestSymbol(t, m, "core-pattern-f"), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	predicate, err := m.MkFuncDecl(astTestSymbol(t, m, "core-pattern-p"), []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	variable, err := m.MkVar(0, sort)
	astTestMust(t, err)
	functionVar, err := m.MkApp(function, []ast.ASTHandle{variable})
	astTestMust(t, err)
	body, err := m.MkApp(predicate, []ast.ASTHandle{functionVar})
	astTestMust(t, err)
	pattern, err := m.MkPattern([]ast.ASTHandle{functionVar})
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{astTestSymbol(t, m, "core-pattern-x")},
		Body: body, Patterns: []ast.ASTHandle{pattern},
	})
	astTestMust(t, err)
	constant, err := m.MkConst(astTestSymbol(t, m, "core-pattern-a"), sort)
	astTestMust(t, err)
	functionConstant, err := m.MkApp(function, []ast.ASTHandle{constant})
	astTestMust(t, err)
	conflict := smtRelevancyBool(t, m, "core-pattern-conflict")
	combined, err := m.MkAnd(conflict, quantifier)
	astTestMust(t, err)
	notConflict, err := m.MkNot(conflict)
	astTestMust(t, err)
	extra, err := m.MkEq(functionConstant, constant)
	astTestMust(t, err)
	quantifierTag := smtRelevancyBool(t, m, "core-pattern-quantifier-tag")
	negativeTag := smtRelevancyBool(t, m, "core-pattern-negative-tag")
	extraTag := smtRelevancyBool(t, m, "core-pattern-extra-tag")
	astTestMust(t, s.AssertTracked(combined, quantifierTag))
	astTestMust(t, s.AssertTracked(notConflict, negativeTag))
	astTestMust(t, s.AssertTracked(extra, extraTag))

	params := s.config.Params.EmptyValue()
	t.Cleanup(func() { astTestMust(t, s.config.Params.Del(&params)) })
	name := "core.extend_patterns"
	astTestMust(t, s.config.Params.SetBoolName(&params, &name, true))
	name = "core.extend_patterns.max_distance"
	astTestMust(t, s.config.Params.SetUIntName(&params, &name, 0))
	astTestMust(t, s.UpdateParams(&params))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("pattern core status", status)
	}
	core, err := s.UnsatCore()
	astTestMust(t, err)
	if len(core) != 2 {
		t.Fatal("zero-distance pattern core", core)
	}
	astTestMust(t, s.config.Params.SetUIntName(&params, &name, 1))
	astTestMust(t, s.UpdateParams(&params))
	status, err = s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("pattern core status after distance update", status)
	}
	core, err = s.UnsatCore()
	astTestMust(t, err)
	seen := make(map[ast.ASTHandle]bool, len(core))
	for _, member := range core {
		seen[member] = true
	}
	if len(core) != 3 || !seen[quantifierTag] || !seen[negativeTag] || !seen[extraTag] {
		t.Fatal("pattern-extended core", core)
	}
}

func TestSMTSolverExtendsUnsatCoreWithNonlocalPatterns(t *testing.T) {
	m, s := smtSolverFixture(t)
	conflict := smtRelevancyBool(t, m, "nonlocal-core-conflict")
	notConflict, err := m.MkNot(conflict)
	astTestMust(t, err)
	positiveTag := smtRelevancyBool(t, m, "nonlocal-core-positive-tag")
	negativeTag := smtRelevancyBool(t, m, "nonlocal-core-negative-tag")
	quantifierTag := smtRelevancyBool(t, m, "nonlocal-core-quantifier-tag")
	astTestMust(t, s.AssertTracked(conflict, positiveTag))
	astTestMust(t, s.AssertTracked(notConflict, negativeTag))

	sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "nonlocal-core-sort"), nil)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	function, err := m.MkFuncDecl(astTestSymbol(t, m, "nonlocal-core-f"), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	predicate, err := m.MkFuncDecl(astTestSymbol(t, m, "nonlocal-core-p"), []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	variable, err := m.MkVar(0, sort)
	astTestMust(t, err)
	body, err := m.MkApp(predicate, []ast.ASTHandle{variable})
	astTestMust(t, err)
	functionVar, err := m.MkApp(function, []ast.ASTHandle{variable})
	astTestMust(t, err)
	pattern, err := m.MkPattern([]ast.ASTHandle{functionVar})
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{astTestSymbol(t, m, "nonlocal-core-x")},
		Body: body, Patterns: []ast.ASTHandle{pattern},
	})
	astTestMust(t, err)
	constant, err := m.MkConst(astTestSymbol(t, m, "nonlocal-core-a"), sort)
	astTestMust(t, err)
	functionConstant, err := m.MkApp(function, []ast.ASTHandle{constant})
	astTestMust(t, err)
	outsideBody, err := m.MkEq(functionConstant, constant)
	astTestMust(t, err)
	combined, err := m.MkAnd(quantifier, outsideBody)
	astTestMust(t, err)
	astTestMust(t, s.AssertTracked(combined, quantifierTag))

	params := s.config.Params.EmptyValue()
	t.Cleanup(func() { astTestMust(t, s.config.Params.Del(&params)) })
	name := "core.extend_nonlocal_patterns"
	astTestMust(t, s.config.Params.SetBoolName(&params, &name, true))
	astTestMust(t, s.UpdateParams(&params))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("nonlocal pattern core status", status)
	}
	core, err := s.UnsatCore()
	astTestMust(t, err)
	seen := make(map[ast.ASTHandle]bool, len(core))
	for _, member := range core {
		seen[member] = true
	}
	if len(core) != 3 || !seen[positiveTag] || !seen[negativeTag] || !seen[quantifierTag] {
		t.Fatal("nonlocal pattern-extended core", core)
	}
}

func TestSMTInternalizerImplicationGateWithoutPreprocessing(t *testing.T) {
	m := astTestManager(t)
	a := smtRelevancyBool(t, m, "smt-implies-a")
	b := smtRelevancyBool(t, m, "smt-implies-b")
	implies, err := m.MkImplies(a, b)
	astTestMust(t, err)
	notImplies, err := m.MkNot(implies)
	astTestMust(t, err)
	notA, err := m.MkNot(a)
	astTestMust(t, err)
	kernel := smtTestKernel(t, m)
	astTestMust(t, kernel.Assert(notImplies, ast.ASTHandle{}))

	status, err := kernel.CheckAssumptions([]ast.ASTHandle{notA})
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("negated implication did not force antecedent", status)
	}
	status, err = kernel.CheckAssumptions([]ast.ASTHandle{b})
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("negated implication did not refute consequent", status)
	}
}

func TestSMTSolverPublishesKernelUnsatProof(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	s, err := NewSMTSolver(m, nil, "QF_UF", SMTSolverConfig{Params: params})
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, s.Close())
		astTestMust(t, params.Close())
	})
	falsity, err := m.MkFalse()
	astTestMust(t, err)
	astTestMust(t, s.Assert(falsity))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("false assertion status", status)
	}
	proof, err := s.Proof()
	astTestMust(t, err)
	if proof.IsNull() {
		t.Fatal("missing unsat proof")
	}
	fact, err := m.GetProofFact(proof)
	astTestMust(t, err)
	if fact != falsity {
		t.Fatal("unsat proof fact", fact, falsity)
	}

	other := smtTestKernel(t, m)
	a := smtRelevancyBool(t, m, "smt-proof-reset-a")
	notA, err := m.MkNot(a)
	astTestMust(t, err)
	astTestMust(t, other.Assert(a, ast.ASTHandle{}))
	status, err = other.CheckAssumptions([]ast.ASTHandle{notA})
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("assumption proof status", status)
	}
	proof, err = other.Proof()
	astTestMust(t, err)
	if proof.IsNull() {
		t.Fatal("missing assumption conflict proof")
	}
	status, err = other.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("post-conflict status", status)
	}
	proof, err = other.Proof()
	astTestMust(t, err)
	if !proof.IsNull() {
		t.Fatal("stale unsat proof", proof)
	}
}

func TestSMTSolverReasonUnknownUsesKernelFailure(t *testing.T) {
	m, s := smtSolverFixture(t)
	astTestMust(t, s.SetReasonUnknown("manual reason"))
	reason, err := s.ReasonUnknown()
	astTestMust(t, err)
	if reason != "manual reason" {
		t.Fatal("manual reason", reason)
	}
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	dummy, err := NewSMTDummyTheory(family, "no arithmetic")
	astTestMust(t, err)
	astTestMust(t, dummy.Reset())
	astTestMust(t, s.kernel.RegisterTheory(dummy))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.Undef {
		t.Fatal("incomplete theory status", status)
	}
	reason, err = s.ReasonUnknown()
	astTestMust(t, err)
	if reason != "(incomplete (theory no arithmetic))" {
		t.Fatal("incomplete theory reason", reason)
	}
	failure, err := s.kernel.LastFailure()
	astTestMust(t, err)
	if failure != SMTFailureTheory || failure.String() != "THEORY" {
		t.Fatal("last failure", failure)
	}
}

func TestSMTSolverPreparesIncompleteTheoryCandidateModel(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	s, err := NewSMTSolver(m, nil, "QF_UF", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	astTestMust(t, s.SetProduceModels(true))
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	dummy, err := NewSMTDummyTheory(family, "candidate")
	astTestMust(t, err)
	astTestMust(t, dummy.Reset())
	astTestMust(t, s.kernel.RegisterTheory(dummy))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.Undef {
		t.Fatal("candidate status", status)
	}
	available, err := s.PrepareCandidateModel()
	astTestMust(t, err)
	if !available {
		t.Fatal("incomplete theory did not expose a candidate model")
	}
	candidate, err := s.AcquireModel()
	astTestMust(t, err)
	if candidate == nil {
		t.Fatal("candidate model was not published")
	}
	astTestMust(t, candidate.DecRef())
}

func TestSMTSolverPublishesRelevantLabelsInInternalizationOrder(t *testing.T) {
	m, s := smtSolverFixture(t)
	first := astTestSymbol(t, m, "smt-result-label-first")
	second := astTestSymbol(t, m, "smt-result-label-second")
	literal, err := m.MkLabelLit([]usymbol.Symbol{first, second})
	astTestMust(t, err)
	astTestMust(t, s.Assert(literal))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("label status", status)
	}
	labels, err := s.Labels()
	astTestMust(t, err)
	if len(labels) != 2 || labels[0] != first || labels[1] != second {
		t.Fatal("relevant labels", labels)
	}

	astTestMust(t, s.Push())
	falsity, err := m.MkFalse()
	astTestMust(t, err)
	astTestMust(t, s.Assert(falsity))
	status, err = s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("scoped false status", status)
	}
	labels, err = s.Labels()
	astTestMust(t, err)
	if len(labels) != 0 {
		t.Fatal("stale satisfiable labels", labels)
	}
}

func TestSMTSolverFactoryAndUnsupportedKernelPrerequisites(t *testing.T) {
	m := astTestManager(t)
	params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, params.Close()) })
	factory, err := NewSMTSolverFactory(SMTSolverConfig{Params: params})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, factory.Close()) })
	value, err := factory.New(m, nil, false, true, true, "QF_UF")
	astTestMust(t, err)
	s, ok := value.(*SMTSolver)
	if !ok || !s.produceModels {
		t.Fatal("factory result", value)
	}
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	translated, err := s.Translate(m, nil)
	astTestMust(t, err)
	astTestMust(t, translated.Close())
	astTestMust(t, s.Push())
	if _, err = s.Translate(m, nil); err == nil {
		t.Fatal("scoped kernel translation accepted")
	}
	astTestMust(t, s.Pop(1))
	if status, _, err := s.GetConsequences(nil, nil); status != lbool.Undef || !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("kernel consequences prerequisite", status, err)
	}
	if status, mutexes, err := s.FindMutexes(nil); status != lbool.True || err != nil || len(mutexes) != 0 {
		t.Fatal("empty kernel mutex query", status, mutexes, err)
	}
	if err = s.Pop(1); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("scope underflow", err)
	}
	var _ solver.Solver = s
}

func TestSMTSolverTranslatesBaseKernelAndTrackedAssertions(t *testing.T) {
	source := astTestManager(t)
	target := astTestManager(t)
	sourceParams, err := uparams.NewParamsContext(source.Symbols, source.NumeralService(), uparams.ParamsConfig{Budget: source.Budget()})
	astTestMust(t, err)
	targetParams, err := uparams.NewParamsContext(target.Symbols, target.NumeralService(), uparams.ParamsConfig{Budget: target.Budget()})
	astTestMust(t, err)
	targetConfig := SMTSolverConfig{Params: targetParams}
	sourceConfig := SMTSolverConfig{Params: sourceParams, TargetConfig: func(manager *ast.ASTManager) (SMTSolverConfig, error) {
		if manager != target {
			return SMTSolverConfig{}, ast.ErrASTHandle
		}
		return targetConfig, nil
	}}
	s, err := NewSMTSolver(source, nil, "QF_UF", sourceConfig)
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, s.Close())
		astTestMust(t, targetParams.Close())
		astTestMust(t, sourceParams.Close())
	})
	a := smtRelevancyBool(t, source, "smt-translate-a")
	b := smtRelevancyBool(t, source, "smt-translate-b")
	tag := smtRelevancyBool(t, source, "smt-translate-tag")
	astTestMust(t, s.Assert(b))
	astTestMust(t, s.AssertTracked(a, tag))

	value, err := s.Translate(target, nil)
	astTestMust(t, err)
	translated, ok := value.(*SMTSolver)
	if !ok {
		t.Fatal("translated solver type", value)
	}
	t.Cleanup(func() { astTestMust(t, translated.Close()) })
	if count, err := translated.NumAssertions(); err != nil || count != 3 {
		t.Fatal("translated copied and named assertions", count, err)
	}
	if count, err := translated.NumAssumptions(); err != nil || count != 1 {
		t.Fatal("translated named assumptions", count, err)
	}
	translation, err := ast.NewASTTranslation(source, target, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, translation.Close()) })
	notA, err := source.MkNot(a)
	astTestMust(t, err)
	targetNotA, err := translation.Translate(notA)
	astTestMust(t, err)
	status, err := translated.CheckSat([]ast.ASTHandle{targetNotA})
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("translated tracked assertion", status)
	}
}

func TestSMTSolverTranslatesWithinManager(t *testing.T) {
	m, s := smtSolverFixture(t)
	params, err := s.Params()
	astTestMust(t, err)
	translatedValue, err := s.Translate(m, params)
	astTestMust(t, err)
	translated := translatedValue.(*SMTSolver)
	t.Cleanup(func() { astTestMust(t, translated.Close()) })
	status, err := translated.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("same-manager translated status", status)
	}
}

func TestSMTSolverSetupUsesLinearRealArithmetic(t *testing.T) {
	m := astTestManager(t)
	arithmetic, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	s, err := NewSMTSolver(m, nil, "QF_LRA", SMTSolverConfig{Params: params})
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, s.Close())
		astTestMust(t, params.Close())
	})
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("unused arithmetic setup", status)
	}
	two, err := m.ArithmeticRealNumeralText("2")
	astTestMust(t, err)
	realSort, err := m.GetSort(two)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "linear-lra-x"), realSort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "linear-lra-y"), realSort)
	astTestMust(t, err)
	sum, err := m.MkFamilyApp(arithmetic, smtArithOpAdd, nil, []ast.ASTHandle{x, y})
	astTestMust(t, err)
	comparison, err := m.MkFamilyApp(arithmetic, smtArithOpLT, nil, []ast.ASTHandle{sum, two})
	astTestMust(t, err)
	astTestMust(t, s.Assert(comparison))
	status, err = s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("strict linear arithmetic", status)
	}
	astTestMust(t, s.Push())
	one, err := m.ArithmeticRealNumeralText("1")
	astTestMust(t, err)
	for _, expression := range []ast.ASTHandle{x, y} {
		lower, lowerErr := m.MkFamilyApp(arithmetic, smtArithOpGE, nil, []ast.ASTHandle{expression, one})
		astTestMust(t, lowerErr)
		astTestMust(t, s.Assert(lower))
	}
	status, err = s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("infeasible linear arithmetic", status)
	}
	astTestMust(t, s.Pop(1))
	status, err = s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("linear arithmetic scope replay", status)
	}
}

func TestSMTSolverCollectsAndResetsLinearArithmeticStatistics(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	one, err := m.ArithmeticRealNumeralText("1")
	astTestMust(t, err)
	realSort, err := m.GetSort(one)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "linear-statistics-x"), realSort)
	astTestMust(t, err)
	bound, err := m.MkFamilyApp(family, smtArithOpLE, nil, []ast.ASTHandle{x, one})
	astTestMust(t, err)
	params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	s, err := NewSMTSolver(m, nil, "QF_LRA", SMTSolverConfig{Params: params})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()); astTestMust(t, params.Close()) })
	astTestMust(t, s.Assert(bound))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("statistics arithmetic status", status)
	}
	stats := statistics.New()
	astTestMust(t, s.CollectStatistics(stats))
	found := false
	for index := 0; index < stats.Size(); index++ {
		if stats.Key(index) == "arith-rows" {
			found = stats.IsUint(index) && stats.UintValue(index) != 0
		}
	}
	if !found {
		t.Fatal("missing arithmetic statistics")
	}
	var beforeReset uint32
	for i := 0; i < stats.Size(); i++ {
		if stats.Key(i) == "arith-rows" {
			beforeReset = stats.UintValue(i)
		}
	}
	astTestMust(t, s.kernel.ResetStatistics())
	stats.Reset()
	astTestMust(t, s.CollectStatistics(stats))
	found = false
	for index := 0; index < stats.Size(); index++ {
		if stats.Key(index) == "arith-rows" {
			found = stats.IsUint(index) && stats.UintValue(index) == beforeReset
		}
	}
	if !found {
		t.Fatal("source no-op reset changed arithmetic statistics")
	}
}

func TestSMTSolverLinearArithmeticEqualityAdapter(t *testing.T) {
	for _, disequality := range []bool{false, true} {
		name := "equality"
		if disequality {
			name = "disequality"
		}
		t.Run(name, func(t *testing.T) {
			m := astTestManager(t)
			family, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			zero, err := m.ArithmeticRealNumeralText("0")
			astTestMust(t, err)
			realSort, err := m.GetSort(zero)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "linear-equality-x"), realSort)
			astTestMust(t, err)
			y, err := m.MkConst(astTestSymbol(t, m, "linear-equality-y"), realSort)
			astTestMust(t, err)
			params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
			astTestMust(t, err)
			s, err := NewSMTSolver(m, nil, "QF_LRA", SMTSolverConfig{Params: params})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, s.Close()); astTestMust(t, params.Close()) })
			equality, err := m.MkEq(x, y)
			astTestMust(t, err)
			if disequality {
				equality, err = m.MkNot(equality)
				astTestMust(t, err)
			}
			astTestMust(t, s.Assert(equality))
			ops := []int32{smtArithOpLT}
			if disequality {
				ops = []int32{smtArithOpLE, smtArithOpGE}
			}
			for _, op := range ops {
				comparison, comparisonErr := m.MkFamilyApp(family, op, nil, []ast.ASTHandle{x, y})
				astTestMust(t, comparisonErr)
				astTestMust(t, s.Assert(comparison))
			}
			status, err := s.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("arithmetic equality adapter", status)
			}
		})
	}
}

func TestSMTSolverUFLinearArithmeticCongruence(t *testing.T) {
	for _, test := range []struct {
		logic   string
		integer bool
	}{
		{logic: "QF_UFLRA"},
		{logic: "QF_UFLIA", integer: true},
	} {
		t.Run(test.logic, func(t *testing.T) {
			m := astTestManager(t)
			family, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			sortKind := int32(0)
			if test.integer {
				sortKind = 1
			}
			numberSort, err := m.MkFamilySort(family, sortKind, nil)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "uf-linear-x"), numberSort)
			astTestMust(t, err)
			y, err := m.MkConst(astTestSymbol(t, m, "uf-linear-y"), numberSort)
			astTestMust(t, err)
			function, err := m.MkFuncDecl(astTestSymbol(t, m, "uf-linear-f"), []ast.ASTHandle{numberSort}, numberSort, ast.ASTFuncDeclInfo{})
			astTestMust(t, err)
			fx, err := m.MkApp(function, []ast.ASTHandle{x})
			astTestMust(t, err)
			fy, err := m.MkApp(function, []ast.ASTHandle{y})
			astTestMust(t, err)
			params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
			astTestMust(t, err)
			s, err := NewSMTSolver(m, nil, test.logic, SMTSolverConfig{Params: params})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, s.Close()); astTestMust(t, params.Close()) })
			equality, err := m.MkEq(x, y)
			astTestMust(t, err)
			astTestMust(t, s.Assert(equality))
			less, err := m.MkFamilyApp(family, smtArithOpLT, nil, []ast.ASTHandle{fx, fy})
			astTestMust(t, err)
			astTestMust(t, s.Assert(less))
			status, err := s.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("UF arithmetic congruence", status)
			}
		})
	}
}

func TestSMTSolverLinearArithmeticPropagatesImpliedEquality(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	realSort, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "implied-equality-x"), realSort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "implied-equality-y"), realSort)
	astTestMust(t, err)
	function, err := m.MkFuncDecl(astTestSymbol(t, m, "implied-equality-f"), []ast.ASTHandle{realSort}, realSort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	fx, err := m.MkApp(function, []ast.ASTHandle{x})
	astTestMust(t, err)
	fy, err := m.MkApp(function, []ast.ASTHandle{y})
	astTestMust(t, err)
	params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	s, err := NewSMTSolver(m, nil, "QF_UFLRA", SMTSolverConfig{Params: params})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()); astTestMust(t, params.Close()) })
	for _, op := range []int32{smtArithOpLE, smtArithOpGE} {
		comparison, comparisonErr := m.MkFamilyApp(family, op, nil, []ast.ASTHandle{x, y})
		astTestMust(t, comparisonErr)
		astTestMust(t, s.Assert(comparison))
	}
	equality, err := m.MkEq(fx, fy)
	astTestMust(t, err)
	disequality, err := m.MkNot(equality)
	astTestMust(t, err)
	astTestMust(t, s.Assert(disequality))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("implied arithmetic equality", status)
	}
}

func TestSMTSolverLinearArithmeticPropagatesFixedEquality(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	one, err := m.ArithmeticRealNumeralText("1")
	astTestMust(t, err)
	realSort, err := m.GetSort(one)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "fixed-equality-x"), realSort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "fixed-equality-y"), realSort)
	astTestMust(t, err)
	function, err := m.MkFuncDecl(astTestSymbol(t, m, "fixed-equality-f"), []ast.ASTHandle{realSort}, realSort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	fx, err := m.MkApp(function, []ast.ASTHandle{x})
	astTestMust(t, err)
	fy, err := m.MkApp(function, []ast.ASTHandle{y})
	astTestMust(t, err)
	params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	s, err := NewSMTSolver(m, nil, "QF_UFLRA", SMTSolverConfig{Params: params})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()); astTestMust(t, params.Close()) })
	for _, expression := range []ast.ASTHandle{x, y} {
		for _, op := range []int32{smtArithOpLE, smtArithOpGE} {
			comparison, comparisonErr := m.MkFamilyApp(family, op, nil, []ast.ASTHandle{expression, one})
			astTestMust(t, comparisonErr)
			astTestMust(t, s.Assert(comparison))
		}
	}
	equality, err := m.MkEq(fx, fy)
	astTestMust(t, err)
	disequality, err := m.MkNot(equality)
	astTestMust(t, err)
	astTestMust(t, s.Assert(disequality))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("fixed arithmetic equality", status)
	}
}

func TestSMTSolverLinearArithmeticPropagatesOffsetEquality(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	one, err := m.ArithmeticRealNumeralText("1")
	astTestMust(t, err)
	minusOne, err := m.ArithmeticRealNumeralText("-1")
	astTestMust(t, err)
	realSort, err := m.GetSort(one)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "offset-equality-x"), realSort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "offset-equality-y"), realSort)
	astTestMust(t, err)
	z, err := m.MkConst(astTestSymbol(t, m, "offset-equality-z"), realSort)
	astTestMust(t, err)
	negativeY, err := m.MkFamilyApp(family, smtArithOpMul, nil, []ast.ASTHandle{minusOne, y})
	astTestMust(t, err)
	offset, err := m.MkFamilyApp(family, smtArithOpAdd, nil, []ast.ASTHandle{x, negativeY, z})
	astTestMust(t, err)
	function, err := m.MkFuncDecl(astTestSymbol(t, m, "offset-equality-f"), []ast.ASTHandle{realSort}, realSort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	fx, err := m.MkApp(function, []ast.ASTHandle{x})
	astTestMust(t, err)
	fy, err := m.MkApp(function, []ast.ASTHandle{y})
	astTestMust(t, err)
	params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	s, err := NewSMTSolver(m, nil, "QF_UFLRA", SMTSolverConfig{Params: params})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()); astTestMust(t, params.Close()) })
	for _, expression := range []ast.ASTHandle{z, offset} {
		for _, op := range []int32{smtArithOpLE, smtArithOpGE} {
			comparison, comparisonErr := m.MkFamilyApp(family, op, nil, []ast.ASTHandle{expression, one})
			astTestMust(t, comparisonErr)
			astTestMust(t, s.Assert(comparison))
		}
	}
	equality, err := m.MkEq(fx, fy)
	astTestMust(t, err)
	disequality, err := m.MkNot(equality)
	astTestMust(t, err)
	astTestMust(t, s.Assert(disequality))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("offset arithmetic equality", status)
	}
}

func TestSMTSolverLinearArithmeticAcceptsNonlinearZeroCandidate(t *testing.T) {
	m := astTestManager(t)
	arithmetic, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	s, err := NewSMTSolver(m, nil, "QF_NRA", SMTSolverConfig{Params: params})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()); astTestMust(t, params.Close()) })
	one, err := m.ArithmeticRealNumeralText("1")
	astTestMust(t, err)
	realSort, err := m.GetSort(one)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "nonlinear-lra-x"), realSort)
	astTestMust(t, err)
	product, err := m.MkFamilyApp(arithmetic, smtArithOpMul, nil, []ast.ASTHandle{x, x})
	astTestMust(t, err)
	comparison, err := m.MkFamilyApp(arithmetic, smtArithOpLT, nil, []ast.ASTHandle{product, one})
	astTestMust(t, err)
	astTestMust(t, s.Assert(comparison))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("nonlinear zero candidate", status)
	}
}

func TestSMTSolverLinearArithmeticSquareNonnegative(t *testing.T) {
	for _, test := range []struct {
		name       string
		op         int32
		squareLeft bool
		negate     bool
		negative   bool
		twoSquares bool
		want       lbool.LBool
	}{
		{name: "square-ge-zero", op: smtArithOpGE, squareLeft: true, want: lbool.True},
		{name: "square-lt-zero", op: smtArithOpLT, squareLeft: true, want: lbool.False},
		{name: "zero-le-square", op: smtArithOpLE, want: lbool.True},
		{name: "zero-gt-square", op: smtArithOpGT, want: lbool.False},
		{name: "negated-square-ge-zero", op: smtArithOpGE, squareLeft: true, negate: true, want: lbool.False},
		{name: "two-even-powers-ge-zero", op: smtArithOpGE, squareLeft: true, twoSquares: true, want: lbool.True},
		{name: "negative-square-le-zero", op: smtArithOpLE, squareLeft: true, negative: true, want: lbool.True},
		{name: "negative-square-gt-zero", op: smtArithOpGT, squareLeft: true, negative: true, want: lbool.False},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := astTestManager(t)
			arithmetic, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
			astTestMust(t, err)
			s, err := NewSMTSolver(m, nil, "QF_LRA", SMTSolverConfig{Params: params})
			astTestMust(t, err)
			// Retain the legacy nonlinear tableau as the explicit test target.
			smtDisableAutoConfig(t, s)
			t.Cleanup(func() { astTestMust(t, s.Close()); astTestMust(t, params.Close()) })
			zero, err := m.ArithmeticRealNumeralText("0")
			astTestMust(t, err)
			realSort, err := m.GetSort(zero)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "square-x"), realSort)
			astTestMust(t, err)
			factors := []ast.ASTHandle{x, x}
			if test.twoSquares {
				y, makeErr := m.MkConst(astTestSymbol(t, m, "square-y"), realSort)
				astTestMust(t, makeErr)
				factors = append(factors, y, y)
			}
			if test.negative {
				minusOne, makeErr := m.ArithmeticRealNumeralText("-1")
				astTestMust(t, makeErr)
				factors = append([]ast.ASTHandle{minusOne}, factors...)
			}
			square, err := m.MkFamilyApp(arithmetic, smtArithOpMul, nil, factors)
			astTestMust(t, err)
			left, right := zero, square
			if test.squareLeft {
				left, right = square, zero
			}
			comparison, err := m.MkFamilyApp(arithmetic, test.op, nil, []ast.ASTHandle{left, right})
			astTestMust(t, err)
			if test.negate {
				comparison, err = m.MkNot(comparison)
				astTestMust(t, err)
			}
			astTestMust(t, s.Assert(comparison))
			status, err := s.CheckSat(nil)
			astTestMust(t, err)
			if status != test.want {
				t.Fatalf("square comparison status %v, want %v", status, test.want)
			}
		})
	}
}

func TestSMTSolverLinearArithmeticEvaluatesFixedPolynomial(t *testing.T) {
	for _, test := range []struct {
		name string
		op   int32
		want lbool.LBool
	}{
		{name: "consistent", op: smtArithOpGT, want: lbool.True},
		{name: "conflict", op: smtArithOpLT, want: lbool.False},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := astTestManager(t)
			arithmetic, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
			astTestMust(t, err)
			s, err := NewSMTSolver(m, nil, "QF_LRA", SMTSolverConfig{Params: params})
			astTestMust(t, err)
			// Retain the legacy nonlinear tableau as the explicit test target.
			smtDisableAutoConfig(t, s)
			t.Cleanup(func() { astTestMust(t, s.Close()); astTestMust(t, params.Close()) })
			two, err := m.ArithmeticRealNumeralText("2")
			astTestMust(t, err)
			three, err := m.ArithmeticRealNumeralText("3")
			astTestMust(t, err)
			five, err := m.ArithmeticRealNumeralText("5")
			astTestMust(t, err)
			realSort, err := m.GetSort(two)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "fixed-polynomial-x"), realSort)
			astTestMust(t, err)
			y, err := m.MkConst(astTestSymbol(t, m, "fixed-polynomial-y"), realSort)
			astTestMust(t, err)
			for _, fixed := range []struct {
				variable ast.ASTHandle
				value    ast.ASTHandle
			}{{x, two}, {y, three}} {
				for _, op := range []int32{smtArithOpLE, smtArithOpGE} {
					bound, makeErr := m.MkFamilyApp(arithmetic, op, nil, []ast.ASTHandle{fixed.variable, fixed.value})
					astTestMust(t, makeErr)
					astTestMust(t, s.Assert(bound))
				}
			}
			product, err := m.MkFamilyApp(arithmetic, smtArithOpMul, nil, []ast.ASTHandle{x, y})
			astTestMust(t, err)
			comparison, err := m.MkFamilyApp(arithmetic, test.op, nil, []ast.ASTHandle{product, five})
			astTestMust(t, err)
			astTestMust(t, s.Assert(comparison))
			status, err := s.CheckSat(nil)
			astTestMust(t, err)
			if status != test.want {
				t.Fatalf("fixed polynomial status %v, want %v", status, test.want)
			}
		})
	}
}

func TestSMTSolverLinearArithmeticPropagatesPolynomialIntervals(t *testing.T) {
	for _, test := range []struct {
		name      string
		op        int32
		threshold string
		want      lbool.LBool
	}{
		{name: "proved", op: smtArithOpGT, threshold: "8", want: lbool.True},
		{name: "refuted", op: smtArithOpLT, threshold: "8", want: lbool.False},
		{name: "overlap", op: smtArithOpLT, threshold: "9", want: lbool.True},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := astTestManager(t)
			arithmetic, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
			astTestMust(t, err)
			s, err := NewSMTSolver(m, nil, "QF_LRA", SMTSolverConfig{Params: params})
			astTestMust(t, err)
			// Retain the legacy nonlinear tableau as the explicit test target.
			smtDisableAutoConfig(t, s)
			t.Cleanup(func() { astTestMust(t, s.Close()); astTestMust(t, params.Close()) })
			two, err := m.ArithmeticRealNumeralText("2")
			astTestMust(t, err)
			three, err := m.ArithmeticRealNumeralText("3")
			astTestMust(t, err)
			four, err := m.ArithmeticRealNumeralText("4")
			astTestMust(t, err)
			five, err := m.ArithmeticRealNumeralText("5")
			astTestMust(t, err)
			threshold, err := m.ArithmeticRealNumeralText(test.threshold)
			astTestMust(t, err)
			realSort, err := m.GetSort(two)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "interval-polynomial-x"), realSort)
			astTestMust(t, err)
			y, err := m.MkConst(astTestSymbol(t, m, "interval-polynomial-y"), realSort)
			astTestMust(t, err)
			for _, bound := range []struct {
				variable ast.ASTHandle
				op       int32
				value    ast.ASTHandle
			}{{x, smtArithOpGT, two}, {x, smtArithOpLE, three}, {y, smtArithOpGE, four}, {y, smtArithOpLE, five}} {
				comparison, makeErr := m.MkFamilyApp(arithmetic, bound.op, nil, []ast.ASTHandle{bound.variable, bound.value})
				astTestMust(t, makeErr)
				astTestMust(t, s.Assert(comparison))
			}
			product, err := m.MkFamilyApp(arithmetic, smtArithOpMul, nil, []ast.ASTHandle{x, y})
			astTestMust(t, err)
			comparison, err := m.MkFamilyApp(arithmetic, test.op, nil, []ast.ASTHandle{product, threshold})
			astTestMust(t, err)
			astTestMust(t, s.Assert(comparison))
			status, err := s.CheckSat(nil)
			astTestMust(t, err)
			if status != test.want {
				t.Fatalf("polynomial interval status %v, want %v", status, test.want)
			}
		})
	}
}

func TestSMTSolverSetupUsesPartialLinearIntegerArithmetic(t *testing.T) {
	for _, test := range []struct {
		name string
		mode int
		want lbool.LBool
	}{
		{name: "strict-conflict", want: lbool.False},
		{name: "parity-conflict", mode: 1, want: lbool.False},
		{name: "integral-model", mode: 2, want: lbool.True},
		{name: "branch-to-integral-model", mode: 3, want: lbool.True},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := astTestManager(t)
			family, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			integer, err := m.MkFamilySort(family, 1, nil)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "linear-lia-x"), integer)
			astTestMust(t, err)
			y, err := m.MkConst(astTestSymbol(t, m, "linear-lia-y"), integer)
			astTestMust(t, err)
			one, err := m.ArithmeticNumeralRat(big.NewRat(1, 1), true)
			astTestMust(t, err)
			params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
			astTestMust(t, err)
			s, err := NewSMTSolver(m, nil, "QF_LIA", SMTSolverConfig{Params: params})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, s.Close()); astTestMust(t, params.Close()) })
			left := x
			firstOp := smtArithOpLT
			if test.mode == 1 {
				left, err = m.MkFamilyApp(family, smtArithOpAdd, nil, []ast.ASTHandle{x, x})
				astTestMust(t, err)
				firstOp = smtArithOpLE
			} else if test.mode == 2 {
				firstOp = smtArithOpLE
			} else if test.mode == 3 {
				left, err = m.MkFamilyApp(family, smtArithOpAdd, nil, []ast.ASTHandle{x, x, y})
				astTestMust(t, err)
				firstOp = smtArithOpLE
			}
			first, err := m.MkFamilyApp(family, firstOp, nil, []ast.ASTHandle{left, one})
			astTestMust(t, err)
			astTestPin(t, m, left) // Also used to build the second assertion.
			astTestMust(t, s.Assert(first))
			second, err := m.MkFamilyApp(family, smtArithOpGE, nil, []ast.ASTHandle{left, one})
			astTestMust(t, err)
			astTestMust(t, s.Assert(second))
			status, err := s.CheckSat(nil)
			astTestMust(t, err)
			if status != test.want {
				t.Fatal("linear integer status", status, "want", test.want)
			}
		})
	}
}

func TestSMTSolverSetupUsesIntegerDifferenceLogic(t *testing.T) {
	for _, test := range []struct {
		name   string
		second int64
		want   lbool.LBool
	}{
		{name: "feasible", second: 1, want: lbool.True},
		{name: "negative-cycle", second: -1, want: lbool.False},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := astTestManager(t)
			family, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			integer, err := m.MkFamilySort(family, 1, nil)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "solver-idl-x"), integer)
			astTestMust(t, err)
			y, err := m.MkConst(astTestSymbol(t, m, "solver-idl-y"), integer)
			astTestMust(t, err)
			params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
			astTestMust(t, err)
			solver, err := NewSMTSolver(m, nil, "QF_IDL", SMTSolverConfig{Params: params})
			astTestMust(t, err)
			t.Cleanup(func() {
				astTestMust(t, solver.Close())
				astTestMust(t, params.Close())
			})
			fixture := smtIntegerDiffFixture{manager: m, family: family, sort: integer, x: x, y: y}
			first := fixture.bound(t, smtArithOpLE, x, y, 0)
			second := fixture.bound(t, smtArithOpLE, y, x, test.second)
			astTestMust(t, solver.Assert(first))
			astTestMust(t, solver.Assert(second))
			status, err := solver.CheckSat(nil)
			astTestMust(t, err)
			if status != test.want {
				t.Fatal("integer difference-logic result", status, test.want)
			}
		})
	}
}

func TestSMTSolverIntegerDifferenceEqualityAdapter(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "solver-idl-eq-x"), integer)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "solver-idl-eq-y"), integer)
	astTestMust(t, err)
	params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "QF_IDL", SMTSolverConfig{Params: params})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()); astTestMust(t, params.Close()) })
	equality, err := m.MkEq(x, y)
	astTestMust(t, err)
	fixture := smtIntegerDiffFixture{manager: m, family: family, sort: integer, x: x, y: y}
	strict := fixture.bound(t, smtArithOpLE, x, y, -1)
	astTestMust(t, solver.Assert(equality))
	astTestMust(t, solver.Assert(strict))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("equality and strict difference", status)
	}
}

type smtSolverProgressRecorder struct{ fast, slow uint32 }

func (r *smtSolverProgressRecorder) FastProgressSample() { r.fast++ }
func (r *smtSolverProgressRecorder) SlowProgressSample() { r.slow++ }

func TestSMTSolverForwardsFastProgressSamples(t *testing.T) {
	m, s := smtSolverFixture(t)
	a := smtRelevancyBool(t, m, "smt-progress-a")
	b := smtRelevancyBool(t, m, "smt-progress-b")
	formula, err := m.MkOr(a, b)
	astTestMust(t, err)
	astTestMust(t, s.Assert(formula))
	recorder := &smtSolverProgressRecorder{}
	astTestMust(t, s.SetProgressCallback(recorder))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True || recorder.fast == 0 || recorder.slow != 0 {
		t.Fatal("progress callback", status, recorder.fast, recorder.slow)
	}
	before := recorder.fast
	astTestMust(t, s.SetProgressCallback(nil))
	_, err = s.CheckSat(nil)
	astTestMust(t, err)
	if recorder.fast != before {
		t.Fatal("detached progress callback", before, recorder.fast)
	}
}

func TestSMTKernelSamplesSlowProgressAtConfiguredCPUIntervals(t *testing.T) {
	m := astTestManager(t)
	kernel, err := NewSMTKernel(m, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, kernel.Close()) })
	var now uint64
	kernel.timerClock = func() (uint64, error) {
		now += 2_000_000
		return now, nil
	}
	kernel.smtState.ProgressSamplingFreq = 1
	recorder := &smtSolverProgressRecorder{}
	astTestMust(t, kernel.SetProgressCallback(recorder))
	a := smtRelevancyBool(t, m, "smt-slow-progress-a")
	b := smtRelevancyBool(t, m, "smt-slow-progress-b")
	formula, err := m.MkOr(a, b)
	astTestMust(t, err)
	astTestMust(t, kernel.Assert(formula, ast.ASTHandle{}))
	status, err := kernel.Check()
	astTestMust(t, err)
	if status != lbool.True || recorder.fast == 0 || recorder.slow == 0 {
		t.Fatal("progress callback", status, recorder.fast, recorder.slow)
	}
}
