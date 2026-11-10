package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func smtQuantifierManagerFixture(t *testing.T, eager, lazy float32, config SMTQuantifierManagerConfig) (*ast.ASTManager, *SMTInternalizerState, *SMTQuantifierRegistry, *SMTQuantifierManager) {
	t.Helper()
	m := astTestManager(t)
	costs, err := NewSMTQICostModel(m, "0", "0", nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, costs.Close()) })
	registry, err := NewSMTQuantifierRegistry(m, NewSMTQIQueueSchedule(m, eager, lazy, false), costs, 100)
	astTestMust(t, err)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	state, err := NewSMTInternalizerState(m, context, registry)
	astTestMust(t, err)
	manager, err := NewSMTQuantifierManager(state, smtQuickTheoryRewriter(t, m), config)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, state.Close()) })
	return m, state, registry, manager
}

func smtManagedQuantifier(t *testing.T, m *ast.ASTManager, name string) (ast.ASTHandle, ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	sort, err := m.MkSort(astTestSymbol(t, m, name+"-sort"), ast.ASTSortInfo{})
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	predicate := smtQuickPredicate(t, m, name+"-predicate", sort, boolean)
	variable, err := m.MkVar(0, sort)
	astTestMust(t, err)
	body, err := m.MkApp(predicate, []ast.ASTHandle{variable})
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{astTestSymbol(t, m, name+"-x")}, Body: body,
	})
	astTestMust(t, err)
	return quantifier, predicate, sort
}

func TestSMTQuantifierActivationHonorsDisabledRelevancyAndAssignment(t *testing.T) {
	m, state, _, manager := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{})
	relevancy, err := NewSMTRelevancyPropagator(state.context, true)
	astTestMust(t, err)
	astTestMust(t, state.SetRelevancy(relevancy))
	quantifier, _, _ := smtManagedQuantifier(t, m, "activation-relevancy")
	astTestMust(t, state.Internalize(quantifier, true))
	astTestMust(t, state.context.SetRelevant(quantifier, false))
	astTestMust(t, state.context.SetAssignment(quantifier, lbool.True))
	if manager.active(quantifier) {
		t.Fatal("enabled filtering activated an unmarked quantifier")
	}
	relevancy.enabled = false
	if !manager.active(quantifier) {
		t.Fatal("disabled filtering skipped an assigned quantifier")
	}
	for _, assignment := range []lbool.LBool{lbool.False, lbool.Undef} {
		astTestMust(t, state.context.SetAssignment(quantifier, assignment))
		if manager.active(quantifier) {
			t.Fatal("disabled filtering ignored the quantifier assignment", assignment)
		}
	}
}

func TestSMTQuantifierManagerNewEnodesAndCursorRollback(t *testing.T) {
	m, state, registry, manager := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{Ematching: true})
	relevancy, err := NewSMTRelevancyPropagator(state.context, false)
	astTestMust(t, err)
	astTestMust(t, state.SetRelevancy(relevancy))
	_, predicate, sort := smtManagedQuantifier(t, m, "new-enode")
	variable, err := m.MkVar(0, sort)
	astTestMust(t, err)
	body, err := m.MkApp(predicate, []ast.ASTHandle{variable})
	astTestMust(t, err)
	pattern, err := m.MkPattern([]ast.ASTHandle{body})
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body, Patterns: []ast.ASTHandle{pattern},
	})
	astTestMust(t, err)
	astTestMust(t, state.Internalize(quantifier, true))
	astTestMust(t, state.context.SetAssignment(quantifier, lbool.True))
	astTestMust(t, manager.AssignQuantifier(quantifier))
	astTestMust(t, manager.Propagate())
	if manager.CanPropagate() {
		t.Fatal("matching did not consume its initial work")
	}
	baseHead := manager.newEnodeHead
	for cycle := uint32(0); cycle < 2; cycle++ {
		astTestMust(t, state.PushScope())
		constant, err := m.MkConst(astTestSymbol(t, m, "new-enode-binding"), sort)
		astTestMust(t, err)
		application, err := m.MkApp(predicate, []ast.ASTHandle{constant})
		astTestMust(t, err)
		astTestMust(t, state.Internalize(application, true))
		astTestMust(t, state.context.SetAssignment(application, lbool.False))
		if !manager.CanPropagate() {
			t.Fatal("new unmarked enode did not expose E-matching work", cycle)
		}
		// propagate matches before reporting new enodes as relevant, so
		// their candidates are matched on the next round.
		for manager.CanPropagate() {
			astTestMust(t, manager.Propagate())
		}
		if registry.schedule.Stats().Instances != cycle+1 || manager.newEnodeHead != len(state.enodeStack) {
			t.Fatal("new enode was not matched", cycle, registry.schedule.Stats(), manager.newEnodeHead, len(state.enodeStack))
		}
		astTestMust(t, state.PopScopes(1))
		if manager.newEnodeHead != baseHead {
			t.Fatal("scoped enode cursor was not restored", cycle, manager.newEnodeHead, baseHead)
		}
	}
}

