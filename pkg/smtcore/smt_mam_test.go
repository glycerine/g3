package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTQuantifierMatchersHonorDisabledRelevancy(t *testing.T) {
	m, state, registry, mam := smtMAMFixture(t)
	relevancy, err := NewSMTRelevancyPropagator(state.context, true)
	astTestMust(t, err)
	astTestMust(t, state.SetRelevancy(relevancy))
	sort, err := m.MkSort(astTestSymbol(t, m, "matcher-relevancy-sort"), ast.ASTSortInfo{})
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	p := smtQuickPredicate(t, m, "matcher-relevancy-p", sort, boolean)
	q := smtQuickPredicate(t, m, "matcher-relevancy-q", sort, boolean)
	variable, err := m.MkVar(0, sort)
	astTestMust(t, err)
	pv, err := m.MkApp(p, []ast.ASTHandle{variable})
	astTestMust(t, err)
	qv, err := m.MkApp(q, []ast.ASTHandle{variable})
	astTestMust(t, err)
	body, err := m.MkAnd(pv, qv)
	astTestMust(t, err)
	pattern, err := m.MkPattern([]ast.ASTHandle{pv, qv})
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{astTestSymbol(t, m, "matcher-x")},
		Body: body, Patterns: []ast.ASTHandle{pattern},
	})
	astTestMust(t, err)
	astTestMust(t, registry.Add(quantifier, 0))
	constants := make([]*SMTEnode, 3)
	for index := range constants {
		_, constants[index] = smtEnodeConstant(t, m, 2630+uint32(index), sort, uint32(index+2))
		astTestMust(t, state.context.RegisterEnode(constants[index], false))
	}
	pa, _ := smtQuickApplicationNode(t, m, state.context, p, constants[0], false)
	smtQuickApplicationNode(t, m, state.context, p, constants[1], false)
	smtQuickApplicationNode(t, m, state.context, q, constants[1], false)
	smtQuickApplicationNode(t, m, state.context, q, constants[2], false)
	astTestMust(t, state.context.SetAssignment(pa, lbool.False))
	quick, err := NewSMTQuickChecker(state.context, smtQuickTheoryRewriter(t, m), registry)
	astTestMust(t, err)
	checker, err := NewSMTChecker(state.context)
	astTestMust(t, err)
	astTestMust(t, mam.AddPattern(quantifier, pattern))
	for _, enabled := range []bool{true, false, true} {
		relevancy.enabled = enabled
		candidates, err := quick.CollectCandidates(quantifier, true)
		astTestMust(t, err)
		if enabled {
			if len(candidates[0]) != 0 {
				t.Fatal("filtering admitted unmarked quick-check candidates", candidates)
			}
		} else {
			if len(candidates[0]) != 1 || candidates[0][0] != constants[1] {
				t.Fatal("disabled filtering lost the multi-predicate intersection", candidates)
			}
			union, err := quick.CollectCandidates(quantifier, false)
			astTestMust(t, err)
			if len(union[0]) != 3 {
				t.Fatal("disabled filtering lost nonconservative candidates", union)
			}
		}
		unsat, err := checker.IsUnsatisfiable(pa, nil)
		astTestMust(t, err)
		if unsat == enabled {
			t.Fatal("checker ignored the current relevancy policy", enabled, unsat)
		}
		quick.checkCache = make(map[smtQuickCheckKey]bool)
		unsat, err = quick.check(pa, false)
		astTestMust(t, err)
		if unsat == enabled {
			t.Fatal("quick checker ignored the current relevancy policy", enabled, unsat)
		}
		mam.dirty = true
		astTestMust(t, mam.Match())
		if enabled && len(registry.schedule.newEntries) != 0 || !enabled && len(registry.schedule.newEntries) != 1 {
			t.Fatal("E-matching did not obey the current relevancy policy", enabled, registry.schedule.newEntries)
		}
		if !enabled && registry.schedule.newEntries[0].Fingerprint.args[0] != constants[1] {
			t.Fatal("E-matching joined different bindings")
		}
		registry.schedule.newEntries = nil
	}
	if len(relevancy.RelevantExpressions()) != 0 {
		t.Fatal("reading relevancy policy changed the scoped relevance marks")
	}
}

