package rel

// Source: relation_manager::get_relation and
// external_relation_plugin::mk_empty.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestExternalPredicateRelationDispatchAndOwnership(t *testing.T) {
	f := newTableRelationFixture(t)
	emptyCalls := 0
	plugin, err := NewExternalRelationPlugin(f.relations, ExternalRelationCallbacks{
		ReduceAssign: func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error {
			kind, err := f.ast.GetDeclKind(declaration)
			if err != nil {
				return err
			}
			if kind == ast.DatalogOpEmpty {
				emptyCalls++
				if len(arguments) != 0 || len(outputs) != 1 {
					t.Fatal("empty callback shape", arguments, outputs)
				}
			}
			return nil
		},
	})
	astTestMust(t, err)
	name, err := f.ast.Symbols.Intern("external-owned-predicate")
	astTestMust(t, err)
	predicate, err := f.ast.MkFuncDecl(name, []ast.ASTHandle{f.boolean, f.boolean}, f.boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	owner, err := f.ast.NewRef(predicate)
	astTestMust(t, err)

	first, err := plugin.PredicateRelation(predicate)
	astTestMust(t, err)
	second, err := plugin.PredicateRelation(predicate)
	astTestMust(t, err)
	if first != second || emptyCalls != 1 {
		t.Fatal("predicate dispatch did not cache the external relation", first == second, emptyCalls)
	}
	sorts, err := first.Sorts()
	astTestMust(t, err)
	if len(sorts) != 2 || sorts[0] != f.boolean || sorts[1] != f.boolean {
		t.Fatal("predicate signature was not transferred", sorts)
	}
	astTestMust(t, f.ast.DelRef(&owner))
	if _, err = f.ast.GetKind(predicate); err != nil {
		t.Fatal("plugin did not retain predicate declaration", err)
	}
	astTestMust(t, plugin.DropPredicateRelation(predicate))
	if _, err = first.Relation(); err == nil {
		t.Fatal("dropped manager relation remained open")
	}
	if f.ast.IsLiveHandle(predicate) {
		t.Fatal("dropping manager relation did not release predicate declaration")
	}
	astTestMust(t, plugin.Close())
}

func TestExternalPredicateRelationPluginCloseRetiresOwnedRelations(t *testing.T) {
	f := newTableRelationFixture(t)
	plugin, err := NewExternalRelationPlugin(f.relations, ExternalRelationCallbacks{})
	astTestMust(t, err)
	var owned []*ExternalRelation
	for _, arity := range []int{0, 1} {
		name, err := f.ast.Symbols.Intern("external-close-owned")
		astTestMust(t, err)
		domain := make([]ast.ASTHandle, arity)
		for i := range domain {
			domain[i] = f.boolean
		}
		predicate, err := f.ast.MkFuncDecl(name, domain, f.boolean, ast.ASTFuncDeclInfo{})
		astTestMust(t, err)
		relation, err := plugin.PredicateRelation(predicate)
		astTestMust(t, err)
		if relation == nil {
			t.Fatal("nil dispatched relation")
		}
		owned = append(owned, relation)
	}
	astTestMust(t, plugin.Close())
	for _, relation := range owned {
		if _, err := relation.Relation(); err == nil {
			t.Fatal("plugin close left a manager-owned relation open")
		}
	}
	if _, err := plugin.PredicateRelation(ast.ASTHandle{}); err == nil {
		t.Fatal("closed plugin accepted predicate dispatch")
	}
}
