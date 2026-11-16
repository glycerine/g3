package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

// TestExternalIntervalDatalogSelectionPreconditions:
// rel_context::get_ordinary_relation_plugin reports the missing
// external_relation_plugin without Z3_fixedpoint_init (the callbacks);
// with it, set_predicate_representation records a product kind over
// external_relation and interval_relation for any signature without
// invoking a callback.
func TestExternalIntervalDatalogSelectionPreconditions(t *testing.T) {
	manager, globals := newPDRLifecycleServices(t)
	engine, err := NewPropositionalEngineWithServices(manager, globals.Context, globals)
	fragmentMust(t, err)
	t.Cleanup(func() { fragmentMust(t, engine.Close()) })
	fragmentMust(t, engine.SelectEngine("datalog"))
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	boolean, err := manager.MkBoolSort()
	fragmentMust(t, err)
	name, err := manager.Symbols.Intern("external-interval-plugin-precondition")
	fragmentMust(t, err)
	p, err := manager.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	fragmentMust(t, engine.RegisterPredicate(p))
	kinds := []string{"external_relation", "interval_relation"}
	if err = engine.SetPredicateRepresentation(p, kinds); err == nil || err.Error() != "relation plugin external_relation does not exist" || len(engine.predicates[p].requestedRepresentations) != 0 {
		t.Fatal("uninstalled external plugin selected", err, engine.predicates[p].requestedRepresentations)
	}
	calls := 0
	fragmentMust(t, engine.SetExternalRelationCallbacks(rel.ExternalRelationCallbacks{
		Reduce: func(ast.ASTHandle, []ast.ASTHandle) (ast.ASTHandle, error) {
			calls++
			return ast.ASTHandle{}, nil
		},
		ReduceAssign: func(ast.ASTHandle, []ast.ASTHandle, []ast.ASTHandle) error {
			calls++
			return nil
		},
	}))
	fragmentMust(t, engine.SetPredicateRepresentation(p, kinds))
	name, err = manager.Symbols.Intern("external-interval-wrong-sort")
	fragmentMust(t, err)
	wrong, err := manager.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	fragmentMust(t, engine.RegisterPredicate(wrong))
	fragmentMust(t, engine.SetPredicateRepresentation(wrong, kinds))
	if calls != 0 {
		t.Fatal("selection invoked callbacks", calls)
	}
}