func smtMAMFixture(t *testing.T) (*ast.ASTManager, *SMTInternalizerState, *SMTQuantifierRegistry, *SMTMAM) {
	t.Helper()
	m := astTestManager(t)
	costs, err := NewSMTQICostModel(m, "0", "0", nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, costs.Close()) })
	registry, err := NewSMTQuantifierRegistry(m, NewSMTQIQueueSchedule(m, 10, 20, false), costs, 100)
	astTestMust(t, err)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	state, err := NewSMTInternalizerState(m, context, registry)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, state.Close()) })
	mam, err := NewSMTMAM(state, registry)
	astTestMust(t, err)
	return m, state, registry, mam
}

func smtMAMQuantifier(t *testing.T, m *ast.ASTManager, name string, sort ast.ASTHandle, terms []ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	pattern, err := m.MkPattern(terms)
	astTestMust(t, err)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{astTestSymbol(t, m, name+"-x")}, Body: truth, Patterns: []ast.ASTHandle{pattern},
	})
	astTestMust(t, err)
	return quantifier, pattern
}

func TestSMTMAMMatchesUnaryAndRepeatedVariablesModuloEquality(t *testing.T) {
	m, state, registry, mam := smtMAMFixture(t)
	sort, err := m.MkSort(astTestSymbol(t, m, "mam-unary-sort"), ast.ASTSortInfo{})
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	predicateInfo, err := m.NewFuncDeclInfo(ast.NullFamilyID, -1, nil, ast.ASTFuncFlags{}, false)
	astTestMust(t, err)
	predicate, err := m.MkFuncDeclWithInfo(astTestSymbol(t, m, "mam-repeated"), []ast.ASTHandle{sort, sort}, boolean, predicateInfo)
	astTestMust(t, err)
	astTestMust(t, m.DelFuncDeclInfo(&predicateInfo))
	variable, err := m.MkVar(0, sort)
	astTestMust(t, err)
	trigger, err := m.MkApp(predicate, []ast.ASTHandle{variable, variable})
	astTestMust(t, err)
	quantifier, pattern := smtMAMQuantifier(t, m, "mam-repeated", sort, []ast.ASTHandle{trigger})
	astTestMust(t, registry.Add(quantifier, 0))
	_, a := smtEnodeConstant(t, m, 2600, sort, 2)
	_, b := smtEnodeConstant(t, m, 2601, sort, 3)
	astTestMust(t, state.context.RegisterEnode(a, true))
	astTestMust(t, state.context.RegisterEnode(b, true))
	application, err := m.MkApp(predicate, []ast.ASTHandle{a.owner, b.owner})
	astTestMust(t, err)
	node, err := NewSMTEnode(m, application, state.context.enodes.get, SMTEnodeOptions{Generation: 4, CongruenceEnabled: true})
	astTestMust(t, err)
	node.congruence = node
	astTestMust(t, state.context.RegisterEnode(node, true))
	astTestMust(t, mam.AddPattern(quantifier, pattern))
	astTestMust(t, mam.Match())
	if len(registry.schedule.newEntries) != 0 {
		t.Fatal("repeated variable matched distinct equality classes")
	}
	astTestMust(t, state.context.SetRelevant(application, false))
	application, err = m.MkApp(predicate, []ast.ASTHandle{a.owner, a.owner})
	astTestMust(t, err)
	node, err = NewSMTEnode(m, application, state.context.enodes.get, SMTEnodeOptions{Generation: 5, CongruenceEnabled: true})
	astTestMust(t, err)
	node.congruence = node
	astTestMust(t, state.context.RegisterEnode(node, true))
	mam.Relevant(node)
	astTestMust(t, mam.Match())
	if len(registry.schedule.newEntries) != 1 || registry.schedule.newEntries[0].Fingerprint.args[0].root != a.root {
		t.Fatal("repeated-variable trigger binding", registry.schedule.newEntries)
	}
}

