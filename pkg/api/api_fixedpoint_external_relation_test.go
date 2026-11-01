package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	muzrel "github.com/glycerine/g3/pkg/muz/rel"
)

func TestSMTContextFixedpointExternalRelationCallbackRegistration(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	manager, err := context.Manager()
	astTestMust(t, err)
	boolean, err := manager.MkBoolSort()
	astTestMust(t, err)
	initialAssignments := 0
	astTestMust(t, context.SetFixedpointExternalRelationCallbacks(muzrel.ExternalRelationCallbacks{
		ReduceAssign: func(ast.ASTHandle, []ast.ASTHandle, []ast.ASTHandle) error {
			initialAssignments++
			return nil
		},
	}))
	relation, err := context.NewFixedpointExternalRelation([]ast.ASTHandle{boolean})
	astTestMust(t, err)
	if initialAssignments != 1 {
		t.Fatal("initial empty relation callback", initialAssignments)
	}
	reductions, assignments := 0, 0
	astTestMust(t, context.SetFixedpointExternalRelationCallbacks(muzrel.ExternalRelationCallbacks{
		Reduce: func(declaration ast.ASTHandle, _ []ast.ASTHandle) (ast.ASTHandle, error) {
			kind, err := manager.GetDeclKind(declaration)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if kind != ast.DatalogOpIsEmpty {
				t.Fatalf("unexpected reduction kind %d", kind)
			}
			reductions++
			return manager.MkTrue()
		},
		ReduceAssign: func(ast.ASTHandle, []ast.ASTHandle, []ast.ASTHandle) error {
			assignments++
			return nil
		},
	}))
	empty, err := relation.Empty()
	astTestMust(t, err)
	if !empty {
		t.Fatal("registered reduction result not used")
	}
	truth, err := manager.MkTrue()
	astTestMust(t, err)
	astTestMust(t, relation.AddFact([]ast.ASTHandle{truth}))
	if reductions != 1 || assignments != 1 || initialAssignments != 1 {
		t.Fatal("callback replacement on live relation", reductions, assignments, initialAssignments)
	}
	if err := context.Close(); err == nil {
		t.Fatal("fixedpoint context closed while its callback relation was live")
	}
	if _, err := relation.Relation(); err != nil {
		t.Fatal("failed context close damaged the live relation", err)
	}
	astTestMust(t, relation.Close())
	astTestMust(t, context.Close())
}
