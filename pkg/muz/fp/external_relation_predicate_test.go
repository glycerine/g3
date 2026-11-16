package fp_test

// Source: rel_context relation-manager dispatch and
// external_relation_plugin::mk_empty.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func TestFixedpointExternalPredicateRelationDispatch(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	manager, err := context.Manager()
	fpMust(t, err)
	engine, err := fp.NewPropositionalEngine(manager)
	fpMust(t, err)
	emptyCalls := 0
	fpMust(t, engine.SetExternalRelationCallbacks(rel.ExternalRelationCallbacks{
		ReduceAssign: func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error {
			kind, err := manager.GetDeclKind(declaration)
			if err != nil {
				return err
			}
			if kind == ast.DatalogOpEmpty {
				emptyCalls++
			}
			return nil
		},
	}))
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	name, err := manager.Symbols.Intern("fixedpoint-external-predicate")
	fpMust(t, err)
	predicate, err := manager.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	fpMust(t, err)
	if _, err = engine.ExternalPredicateRelation(predicate); err == nil {
		t.Fatal("unregistered predicate received an external relation")
	}
	fpMust(t, engine.RegisterPredicate(predicate))
	first, err := engine.ExternalPredicateRelation(predicate)
	fpMust(t, err)
	second, err := engine.ExternalPredicateRelation(predicate)
	fpMust(t, err)
	if first != second || emptyCalls != 1 {
		t.Fatal("fixedpoint dispatch did not reuse the manager relation", first == second, emptyCalls)
	}
	fpMust(t, engine.Close())
	if _, err = first.Relation(); err == nil {
		t.Fatal("fixedpoint close left manager-owned external relation open")
	}
	fpMust(t, context.Close())
}
