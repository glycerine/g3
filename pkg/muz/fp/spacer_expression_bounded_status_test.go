package fp

// Source: src/muz/spacer/spacer_dl_interface.cpp expression queries run in a
// cloned context; context::solve_core still reports datalog::BOUNDED.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSpacerExpressionQueryReportsCloneBoundedStatus(t *testing.T) {
	manager, globals := newPDRLifecycleServices(t)
	fragmentMust(t, globals.SourceSetName("fixedpoint.spacer.max_level", "0"))
	engine, err := NewPropositionalEngineWithServices(manager, globals.Context, globals)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, engine.Close()) }()
	fragmentMust(t, engine.SelectEngine("spacer"))

	var refs []ast.ASTRef
	defer func() {
		for index := len(refs) - 1; index >= 0; index-- {
			fragmentMust(t, manager.DelRef(&refs[index]))
		}
	}()
	own := func(handle ast.ASTHandle, err error) ast.ASTHandle {
		fragmentMust(t, err)
		reference, err := manager.NewRef(handle)
		fragmentMust(t, err)
		refs = append(refs, reference)
		return handle
	}
	boolean := own(manager.MkBoolSort())
	name, err := manager.Symbols.Intern("spacer-expression-bounded")
	fragmentMust(t, err)
	declaration := own(manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{}))
	fragmentMust(t, engine.RegisterPredicate(declaration))
	query := own(manager.MkApp(declaration, nil))
	fragmentMust(t, engine.addRuleEager(query, "fact"))

	status, err := engine.QueryExpression(query)
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("zero-level expression query", status)
	}
	if engine.spacer != nil || engine.queryClone == nil || engine.queryClone.spacer == nil {
		t.Fatal("expression query did not preserve the clone Spacer owner")
	}
	reason, err := engine.ReasonUnknown()
	fragmentMust(t, err)
	if reason != "bounded" {
		t.Fatal("expression query reason", reason)
	}
}
