package fp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/fp"
)

func TestDatalogRelationSnapshotsCopySaturatedTables(t *testing.T) {
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
	name, err := manager.Symbols.Intern("snapshot-P")
	fpMust(t, err)
	declaration, err := manager.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	fpMust(t, err)
	fpMust(t, engine.RegisterPredicate(declaration))
	fpMust(t, engine.AddTableFact(declaration, []uint32{1}))
	if _, err = engine.QueryPredicates([]ast.ASTHandle{declaration}); err != nil {
		t.Fatal(err)
	}
	snapshots, err := engine.DatalogRelationSnapshots([]ast.ASTHandle{declaration})
	fpMust(t, err)
	if len(snapshots) != 1 || len(snapshots[0].Facts) != 1 || snapshots[0].Facts[0][0] != 1 {
		t.Fatalf("snapshots %#v", snapshots)
	}
	snapshots[0].Facts[0][0] = 0
	again, err := engine.DatalogRelationSnapshots([]ast.ASTHandle{declaration})
	fpMust(t, err)
	if again[0].Facts[0][0] != 1 {
		t.Fatalf("aliased snapshot %#v", again)
	}
}
