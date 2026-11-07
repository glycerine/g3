package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type smtLazyMAMFixture struct {
	m                *ast.ASTManager
	state            *SMTInternalizerState
	registry         *SMTQuantifierRegistry
	manager          *SMTQuantifierManager
	quantifier, sort ast.ASTHandle
	functions        map[string]ast.ASTHandle
}

func smtNewLazyMAMFixture(t *testing.T, unary bool, eager, lazy uint32) smtLazyMAMFixture {
	t.Helper()
	m, state, registry, manager := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{Ematching: true, MaxEagerMultipatterns: eager, MaxLazyMultipatternMatching: lazy})
	relevancy, err := NewSMTRelevancyPropagator(state.context, false)
	astTestMust(t, err)
	astTestMust(t, state.SetRelevancy(relevancy))
	sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "lazy-pattern-U"), nil)
	astTestMust(t, err)
	x, err := m.MkVar(0, sort)
	astTestMust(t, err)
	functions := make(map[string]ast.ASTHandle)
	terms := make(map[string]ast.ASTHandle)
	for _, name := range []string{"f", "g", "h", "k", "l"} {
		function, err := m.MkFuncDecl(astTestSymbol(t, m, "lazy-pattern-"+name), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
		astTestMust(t, err)
		functions[name] = function
		terms[name], err = m.MkApp(function, []ast.ASTHandle{x})
		astTestMust(t, err)
	}
	var patterns []ast.ASTHandle
	if unary {
		pattern, err := m.MkPattern([]ast.ASTHandle{terms["f"]})
		astTestMust(t, err)
		patterns = append(patterns, pattern)
	}
	for _, pair := range [][2]string{{"g", "h"}, {"k", "l"}} {
		pattern, err := m.MkPattern([]ast.ASTHandle{terms[pair[0]], terms[pair[1]]})
		astTestMust(t, err)
		patterns = append(patterns, pattern)
	}
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	p := smtQuickPredicate(t, m, "lazy-pattern-p", sort, boolean)
	body, err := m.MkApp(p, []ast.ASTHandle{x})
	astTestMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{astTestSymbol(t, m, "lazy-pattern-x")}, Body: body, Patterns: patterns})
	astTestMust(t, err)
	astTestMust(t, state.Internalize(q, true))
	astTestMust(t, state.context.SetRelevant(q, true))
	astTestMust(t, state.context.SetAssignment(q, lbool.True))
	astTestMust(t, manager.Propagate())
	return smtLazyMAMFixture{m: m, state: state, registry: registry, manager: manager, quantifier: q, sort: sort, functions: functions}
}

func (f smtLazyMAMFixture) ground(t *testing.T, names ...string) {
	f.groundOn(t, "lazy-pattern-c", names...)
}

func (f smtLazyMAMFixture) groundOn(t *testing.T, name string, names ...string) {
	t.Helper()
	c, err := f.m.MkConst(astTestSymbol(t, f.m, name), f.sort)
	astTestMust(t, err)
	for _, name := range names {
		term, err := f.m.MkApp(f.functions[name], []ast.ASTHandle{c})
		astTestMust(t, err)
		astTestMust(t, f.state.Internalize(term, false))
	}
}

func TestSMTQuantifierPartitionsEagerAndLazyMultipatterns(t *testing.T) {
	for _, unary := range []bool{false, true} {
		for _, eager := range []uint32{0, 1, 2} {
			t.Run(fmt.Sprint(unary, "/", eager), func(t *testing.T) {
				f := smtNewLazyMAMFixture(t, unary, eager, 2)
				wantEager := int(eager)
				if !unary {
					wantEager++
				}
				if wantEager > 2 {
					wantEager = 2
				}
				wantLazy := 2 - wantEager
				if unary {
					wantEager++
				}
				if len(f.manager.mam.patterns) != wantEager || len(f.manager.lazyMAM.patterns) != wantLazy {
					t.Fatal("source unary/multipattern partition", len(f.manager.mam.patterns), len(f.manager.lazyMAM.patterns), wantEager, wantLazy)
				}
			})
		}
	}
}

func TestSMTLazyMultipatternsWaitForBoundedFinalCheckAndNewSearch(t *testing.T) {
	f := smtNewLazyMAMFixture(t, true, 0, 2)
	f.ground(t, "g", "h")
	astTestMust(t, f.manager.Propagate())
	if f.registry.schedule.Stats().Instances != 0 || f.manager.CanPropagate() || !f.manager.lazyMAM.HasWork() {
		t.Fatal("ordinary propagation ran the deferred multipattern")
	}
	status, err := f.manager.FinalCheck(true)
	astTestMust(t, err)
	if status != SMTFinalCheckContinue || !f.registry.schedule.HasWork() || f.manager.lazyMatchingIndex != 1 {
		t.Fatal("lazy full final check did not publish its match", status, f.manager.lazyMatchingIndex)
	}
	astTestMust(t, f.manager.Propagate())
	if f.registry.schedule.Stats().Instances != 1 {
		t.Fatal("deferred instance was not consumed")
	}
	_, err = f.manager.FinalCheck(true)
	astTestMust(t, err)
	if f.manager.lazyMatchingIndex != 2 {
		t.Fatal("second rematch did not consume the source allowance")
	}
	f.groundOn(t, "lazy-pattern-d", "k", "l")
	astTestMust(t, f.manager.Propagate())
	_, err = f.manager.FinalCheck(true)
	astTestMust(t, err)
	if f.registry.schedule.HasWork() || f.manager.lazyMatchingIndex != 2 || f.registry.schedule.Stats().Instances != 1 {
		t.Fatal("exhausted lazy matcher ran again")
	}
	astTestMust(t, f.manager.InitSearch())
	status, err = f.manager.FinalCheck(true)
	astTestMust(t, err)
	if status != SMTFinalCheckContinue || !f.registry.schedule.HasWork() || f.manager.lazyMatchingIndex != 1 {
		t.Fatal("new search did not renew lazy rematching", status, f.manager.lazyMatchingIndex)
	}
}

