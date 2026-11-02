package pdr_test

// Source: src/muz/pdr/pdr_context.cpp context::init_rules allocation,
// transformer initialization, and level-zero query setup.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/messages"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func TestContextInitializesHornProgramAndAnswersInitialQueries(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	pdrMust(t, err)
	manager, err := command.Manager()
	pdrMust(t, err)
	globals, err := command.GlobalParameters()
	pdrMust(t, err)
	context, err := pdr.NewContext(manager, nil, 4, pdr.ReachableConstraintCache, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	pdrMust(t, err)
	boolean, err := manager.MkBoolSort()
	pdrMust(t, err)
	pDeclaration := pdrFunction(t, manager, "context_p", []ast.ASTHandle{boolean}, boolean)
	qDeclaration := pdrFunction(t, manager, "context_q", []ast.ASTHandle{boolean}, boolean)
	ruleManager, err := base.NewRuleManager(manager)
	pdrMust(t, err)
	pdrMust(t, ruleManager.RegisterPredicate(pDeclaration))
	pdrMust(t, ruleManager.RegisterPredicate(qDeclaration))
	set, err := base.NewRuleSet(ruleManager)
	pdrMust(t, err)
	variable, err := manager.MkVar(0, boolean)
	pdrMust(t, err)
	truth, err := manager.MkTrue()
	pdrMust(t, err)
	falsity, err := manager.MkFalse()
	pdrMust(t, err)
	pHead, err := manager.MkApp(pDeclaration, []ast.ASTHandle{variable})
	pdrMust(t, err)
	constraint, err := manager.MkEq(variable, truth)
	pdrMust(t, err)
	pRule, err := ruleManager.NewRule(pHead, []ast.ASTHandle{constraint}, nil, "p-initial")
	pdrMust(t, err)
	pdrMust(t, set.Add(pRule))
	pdrMust(t, pRule.Close())
	qHead, err := manager.MkApp(qDeclaration, []ast.ASTHandle{variable})
	pdrMust(t, err)
	pTail, err := manager.MkApp(pDeclaration, []ast.ASTHandle{variable})
	pdrMust(t, err)
	qRule, err := ruleManager.NewRule(qHead, []ast.ASTHandle{pTail}, nil, "q-step")
	pdrMust(t, err)
	pdrMust(t, set.Add(qRule))
	pdrMust(t, qRule.Close())
	pdrMust(t, context.Initialize(set))

	for _, check := range []struct {
		declaration ast.ASTHandle
		argument    ast.ASTHandle
		want        lbool.LBool
	}{
		{pDeclaration, truth, lbool.True},
		{pDeclaration, falsity, lbool.False},
		{qDeclaration, truth, lbool.False},
	} {
		query, x := manager.MkApp(check.declaration, []ast.ASTHandle{check.argument})
		pdrMust(t, x)
		result, x := context.CheckInitial(query, check.want == lbool.True)
		pdrMust(t, x)
		if result.Status != check.want {
			t.Fatal("initial query", check.declaration, check.argument, result.Status, check.want)
		}
		if result.Model != nil {
			pdrMust(t, result.Model.DecRef())
		}
	}
	if transformer, found, x := context.Transformer(qDeclaration); x != nil || !found || transformer == nil {
		t.Fatal("query transformer lookup", transformer, found, x)
	}
	coverProperty, err := manager.MkEq(variable, truth)
	pdrMust(t, err)
	pdrMust(t, context.AddCover(-1, qDeclaration, coverProperty))
	cover, err := context.CoverDelta(-1, qDeclaration)
	pdrMust(t, err)
	coverHandle, err := manager.RefGet(cover)
	pdrMust(t, err)
	if coverHandle != coverProperty {
		t.Fatal("invariant cover round trip", coverHandle, coverProperty)
	}
	pdrMust(t, manager.DelRef(&cover))
	finiteProperty, err := manager.MkEq(variable, falsity)
	pdrMust(t, err)
	pdrMust(t, context.AddCover(0, qDeclaration, finiteProperty))
	levels, err := context.NumLevels(qDeclaration)
	pdrMust(t, err)
	if levels == 0 {
		t.Fatal("finite cover did not create a frame")
	}
	cover, err = context.CoverDelta(0, qDeclaration)
	pdrMust(t, err)
	coverHandle, err = manager.RefGet(cover)
	pdrMust(t, err)
	conjuncts, err := manager.FlattenAnd([]ast.ASTHandle{coverHandle})
	pdrMust(t, err)
	negatedVariable, err := manager.MkNot(variable)
	pdrMust(t, err)
	foundProperty := false
	for _, conjunct := range conjuncts {
		foundProperty = foundProperty || conjunct == finiteProperty || conjunct == negatedVariable
	}
	if !foundProperty {
		t.Fatal("finite cover omitted added property", coverHandle, finiteProperty)
	}
	pdrMust(t, manager.DelRef(&cover))
	if err = context.AddCover(-2, qDeclaration, coverProperty); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("invalid cover level", err)
	}
	rDeclaration := pdrFunction(t, manager, "context_r", []ast.ASTHandle{boolean}, boolean)
	pdrMust(t, context.AddCover(-1, rDeclaration, coverProperty))
	if transformer, found, x := context.Transformer(rDeclaration); x != nil || !found || transformer == nil {
		t.Fatal("cover-created transformer", transformer, found, x)
	}

	pdrMust(t, context.Close())
	pdrMust(t, set.Close())
	pdrMust(t, ruleManager.Close())
	pdrMust(t, command.Close())
}