func TestSMTMAMJoinsMultiPatternsAndRollsBackPatterns(t *testing.T) {
	m, state, registry, mam := smtMAMFixture(t)
	sort, err := m.MkSort(astTestSymbol(t, m, "mam-multi-sort"), ast.ASTSortInfo{})
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	p := smtQuickPredicate(t, m, "mam-multi-p", sort, boolean)
	q := smtQuickPredicate(t, m, "mam-multi-q", sort, boolean)
	variable, err := m.MkVar(0, sort)
	astTestMust(t, err)
	pv, err := m.MkApp(p, []ast.ASTHandle{variable})
	astTestMust(t, err)
	qv, err := m.MkApp(q, []ast.ASTHandle{variable})
	astTestMust(t, err)
	quantifier, pattern := smtMAMQuantifier(t, m, "mam-multi", sort, []ast.ASTHandle{pv, qv})
	astTestMust(t, registry.Add(quantifier, 0))
	_, a := smtEnodeConstant(t, m, 2610, sort, 1)
	_, b := smtEnodeConstant(t, m, 2611, sort, 2)
	astTestMust(t, state.context.RegisterEnode(a, true))
	astTestMust(t, state.context.RegisterEnode(b, true))
	smtQuickApplicationNode(t, m, state.context, p, a, true)
	smtQuickApplicationNode(t, m, state.context, p, b, true)
	smtQuickApplicationNode(t, m, state.context, q, b, true)
	astTestMust(t, mam.PushScope())
	astTestMust(t, mam.AddPattern(quantifier, pattern))
	astTestMust(t, mam.Match())
	if len(registry.schedule.newEntries) != 1 || registry.schedule.newEntries[0].Fingerprint.args[0].root != b.root {
		t.Fatal("multi-pattern join", registry.schedule.newEntries)
	}
	astTestMust(t, mam.PopScopes(1))
	registry.schedule.newEntries = nil
	mam.Relevant(b)
	astTestMust(t, mam.Match())
	if len(registry.schedule.newEntries) != 0 {
		t.Fatal("popped pattern remained active")
	}
}

func TestSMTMAMTracksNestedGroundPatternTermsAsShared(t *testing.T) {
	m, state, registry, mam := smtMAMFixture(t)
	sort, err := m.MkSort(astTestSymbol(t, m, "mam-shared-sort"), ast.ASTSortInfo{})
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	info, err := m.NewFuncDeclInfo(ast.NullFamilyID, -1, nil, ast.ASTFuncFlags{}, false)
	astTestMust(t, err)
	predicate, err := m.MkFuncDeclWithInfo(astTestSymbol(t, m, "mam-shared-p"), []ast.ASTHandle{sort, sort}, boolean, info)
	astTestMust(t, err)
	astTestMust(t, m.DelFuncDeclInfo(&info))
	constant, err := m.MkConst(astTestSymbol(t, m, "mam-shared-c"), sort)
	astTestMust(t, err)
	variable, err := m.MkVar(0, sort)
	astTestMust(t, err)
	trigger, err := m.MkApp(predicate, []ast.ASTHandle{variable, constant})
	astTestMust(t, err)
	quantifier, pattern := smtMAMQuantifier(t, m, "mam-shared", sort, []ast.ASTHandle{trigger})
	astTestMust(t, registry.Add(quantifier, 0))
	astTestMust(t, mam.PushScope())
	astTestMust(t, mam.AddPattern(quantifier, pattern))
	node := state.context.enodes.get(constant)
	if node == nil || !mam.IsShared(node) {
		t.Fatal("nested ground pattern term was not internalized and shared")
	}
	astTestMust(t, mam.PopScopes(1))
	if mam.IsShared(node) {
		t.Fatal("shared pattern term survived pattern rollback")
	}
}