func TestSMTLazyMatchingCounterTrailsIncrementsAcrossSearchReset(t *testing.T) {
	f := smtNewLazyMAMFixture(t, true, 0, 2)
	astTestMust(t, f.manager.matchLazyPatterns())
	if f.manager.lazyMatchingIndex != 1 {
		t.Fatal("base lazy rematch")
	}
	astTestMust(t, f.state.PushScope())
	astTestMust(t, f.manager.InitSearch())
	astTestMust(t, f.manager.matchLazyPatterns())
	astTestMust(t, f.manager.matchLazyPatterns())
	if f.manager.lazyMatchingIndex != 2 {
		t.Fatal("child search did not use its renewed allowance")
	}
	astTestMust(t, f.state.PopScopes(1))
	if f.manager.lazyMatchingIndex != 0 {
		t.Fatal("pop restored a pre-search snapshot instead of undoing actual counter increments", f.manager.lazyMatchingIndex)
	}
}

func TestSMTLazyInstantiationControlsNonFullPreflight(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		f := smtNewLazyMAMFixture(t, true, 0, 2)
		f.ground(t, "g", "h")
		astTestMust(t, f.manager.Propagate())
		f.manager.lazyInstantiation = enabled
		status, err := f.manager.FinalCheck(false)
		astTestMust(t, err)
		if status != SMTFinalCheckDone || f.registry.schedule.HasWork() != enabled || (f.manager.lazyMatchingIndex != 0) != enabled {
			t.Fatal("source lazy-instantiation preflight", enabled, status, f.manager.lazyMatchingIndex)
		}
	}
}

func TestSMTLazyPatternsRestoreScopesAndContributeSharedGroundTerms(t *testing.T) {
	f := smtNewLazyMAMFixture(t, true, 0, 2)
	astTestMust(t, f.state.PushScope())
	anchor, err := f.m.MkConst(astTestSymbol(t, f.m, "lazy-pattern-anchor"), f.sort)
	astTestMust(t, err)
	x, err := f.m.MkVar(0, f.sort)
	astTestMust(t, err)
	g, err := f.m.MkFuncDecl(astTestSymbol(t, f.m, "lazy-pattern-ground-g"), []ast.ASTHandle{f.sort, f.sort}, f.sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	gx, err := f.m.MkApp(g, []ast.ASTHandle{x, anchor})
	astTestMust(t, err)
	hx, err := f.m.MkApp(f.functions["h"], []ast.ASTHandle{x})
	astTestMust(t, err)
	multi, err := f.m.MkPattern([]ast.ASTHandle{gx, hx})
	astTestMust(t, err)
	unary, err := f.m.GetPattern(f.quantifier, 0)
	astTestMust(t, err)
	body, err := f.m.GetQuantifierBody(f.quantifier)
	astTestMust(t, err)
	q, err := f.m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.sort}, Names: []symbol.Symbol{astTestSymbol(t, f.m, "lazy-pattern-child-x")}, Body: body, Patterns: []ast.ASTHandle{unary, multi}})
	astTestMust(t, err)
	astTestMust(t, f.state.Internalize(q, true))
	astTestMust(t, f.state.context.SetAssignment(q, lbool.True))
	astTestMust(t, f.manager.Propagate())
	node := f.state.context.enodes.get(anchor)
	if node == nil || f.manager.mam.IsShared(node) || !f.manager.lazyMAM.IsShared(node) || !f.manager.IsShared(node) {
		t.Fatal("deferred pattern ground term was omitted from context sharing")
	}
	if len(f.manager.mam.patterns) != 2 || len(f.manager.lazyMAM.patterns) != 3 {
		t.Fatal("child pattern partition")
	}
	astTestMust(t, f.state.PopScopes(1))
	if len(f.manager.mam.patterns) != 1 || len(f.manager.lazyMAM.patterns) != 2 || f.manager.IsShared(node) || f.manager.activated[q] {
		t.Fatal("child lazy patterns/sharing/activation escaped pop")
	}
}

func TestSMTKernelUpdatesLiveMultipatternControls(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	s, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	q := s.kernel.internalizer.quantifierManager
	if q.maxEagerMultipatterns != 0 || q.maxLazyMultipatternMatching != 2 || q.lazyInstantiation {
		t.Fatal("source multipattern defaults did not reach the context")
	}
	s.kernel.smtParams.qi.state.MaxLazyMultipatternMatching = 0
	s.kernel.smtParams.qi.state.LazyInstantiation = true
	params, err := cfg.Context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
	key := "qi.max_multi_patterns"
	astTestMust(t, cfg.Context.SetUIntName(&params, &key, 1))
	astTestMust(t, s.UpdateParams(&params))
	if q.maxEagerMultipatterns != 1 || q.maxLazyMultipatternMatching != 0 || !q.lazyInstantiation {
		t.Fatal("live parameter-owner controls were incorrectly constructor-cached", q.maxEagerMultipatterns, q.maxLazyMultipatternMatching, q.lazyInstantiation)
	}
}
