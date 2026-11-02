package fp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func TestDatalogProfileCountsRulesRelationsTuplesAndCompiler(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, context.Close()) })
	manager, err := context.Manager()
	fpMust(t, err)
	engine, err := fp.NewPropositionalEngine(manager)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, engine.Close()) })
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	declarations := make([]ast.ASTHandle, 2)
	for i, text := range []string{"profile-P", "profile-Q"} {
		name, e := manager.Symbols.Intern(text)
		fpMust(t, e)
		declarations[i], e = manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
		fpMust(t, e)
		fpMust(t, engine.RegisterPredicate(declarations[i]))
	}
	fpMust(t, engine.AddTableFact(declarations[0], nil))
	// Keep a real Horn rule so instruction totals are exercised independently
	// of the directly stored seed tuple.
	fact, err := manager.MkApp(declarations[0], nil)
	fpMust(t, err)
	fpMust(t, engine.AddRule(fact, "profile-fact-rule"))
	if _, err = engine.QueryPredicates(declarations); err != nil {
		t.Fatal(err)
	}
	profile, err := engine.DatalogProfile()
	fpMust(t, err)
	if profile.RuleCount != 1 || profile.RelationCount != 2 || profile.TupleCount != 1 || !profile.Compiled || profile.Relations[0].TupleCount != 1 || profile.Relations[1].TupleCount != 0 {
		t.Fatalf("profile %#v", profile)
	}
	stats := statistics.New()
	fpMust(t, engine.CollectStatistics(stats))
	seen := make(map[string]bool)
	for index := 0; index < stats.Size(); index++ {
		seen[string(stats.Key(index))] = true
	}
	if !seen["saturation time"] || !seen["dl.total"] {
		t.Fatalf("Datalog statistics omit source saturation or instruction totals: %v", seen)
	}
}