func TestSMTQuantifiedSearchRequiresModelValidation(t *testing.T) {
	m, solver := smtAssertedNNFFixture(t, false)
	local, err := solver.kernel.paramContext.NewValue()
	astTestMust(t, err)
	for _, name := range []string{"mbqi", "preprocess", "auto_config"} {
		astTestMust(t, solver.kernel.paramContext.SetBoolName(&local, &name, false))
	}
	name := "relevancy"
	astTestMust(t, solver.kernel.paramContext.SetUIntName(&local, &name, 0))
	astTestMust(t, solver.UpdateParams(&local))
	t.Cleanup(func() { astTestMust(t, solver.kernel.paramContext.Del(&local)) })
	sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "validation-sort"), nil)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	predicate := smtQuickPredicate(t, m, "validation-p", sort, boolean)
	variable, err := m.MkVar(0, sort)
	astTestMust(t, err)
	body, err := m.MkApp(predicate, []ast.ASTHandle{variable})
	astTestMust(t, err)
	pattern, err := m.MkPattern([]ast.ASTHandle{body})
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body, Patterns: []ast.ASTHandle{pattern},
	})
	astTestMust(t, err)
	astTestMust(t, solver.Assert(quantifier))
	for cycle := 0; cycle < 2; cycle++ {
		status, err := solver.CheckSat(nil)
		astTestMust(t, err)
		failure, err := solver.kernel.LastFailure()
		astTestMust(t, err)
		reason, err := solver.ReasonUnknown()
		astTestMust(t, err)
		if status != lbool.Undef || failure != SMTFailureQuantifiers || reason != "(incomplete quantifiers)" {
			t.Fatal("unchecked quantified model was accepted", status, failure, reason)
		}
		astTestMust(t, solver.Push())
		constant, err := m.MkConst(astTestSymbol(t, m, "validation-binding"), sort)
		astTestMust(t, err)
		application, err := m.MkApp(predicate, []ast.ASTHandle{constant})
		astTestMust(t, err)
		negative, err := m.MkNot(application)
		astTestMust(t, err)
		astTestMust(t, solver.Assert(negative))
		name := "ematching"
		astTestMust(t, solver.kernel.paramContext.SetBoolName(&local, &name, false))
		astTestMust(t, solver.UpdateParams(&local))
		status, err = solver.CheckSat(nil)
		astTestMust(t, err)
		if status != lbool.Undef {
			t.Fatal("disabled E-matching processed a fresh scoped term", cycle, status)
		}
		astTestMust(t, solver.kernel.paramContext.SetBoolName(&local, &name, true))
		astTestMust(t, solver.UpdateParams(&local))
		status, err = solver.CheckSat(nil)
		if err != nil {
			t.Fatal("scoped check", cycle, err)
		}
		if status != lbool.False {
			t.Fatal("E-matching failed to refute a grounded contradiction", cycle, status)
		}
		astTestMust(t, solver.Pop(1))
	}
}

