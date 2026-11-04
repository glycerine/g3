package dataflow_test

// Source: src/muz/dataflow/dataflow.h and reachability.h bottom-up and
// top-down concrete reachability execution.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/dataflow"
)

func reachMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestReachabilityRunsFromIngestedHornProgram(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	reachMust(t, err)
	t.Cleanup(func() { reachMust(t, context.Close()) })
	manager, err := context.Manager()
	reachMust(t, err)
	rules, err := base.NewRuleManager(manager)
	reachMust(t, err)
	t.Cleanup(func() { reachMust(t, rules.Close()) })
	set, err := base.NewRuleSet(rules)
	reachMust(t, err)
	t.Cleanup(func() { reachMust(t, set.Close()) })
	boolean, err := manager.MkBoolSort()
	reachMust(t, err)
	declarations := make(map[string]ast.ASTHandle)
	applications := make(map[string]ast.ASTHandle)
	for _, name := range []string{"P", "Q", "R", "U", "V", "N", "M", "S", "Z"} {
		symbol, e := manager.Symbols.Intern("reach-" + name)
		reachMust(t, e)
		declarations[name], e = manager.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
		reachMust(t, e)
		applications[name], e = manager.MkApp(declarations[name], nil)
		reachMust(t, e)
		reachMust(t, rules.RegisterPredicate(declarations[name]))
	}
	add := func(formula ast.ASTHandle, name string) {
		t.Helper()
		reference, e := manager.NewRef(formula)
		reachMust(t, e)
		t.Cleanup(func() { reachMust(t, manager.DelRef(&reference)) })
		reachMust(t, rules.AddFormula(set, formula, name))
	}
	add(applications["P"], "fact-P")
	for _, edge := range []struct{ from, to string }{{"P", "Q"}, {"Q", "R"}, {"U", "V"}} {
		formula, e := manager.MkImplies(applications[edge.from], applications[edge.to])
		reachMust(t, e)
		add(formula, edge.from+"-"+edge.to)
	}
	notN, err := manager.MkNot(applications["N"])
	reachMust(t, err)
	negative, err := manager.MkImplies(notN, applications["M"])
	reachMust(t, err)
	add(negative, "not-N-M")
	both, err := manager.MkAndArray([]ast.ASTHandle{applications["P"], applications["U"]})
	reachMust(t, err)
	partial, err := manager.MkImplies(both, applications["S"])
	reachMust(t, err)
	add(partial, "partial-S")
	reachMust(t, set.SetOutputPredicate(declarations["R"]))
	reachMust(t, set.SetOutputPredicate(declarations["M"]))
	reachMust(t, set.SetOutputPredicate(declarations["Z"]))
	if _, err = dataflow.BottomUp(set); !errors.Is(err, dataflow.ErrReachabilityRules) {
		t.Fatal("open rule set accepted by dataflow", err)
	}
	valid, err := set.Finalize()
	reachMust(t, err)
	if !valid {
		t.Fatal("reachability fixture rejected")
	}

	bottom, err := dataflow.BottomUp(set)
	reachMust(t, err)
	top, err := dataflow.TopDown(set)
	reachMust(t, err)
	for _, name := range []string{"P", "Q", "R"} {
		if !bottom.IsReachable(declarations[name]) || !top.IsReachable(declarations[name]) {
			t.Fatal("reachable predicate omitted", name)
		}
	}
	for _, name := range []string{"U", "V"} {
		if bottom.IsReachable(declarations[name]) || top.IsReachable(declarations[name]) {
			t.Fatal("disconnected predicate retained", name)
		}
	}
	if !bottom.IsReachable(declarations["M"]) || bottom.IsReachable(declarations["N"]) {
		t.Fatal("bottom-up negative-tail reachability does not match source")
	}
	if !top.IsReachable(declarations["M"]) || !top.IsReachable(declarations["N"]) {
		t.Fatal("top-down negative-tail reachability does not match source")
	}
	if bottom.IsReachable(declarations["S"]) || top.IsReachable(declarations["Z"]) {
		t.Fatal("partially supported head or output without rules marked reachable")
	}
	knownOrder := bottom.KnownDeclarations()
	wantKnownOrder := []ast.ASTHandle{declarations["P"], declarations["M"], declarations["Q"], declarations["S"], declarations["R"]}
	if len(knownOrder) != len(wantKnownOrder) {
		t.Fatal("source fact database order", knownOrder)
	}
	known := make(map[ast.ASTHandle]bool)
	for index, declaration := range knownOrder {
		if declaration != wantKnownOrder[index] {
			t.Fatal("source fact database order", knownOrder, wantKnownOrder)
		}
		known[declaration] = true
	}
	for _, name := range []string{"P", "Q", "R", "M", "S"} {
		if !known[declarations[name]] {
			t.Fatal("source fact database entry omitted", name)
		}
	}
	for _, name := range []string{"U", "V", "N", "Z"} {
		if known[declarations[name]] {
			t.Fatal("unevaluated declaration received a source fact entry", name)
		}
	}
}
