package transforms_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/dataflow"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// A nested dependency starts another ordinary reachability step, while nested
// dependencies in disconnected rules must not retain their definitions.
func TestConeNestedDependencyFollowsOnlyReachableDefinitions(t *testing.T) {
	f := newConeFixture(t, "Goal", "Seed", "Unused")
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	mkNested := func(name string) (ast.ASTHandle, ast.ASTHandle, ast.ASTHandle) {
		sym, e := f.manager.Symbols.Intern(name)
		coneMust(t, e)
		pred, e := f.manager.MkFuncDecl(sym, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, e)
		coneMust(t, f.rules.RegisterPredicate(pred))
		v, e := f.manager.MkVar(0, boolean)
		coneMust(t, e)
		app, e := f.manager.MkApp(pred, []ast.ASTHandle{v})
		coneMust(t, e)
		q, e := f.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: app})
		coneMust(t, e)
		truth, e := f.manager.MkTrue()
		coneMust(t, e)
		tail, e := f.manager.MkEq(q, truth)
		coneMust(t, e)
		return pred, app, tail
	}
	liveDecl, live, liveTail := mkNested("nested-live-transitive")
	deadDecl, dead, deadTail := mkNested("nested-dead-transitive")
	addTransformRule(t, f, f.applications["Seed"], nil, nil, "seed")
	addTransformRule(t, f, live, []ast.ASTHandle{f.applications["Seed"]}, nil, "live-definition")
	addTransformRule(t, f, f.applications["Goal"], []ast.ASTHandle{liveTail}, nil, "goal")
	addTransformRule(t, f, dead, nil, nil, "dead-definition")
	addTransformRule(t, f, f.applications["Unused"], []ast.ASTHandle{deadTail}, nil, "unused")
	coneMust(t, f.set.SetOutputPredicate(f.declarations["Goal"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("invalid fixture")
	}
	reach, err := dataflow.TopDown(f.set)
	coneMust(t, err)
	if !reach.IsReachable(liveDecl) || !reach.IsReachable(f.declarations["Seed"]) || reach.IsReachable(deadDecl) || reach.IsReachable(f.declarations["Unused"]) {
		t.Fatal("incorrect transitive reachability of nested dependencies")
	}
	plugin := transforms.NewConePlugin(0)
	defer func() { coneMust(t, plugin.Close()) }()
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	if result == nil {
		t.Fatal("disconnected rules were not pruned")
	}
	defer func() { coneMust(t, result.Close()) }()
	count, err := result.Len()
	coneMust(t, err)
	if count != 3 {
		t.Fatal("unexpected retained rule count", count)
	}
}
