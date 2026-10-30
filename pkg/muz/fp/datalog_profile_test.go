package fp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/fp"
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
	if _, err = engine.QueryPredicates(declarations); err != nil {
		t.Fatal(err)
	}
	profile, err := engine.DatalogProfile()
	fpMust(t, err)
	if profile.RuleCount != 1 || profile.RelationCount != 2 || profile.TupleCount != 1 || !profile.Compiled || profile.Relations[0].TupleCount != 1 || profile.Relations[1].TupleCount != 0 {
		t.Fatalf("profile %#v", profile)
	}
}
