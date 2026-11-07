package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func smtQuickTheoryRewriter(t *testing.T, m *ast.ASTManager) *rewriter.TheoryRewriter {
	t.Helper()
	context, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	catalog, err := uparams.NewGlobalParamsCatalog(m.Symbols, uparams.GlobalParamsCatalogConfig{Budget: m.Budget()})
	astTestMust(t, err)
	globals, err := uparams.NewGlobalParamsService(m.Symbols, context, uparams.GlobalParamsConfig{Budget: m.Budget(), Catalog: catalog})
	astTestMust(t, err)
	astTestMust(t, globals.InitializeSource())
	rw, err := rewriter.NewTheoryRewriter(m, context, globals, nil)
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, rw.Close())
		astTestMust(t, globals.Close())
		astTestMust(t, catalog.Close())
		astTestMust(t, context.Close())
	})
	return rw
}

func smtQuickRegistry(t *testing.T, m *ast.ASTManager) (*SMTQuantifierRegistry, *SMTQIQueueSchedule) {
	t.Helper()
	costs, err := NewSMTQICostModel(m, "0", "0", nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, costs.Close()) })
	schedule := NewSMTQIQueueSchedule(m, 1, 1, false)
	registry, err := NewSMTQuantifierRegistry(m, schedule, costs, 100)
	astTestMust(t, err)
	return registry, schedule
}

func smtQuickPredicate(t *testing.T, m *ast.ASTManager, name string, domain, boolean ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	info, err := m.NewFuncDeclInfo(ast.NullFamilyID, -1, nil, ast.ASTFuncFlags{}, false)
	astTestMust(t, err)
	declaration, err := m.MkFuncDeclWithInfo(astTestSymbol(t, m, name), []ast.ASTHandle{domain}, boolean, info)
	astTestMust(t, err)
	astTestMust(t, m.DelFuncDeclInfo(&info))
	return declaration
}

func smtQuickApplicationNode(t *testing.T, m *ast.ASTManager, context *SMTCheckerContext, declaration ast.ASTHandle, child *SMTEnode, relevant bool) (ast.ASTHandle, *SMTEnode) {
	t.Helper()
	expression, err := m.MkApp(declaration, []ast.ASTHandle{child.owner})
	astTestMust(t, err)
	node, err := NewSMTEnode(m, expression, smtEnodeMapLookup(map[ast.ASTHandle]*SMTEnode{child.owner: child}), SMTEnodeOptions{CongruenceEnabled: true})
	astTestMust(t, err)
	node.congruence = node
	astTestMust(t, context.RegisterEnode(node, relevant))
	return expression, node
}

func TestSMTQuickCheckerCollectorConservativeIntersectionAndUnion(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkSort(astTestSymbol(t, m, "QuickCollectSort"), ast.ASTSortInfo{})
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	p := smtQuickPredicate(t, m, "quick-collect-p", sort, boolean)
	qdecl := smtQuickPredicate(t, m, "quick-collect-q", sort, boolean)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	constants := make([]*SMTEnode, 3)
	for index := range constants {
		_, constants[index] = smtEnodeConstant(t, m, 2200+uint32(index), sort, 0)
		astTestMust(t, context.RegisterEnode(constants[index], true))
	}
	for _, index := range []int{0, 1} {
		smtQuickApplicationNode(t, m, context, p, constants[index], true)
	}
	for _, index := range []int{1, 2} {
		smtQuickApplicationNode(t, m, context, qdecl, constants[index], true)
	}
	variable, err := m.MkVar(0, sort)
	astTestMust(t, err)
	pv, err := m.MkApp(p, []ast.ASTHandle{variable})
	astTestMust(t, err)
	qv, err := m.MkApp(qdecl, []ast.ASTHandle{variable})
	astTestMust(t, err)
	body, err := m.MkAnd(pv, qv)
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{astTestSymbol(t, m, "quick-collect-x")}, Body: body})
	astTestMust(t, err)
	registry, _ := smtQuickRegistry(t, m)
	checker, err := NewSMTQuickChecker(context, smtQuickTheoryRewriter(t, m), registry)
	astTestMust(t, err)
	conservative, err := checker.CollectCandidates(quantifier, true)
	astTestMust(t, err)
	if len(conservative) != 1 || len(conservative[0]) != 1 || conservative[0][0] != constants[1] {
		t.Fatal("conservative repeated-variable intersection", conservative)
	}
	nonconservative, err := checker.CollectCandidates(quantifier, false)
	astTestMust(t, err)
	if len(nonconservative) != 1 || len(nonconservative[0]) != 3 {
		t.Fatal("non-conservative repeated-variable union", nonconservative)
	}
}

func TestSMTQuickCheckerUnsatInstantiationCanonicalizationAndDuplicates(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkSort(astTestSymbol(t, m, "QuickInstantiateSort"), ast.ASTSortInfo{})
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	predicate := smtQuickPredicate(t, m, "quick-instantiate-p", sort, boolean)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	constants := make([]*SMTEnode, 2)
	applications := make([]ast.ASTHandle, 2)
	for index := range constants {
		_, constants[index] = smtEnodeConstant(t, m, 2210+uint32(index), sort, uint32(index+2))
		astTestMust(t, context.RegisterEnode(constants[index], true))
		applications[index], _ = smtQuickApplicationNode(t, m, context, predicate, constants[index], true)
		astTestMust(t, context.SetAssignment(applications[index], lbool.False))
	}
	variable, err := m.MkVar(0, sort)
	astTestMust(t, err)
	body, err := m.MkApp(predicate, []ast.ASTHandle{variable})
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{astTestSymbol(t, m, "quick-instantiate-x")}, Body: body})
	astTestMust(t, err)
	registry, schedule := smtQuickRegistry(t, m)
	astTestMust(t, registry.Add(quantifier, 0))
	checker, err := NewSMTQuickChecker(context, smtQuickTheoryRewriter(t, m), registry)
	astTestMust(t, err)
	generated, err := checker.InstantiateUnsat(quantifier)
	astTestMust(t, err)
	if !generated || len(schedule.newEntries) != 2 {
		t.Fatal("unsatisfied instances", generated, len(schedule.newEntries))
	}
	generationSeen := map[uint32]bool{}
	for _, entry := range schedule.newEntries {
		generationSeen[entry.Generation] = true
	}
	if !generationSeen[2] || !generationSeen[3] {
		t.Fatal("binding generations", schedule.newEntries)
	}
	generated, err = checker.InstantiateUnsat(quantifier)
	astTestMust(t, err)
	if generated || len(schedule.newEntries) != 2 {
		t.Fatal("duplicate fingerprints", generated, len(schedule.newEntries))
	}
	generated, err = checker.InstantiateNotSatCandidates(quantifier, []ast.ASTHandle{constants[0].owner})
	astTestMust(t, err)
	if generated {
		t.Fatal("explicit duplicate candidate")
	}
	thirdOwner, _ := smtEnodeConstant(t, m, 2212, sort, 4)
	generated, err = checker.InstantiateNotSatCandidates(quantifier, []ast.ASTHandle{thirdOwner})
	astTestMust(t, err)
	if !generated || context.enodes.get(thirdOwner) == nil || len(schedule.newEntries) != 3 {
		t.Fatal("explicit candidate internalization", generated, context.enodes.get(thirdOwner), len(schedule.newEntries))
	}
}