func TestSMTQuantifierLiveParamsPreservePendingAndConstructorCaches(t *testing.T) {
	m, solver := smtAssertedNNFFixture(t, false)
	context := solver.kernel.paramContext
	local, err := context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Del(&local)) })
	for _, name := range []string{"qi.eager_threshold", "qi.lazy_threshold"} {
		astTestMust(t, context.SetDoubleName(&local, &name, 0))
	}
	name := "qi.max_instances"
	astTestMust(t, context.SetUIntName(&local, &name, 0))
	astTestMust(t, solver.UpdateParams(&local))
	state := solver.kernel.internalizer
	manager := state.quantifierManager
	quantifier, _, sort := smtManagedQuantifier(t, m, "live-qi-params")
	astTestMust(t, state.Internalize(quantifier, true))
	bindings := make([]*SMTEnode, 3)
	for index := range bindings {
		constant, err := m.MkConst(symbol.Numerical(2740+uint32(index)), sort)
		astTestMust(t, err)
		astTestMust(t, state.Internalize(constant, false))
		bindings[index] = state.context.enodes.get(constant)
	}
	registry := manager.registry
	first, err := registry.AddInstance(quantifier, ast.ASTHandle{}, bindings[:1], 50, 0, 0)
	astTestMust(t, err)
	blocked, err := registry.AddInstance(quantifier, ast.ASTHandle{}, bindings[1:2], 5, 0, 0)
	astTestMust(t, err)
	if !first || blocked {
		t.Fatal("source max_instances boundary was not applied", first, blocked)
	}
	queue := registry.schedule
	ready, err := queue.ScheduleNew(nil)
	astTestMust(t, err)
	if len(ready) != 0 || len(queue.delayedEntries) != 1 {
		t.Fatal("high-cost instance was not retained for delayed checking", ready)
	}
	done, ready, err := queue.FinalCheck()
	astTestMust(t, err)
	if !done || len(ready) != 0 {
		t.Fatal("zero lazy threshold processed an ineligible instance", done, ready)
	}
	name = "qi.max_instances"
	astTestMust(t, context.SetUIntName(&local, &name, 1))
	name = "qi.lazy_threshold"
	astTestMust(t, context.SetDoubleName(&local, &name, 100))
	name, replacementCost := "qi.cost", "999"
	astTestMust(t, context.SetStringName(&local, &name, &replacementCost))
	astTestMust(t, solver.UpdateParams(&local))
	second, err := registry.AddInstance(quantifier, ast.ASTHandle{}, bindings[1:2], 5, 0, 0)
	astTestMust(t, err)
	blocked, err = registry.AddInstance(quantifier, ast.ASTHandle{}, bindings[2:], 5, 0, 0)
	astTestMust(t, err)
	if !second || blocked {
		t.Fatal("live instance limit reset the count or kept its stale value", second, blocked)
	}
	ready, err = queue.ScheduleNew(nil)
	astTestMust(t, err)
	if len(ready) != 1 || ready[0].Cost != 5 || ready[0].Fingerprint.args[0] != bindings[1] {
		t.Fatal("constructor-cached eager threshold or cost function was replaced", ready)
	}
	done, ready, err = queue.FinalCheck()
	astTestMust(t, err)
	if done || len(ready) != 1 || ready[0].Fingerprint.args[0] != bindings[0] {
		t.Fatal("updated lazy threshold lost the pending instance", done, ready)
	}
}

func TestSMTQuantifierManagerPropagatesAndScopesQueuedInstances(t *testing.T) {
	m, state, registry, manager := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{})
	quantifier, _, sort := smtManagedQuantifier(t, m, "managed-queue")
	astTestMust(t, state.Internalize(quantifier, true))
	owner, binding := smtEnodeConstant(t, m, 2500, sort, 3)
	astTestMust(t, state.context.RegisterEnode(binding, true))
	inserted, err := registry.AddInstance(quantifier, ast.ASTHandle{}, []*SMTEnode{binding}, 3, 0, 0)
	astTestMust(t, err)
	if !inserted || !manager.CanPropagate() {
		t.Fatal("queued instance was not exposed as propagation work")
	}
	astTestMust(t, manager.Propagate())
	if manager.CanPropagate() || len(state.Clauses()) == 0 || registry.schedule.Stats().Instances != 1 {
		t.Fatal("queued instance was not materialized", manager.CanPropagate(), len(state.Clauses()), registry.schedule.Stats())
	}
	astTestMust(t, state.PushScope())
	astTestMust(t, state.PopScopes(1))
	if registry.scopeLevel != 0 {
		t.Fatal("quantifier registry scope was not synchronized", registry.scopeLevel)
	}
	_ = owner
}