func TestContextBoundedQueryTraversesTaggedHornChain(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	pdrMust(t, err)
	manager, err := command.Manager()
	pdrMust(t, err)
	globals, err := command.GlobalParameters()
	pdrMust(t, err)
	context, err := pdr.NewContext(manager, nil, 4, pdr.ReachableHashCache, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	pdrMust(t, err)
	pdrMust(t, context.SetGenerateProofTrace(true))
	boolean, err := manager.MkBoolSort()
	pdrMust(t, err)
	declarations := []ast.ASTHandle{
		pdrFunction(t, manager, "bounded_p", []ast.ASTHandle{boolean}, boolean),
		pdrFunction(t, manager, "bounded_q", []ast.ASTHandle{boolean}, boolean),
		pdrFunction(t, manager, "bounded_r", []ast.ASTHandle{boolean}, boolean),
	}
	ruleManager, err := base.NewRuleManager(manager)
	pdrMust(t, err)
	for _, declaration := range declarations {
		pdrMust(t, ruleManager.RegisterPredicate(declaration))
	}
	set, err := base.NewRuleSet(ruleManager)
	pdrMust(t, err)
	variable, err := manager.MkVar(0, boolean)
	pdrMust(t, err)
	truth, err := manager.MkTrue()
	pdrMust(t, err)
	falsity, err := manager.MkFalse()
	pdrMust(t, err)
	for index, declaration := range declarations {
		head, x := manager.MkApp(declaration, []ast.ASTHandle{variable})
		pdrMust(t, x)
		var tail ast.ASTHandle
		if index == 0 {
			tail, x = manager.MkEq(variable, truth)
		} else {
			tail, x = manager.MkApp(declarations[index-1], []ast.ASTHandle{variable})
		}
		pdrMust(t, x)
		rule, x := ruleManager.NewRule(head, []ast.ASTHandle{tail}, nil, "bounded")
		pdrMust(t, x)
		pdrMust(t, set.Add(rule))
		pdrMust(t, rule.Close())
	}
	selfHead, err := manager.MkApp(declarations[2], []ast.ASTHandle{variable})
	pdrMust(t, err)
	selfTail, err := manager.MkApp(declarations[2], []ast.ASTHandle{variable})
	pdrMust(t, err)
	selfRule, err := ruleManager.NewRule(selfHead, []ast.ASTHandle{selfTail}, nil, "bounded-self")
	pdrMust(t, err)
	pdrMust(t, set.Add(selfRule))
	pdrMust(t, selfRule.Close())
	pdrMust(t, context.Initialize(set))

	for _, check := range []struct {
		predicate int
		argument  ast.ASTHandle
		depth     uint32
		want      lbool.LBool
	}{
		{0, truth, 0, lbool.True},
		{1, truth, 0, lbool.False},
		{1, truth, 1, lbool.True},
		{2, truth, 1, lbool.False},
		{2, truth, 2, lbool.True},
		{2, falsity, 3, lbool.False},
	} {
		query, x := manager.MkApp(declarations[check.predicate], []ast.ASTHandle{check.argument})
		pdrMust(t, x)
		result, x := context.CheckBounded(query, check.depth, check.want == lbool.True)
		pdrMust(t, x)
		if result.Status != check.want {
			t.Fatal("bounded query", check.predicate, check.depth, result.Status, check.want)
		}
		if result.Model != nil {
			pdrMust(t, result.Model.DecRef())
		}
	}
	status, err := context.Solve(declarations[2])
	pdrMust(t, err)
	if status != lbool.True {
		t.Fatal("unbounded Horn chain query", status)
	}
	proof, err := context.Proof(declarations[2])
	pdrMust(t, err)
	proofHandle, err := manager.RefGet(proof)
	pdrMust(t, err)
	hyper, err := manager.IsProofKind(proofHandle, ast.ASTPrHyperResolve)
	pdrMust(t, err)
	parents, err := manager.GetProofNumParents(proofHandle)
	pdrMust(t, err)
	if !hyper || parents != 2 {
		t.Fatal("reduced Horn-chain proof", hyper, parents)
	}
	pdrMust(t, manager.DelRef(&proof))
	proofBytes := manager.Budget().Used()
	proof, err = context.Proof(declarations[2])
	pdrMust(t, err)
	pdrMust(t, manager.DelRef(&proof))
	if manager.Budget().Used() != proofBytes {
		t.Fatal("repeated proof reconstruction retained owner bytes", proofBytes, manager.Budget().Used())
	}
	stats := statistics.New()
	pdrMust(t, context.CollectStatistics(stats))
	foundNodes, foundDepth := false, false
	var nodeCount, depthCount uint32
	for index := 0; index < stats.Size(); index++ {
		switch stats.Key(index) {
		case "PDR num unfoldings":
			foundNodes = stats.IsUint(index) && stats.UintValue(index) >= 2
			nodeCount = stats.UintValue(index)
		case "PDR max depth":
			foundDepth = stats.IsUint(index) && stats.UintValue(index) >= 2
			depthCount = stats.UintValue(index)
		}
	}
	if !foundNodes || !foundDepth {
		t.Fatal("PDR search statistics", foundNodes, foundDepth)
	}
	pdrMust(t, context.UpdateRules(set))
	updated := statistics.New()
	pdrMust(t, context.CollectStatistics(updated))
	for _, counter := range []struct {
		key  statistics.Key
		want uint32
	}{
		{"PDR num unfoldings", nodeCount}, {"PDR max depth", depthCount},
	} {
		var got uint32
		for index := 0; index < updated.Size(); index++ {
			if updated.IsUint(index) && updated.Key(index) == counter.key {
				got += updated.UintValue(index)
			}
		}
		if got != counter.want {
			t.Fatal("rule update cleared source cumulative search statistics", counter.key, got, counter.want)
		}
	}
	pdrMust(t, context.ResetStatistics())
	stats.Reset()
	pdrMust(t, context.CollectStatistics(stats))
	for index := 0; index < stats.Size(); index++ {
		if stats.Key(index) == "PDR num unfoldings" || stats.Key(index) == "PDR max depth" || stats.Key(index) == "PDR inductive level" {
			t.Fatal("reset retained PDR search statistic", stats.Key(index))
		}
	}

	pdrMust(t, context.Close())
	pdrMust(t, set.Close())
	pdrMust(t, ruleManager.Close())
	pdrMust(t, command.Close())
}

func TestContextSolveFindsInductiveUnreachablePredicate(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	pdrMust(t, err)
	manager, err := command.Manager()
	pdrMust(t, err)
	globals, err := command.GlobalParameters()
	pdrMust(t, err)
	context, err := pdr.NewContext(manager, nil, 3, pdr.ReachableHashCache, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	pdrMust(t, err)
	boolean, err := manager.MkBoolSort()
	pdrMust(t, err)
	predicate := pdrFunction(t, manager, "unreachable", []ast.ASTHandle{boolean}, boolean)
	ruleManager, err := base.NewRuleManager(manager)
	pdrMust(t, err)
	pdrMust(t, ruleManager.RegisterPredicate(predicate))
	set, err := base.NewRuleSet(ruleManager)
	pdrMust(t, err)
	variable, err := manager.MkVar(0, boolean)
	pdrMust(t, err)
	head, err := manager.MkApp(predicate, []ast.ASTHandle{variable})
	pdrMust(t, err)
	tail, err := manager.MkApp(predicate, []ast.ASTHandle{variable})
	pdrMust(t, err)
	rule, err := ruleManager.NewRule(head, []ast.ASTHandle{tail}, nil, "self")
	pdrMust(t, err)
	pdrMust(t, set.Add(rule))
	pdrMust(t, rule.Close())
	pdrMust(t, context.Initialize(set))
	status, err := context.Solve(predicate)
	pdrMust(t, err)
	if status != lbool.False {
		t.Fatal("self-recursive predicate without facts", status)
	}
	if last, x := context.LastResult(); x != nil || last != status {
		t.Fatal("last result", last, x)
	}
	certificate, err := context.Answer(predicate)
	pdrMust(t, err)
	certificateHandle, err := manager.RefGet(certificate)
	pdrMust(t, err)
	if !manager.IsTrue(certificateHandle) {
		t.Fatal("single-query-relation certificate is not the empty conjunction")
	}
	pdrMust(t, manager.DelRef(&certificate))
	inductiveLevel := func() uint32 {
		t.Helper()
		stats := statistics.New()
		pdrMust(t, context.CollectStatistics(stats))
		for index := 0; index < stats.Size(); index++ {
			if stats.IsUint(index) && stats.Key(index) == "PDR inductive level" {
				return stats.UintValue(index)
			}
		}
		return 0
	}
	level := inductiveLevel()
	if level == 0 {
		t.Fatal("fixture did not establish an inductive frame")
	}
	pdrMust(t, context.ResetStatistics())
	if got := inductiveLevel(); got != level {
		t.Fatal("statistics reset changed the inductive result frame", got, level)
	}
	before := manager.ResourceLimit().Count()
	manager.ResourceLimit().Cancel()
	status, err = context.Solve(predicate)
	manager.ResourceLimit().ResetCancel()
	if status != lbool.Undef || err == nil || err.Error() != messages.Canceled || manager.ResourceLimit().Count() != before+1 {
		t.Fatal("source solve checkpoint", status, err, manager.ResourceLimit().Count()-before)
	}

	pdrMust(t, context.Close())
	pdrMust(t, set.Close())
	pdrMust(t, ruleManager.Close())
	pdrMust(t, command.Close())
}

func TestContextActivatesFarkasBeforeArithmeticSolvers(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	pdrMust(t, err)
	manager, err := command.Manager()
	pdrMust(t, err)
	globals, err := command.GlobalParameters()
	pdrMust(t, err)
	context, err := pdr.NewContext(manager, nil, 2, pdr.ReachableHashCache, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	pdrMust(t, err)
	pdrMust(t, context.SetUseFarkas(true))
	breadthFirst, err := context.BreadthFirstModelSearch()
	pdrMust(t, err)
	if !breadthFirst {
		t.Fatal("PDR context did not default to breadth-first model search")
	}
	pdrMust(t, context.SetBreadthFirstModelSearch(false))
	pdrMust(t, context.SetFlexibleTrace(true))
	pdrMust(t, context.SetUseModelGeneralizer(true))
	pdrMust(t, context.SetUseMultiCoreGeneralizer(true))
	pdrMust(t, context.SetUseArithmeticInductiveGeneralizer(true))
	pdrMust(t, context.SetUseInductiveReachabilityGeneralizer(true))
	pdrMust(t, context.SetUseConvexClosureGeneralizer(true))
	pdrMust(t, context.SetUseConvexInteriorGeneralizer(true))
	arithmetic, err := manager.RegisterArithmeticPlugin()
	pdrMust(t, err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	pdrMust(t, err)
	boolean, err := manager.MkBoolSort()
	pdrMust(t, err)
	predicate := pdrFunction(t, manager, "farkas_context", []ast.ASTHandle{integer}, boolean)
	ruleManager, err := base.NewRuleManager(manager)
	pdrMust(t, err)
	pdrMust(t, ruleManager.RegisterPredicate(predicate))
	set, err := base.NewRuleSet(ruleManager)
	pdrMust(t, err)
	variable, err := manager.MkVar(0, integer)
	pdrMust(t, err)
	head, err := manager.MkApp(predicate, []ast.ASTHandle{variable})
	pdrMust(t, err)
	tail, err := manager.MkApp(predicate, []ast.ASTHandle{variable})
	pdrMust(t, err)
	rule, err := ruleManager.NewRule(head, []ast.ASTHandle{tail}, nil, "farkas-self")
	pdrMust(t, err)
	pdrMust(t, set.Add(rule))
	pdrMust(t, rule.Close())
	pdrMust(t, context.Initialize(set))
	active, err := context.FarkasActive()
	pdrMust(t, err)
	proofs, err := manager.ProofsEnabled()
	pdrMust(t, err)
	if !active || proofs {
		t.Fatal("arithmetic PDR did not scope Farkas proof mode", active, proofs)
	}
	if err = context.SetUseFarkas(false); err == nil {
		t.Fatal("initialized context accepted a Farkas mode change")
	}
	if err = context.SetBreadthFirstModelSearch(true); err == nil {
		t.Fatal("initialized context accepted a model-search strategy change")
	}
	if err = context.SetFlexibleTrace(false); err == nil {
		t.Fatal("initialized context accepted a flexible-trace change")
	}
	if err = context.SetUseModelGeneralizer(false); err == nil {
		t.Fatal("initialized context accepted a model-generalizer change")
	}
	if err = context.SetUseMultiCoreGeneralizer(false); err == nil {
		t.Fatal("initialized context accepted a multi-core-generalizer change")
	}
	if err = context.SetUseArithmeticInductiveGeneralizer(false); err == nil {
		t.Fatal("initialized context accepted an arithmetic-inductive-generalizer change")
	}
	if err = context.SetUseInductiveReachabilityGeneralizer(false); err == nil {
		t.Fatal("initialized context accepted an inductive-reachability-generalizer change")
	}
	if err = context.SetUseConvexClosureGeneralizer(false); err == nil {
		t.Fatal("initialized context accepted a convex-closure generalizer change")
	}
	if err = context.SetUseConvexInteriorGeneralizer(false); err == nil {
		t.Fatal("initialized context accepted a convex-interior generalizer change")
	}
	pdrMust(t, context.Close())
	pdrMust(t, set.Close())
	pdrMust(t, ruleManager.Close())
	pdrMust(t, command.Close())
}

func TestContextSolveProjectsCoupledPredecessors(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	pdrMust(t, err)
	manager, err := command.Manager()
	pdrMust(t, err)
	globals, err := command.GlobalParameters()
	pdrMust(t, err)
	context, err := pdr.NewContext(manager, nil, 4, pdr.ReachableHashCache, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	pdrMust(t, err)
	boolean, err := manager.MkBoolSort()
	pdrMust(t, err)
	predicateP := pdrFunction(t, manager, "project_p", []ast.ASTHandle{boolean}, boolean)
	predicateR := pdrFunction(t, manager, "project_r", []ast.ASTHandle{boolean}, boolean)
	predicateQ := pdrFunction(t, manager, "project_q", []ast.ASTHandle{boolean}, boolean)
	ruleManager, err := base.NewRuleManager(manager)
	pdrMust(t, err)
	for _, declaration := range []ast.ASTHandle{predicateP, predicateR, predicateQ} {
		pdrMust(t, ruleManager.RegisterPredicate(declaration))
	}
	set, err := base.NewRuleSet(ruleManager)
	pdrMust(t, err)
	truth, err := manager.MkTrue()
	pdrMust(t, err)
	for index, declaration := range []ast.ASTHandle{predicateP, predicateR} {
		variable, x := manager.MkVar(uint32(index+1), boolean)
		pdrMust(t, x)
		head, x := manager.MkApp(declaration, []ast.ASTHandle{variable})
		pdrMust(t, x)
		constraint, x := manager.MkEq(variable, truth)
		pdrMust(t, x)
		rule, x := ruleManager.NewRule(head, []ast.ASTHandle{constraint}, nil, "project-fact")
		pdrMust(t, x)
		pdrMust(t, set.Add(rule))
		pdrMust(t, rule.Close())
	}
	x, err := manager.MkVar(0, boolean)
	pdrMust(t, err)
	y, err := manager.MkVar(1, boolean)
	pdrMust(t, err)
	z, err := manager.MkVar(2, boolean)
	pdrMust(t, err)
	qHead, err := manager.MkApp(predicateQ, []ast.ASTHandle{x})
	pdrMust(t, err)
	pTail, err := manager.MkApp(predicateP, []ast.ASTHandle{y})
	pdrMust(t, err)
	rTail, err := manager.MkApp(predicateR, []ast.ASTHandle{z})
	pdrMust(t, err)
	coupling, err := manager.MkEq(y, z)
	pdrMust(t, err)
	qRule, err := ruleManager.NewRule(qHead, []ast.ASTHandle{pTail, rTail, coupling}, nil, "project-coupled")
	pdrMust(t, err)
	pdrMust(t, set.Add(qRule))
	pdrMust(t, qRule.Close())
	pdrMust(t, context.Initialize(set))
	status, err := context.Solve(predicateQ)
	pdrMust(t, err)
	if status != lbool.True {
		t.Fatal("coupled predecessor query", status)
	}

	pdrMust(t, context.Close())
	pdrMust(t, set.Close())
	pdrMust(t, ruleManager.Close())
	pdrMust(t, command.Close())
}