func TestSMTQuantifierManagerFinalCheckRunsActiveQuickChecker(t *testing.T) {
	m, state, registry, manager := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{QuickUnsat: true})
	quantifier, predicate, sort := smtManagedQuantifier(t, m, "managed-quick")
	astTestMust(t, state.Internalize(quantifier, true))
	_, binding := smtEnodeConstant(t, m, 2510, sort, 1)
	astTestMust(t, state.context.RegisterEnode(binding, true))
	application, _ := smtQuickApplicationNode(t, m, state.context, predicate, binding, true)
	astTestMust(t, state.context.SetRelevant(quantifier, true))
	astTestMust(t, state.context.SetAssignment(quantifier, lbool.True))
	astTestMust(t, state.context.SetAssignment(application, lbool.False))
	status, err := manager.FinalCheck(true)
	astTestMust(t, err)
	if status != SMTFinalCheckContinue || registry.schedule.Stats().Instances != 1 || len(state.Clauses()) == 0 {
		t.Fatal("quick checker did not force and internalize a conflicting instance", status, registry.schedule.Stats(), len(state.Clauses()))
	}
	status, err = manager.FinalCheck(true)
	astTestMust(t, err)
	if status != SMTFinalCheckDone {
		t.Fatal("duplicate quick-check instance prevented completion", status)
	}
}

func TestSMTQuantifierManagerFiltersMBQIByQuantifierIDPrefix(t *testing.T) {
	m, _, _, manager := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{
		MBQI: true, MaxCexs: 1, MaxIterations: 10, MBQIID: "selected.",
	})
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	variable, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	makeQuantifier := func(name string, id symbol.Symbol) ast.ASTHandle {
		predicate := smtQuickPredicate(t, m, "mbqi-prefix-"+name, boolean, boolean)
		body, makeErr := m.MkApp(predicate, []ast.ASTHandle{variable})
		astTestMust(t, makeErr)
		value, makeErr := m.MkQuantifier(ast.ASTQuantifierSpec{
			Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{astTestSymbol(t, m, "mbqi-prefix-x")},
			Body: body, QID: id,
		})
		astTestMust(t, makeErr)
		return value
	}
	selected := makeQuantifier("selected", astTestSymbol(t, m, "selected.quantifier"))
	rejected := makeQuantifier("rejected", astTestSymbol(t, m, "other.quantifier"))
	numerical := makeQuantifier("numerical", symbol.Numerical(7))
	unnamed := makeQuantifier("unnamed", symbol.Symbol{})
	for _, test := range []struct {
		name       string
		quantifier ast.ASTHandle
		want       bool
	}{
		{"selected", selected, true}, {"rejected", rejected, false}, {"numerical", numerical, false}, {"unnamed", unnamed, false},
	} {
		got, err := manager.mbqiEnabled(test.quantifier)
		astTestMust(t, err)
		if got != test.want {
			t.Fatal(test.name, "MBQI id-prefix classification", got, "want", test.want)
		}
	}
}

func TestSMTQuantifierManagerEMatchingFeedsInstanceQueue(t *testing.T) {
	m, state, registry, manager := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{Ematching: true})
	sort, err := m.MkSort(astTestSymbol(t, m, "managed-ematch-sort"), ast.ASTSortInfo{})
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	predicate := smtQuickPredicate(t, m, "managed-ematch-p", sort, boolean)
	variable, err := m.MkVar(0, sort)
	astTestMust(t, err)
	body, err := m.MkApp(predicate, []ast.ASTHandle{variable})
	astTestMust(t, err)
	pattern, err := m.MkPattern([]ast.ASTHandle{body})
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{astTestSymbol(t, m, "managed-ematch-x")}, Body: body, Patterns: []ast.ASTHandle{pattern},
	})
	astTestMust(t, err)
	astTestMust(t, state.Internalize(quantifier, true))
	_, binding := smtEnodeConstant(t, m, 2520, sort, 2)
	astTestMust(t, state.context.RegisterEnode(binding, true))
	application, _ := smtQuickApplicationNode(t, m, state.context, predicate, binding, true)
	astTestMust(t, state.context.SetRelevant(quantifier, true))
	astTestMust(t, state.context.SetAssignment(quantifier, lbool.True))
	astTestMust(t, manager.AssignQuantifier(quantifier))
	astTestMust(t, state.context.SetAssignment(application, lbool.False))
	if !manager.CanPropagate() {
		t.Fatal("active patterned quantifier did not expose matching work")
	}
	astTestMust(t, manager.Propagate())
	if registry.schedule.Stats().Instances != 1 || len(state.Clauses()) == 0 {
		t.Fatal("E-matching did not materialize its instance", registry.schedule.Stats(), len(state.Clauses()))
	}
}
