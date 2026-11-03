package fp

import (
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestQueryMagicSetsRunsForSyntheticQueryPredicate(t *testing.T) {
	for _, tc := range []struct {
		name   string
		global bool
		local  *bool
	}{
		{name: "default"}, {name: "global", global: true},
		{name: "local-enables", local: magicQueryBool(true)},
		{name: "local-disables", global: true, local: magicQueryBool(false)},
	} {
		t.Run(tc.name, func(t *testing.T) { testQueryMagicSets(t, tc.global, tc.local) })
	}
}
func magicQueryBool(value bool) *bool { return &value }
func testQueryMagicSets(t *testing.T, global bool, local *bool) {
	manager, globals := newPDRLifecycleServices(t)
	if err := globals.SourceSetName("fixedpoint.datalog.magic_sets_for_queries", strconv.FormatBool(global)); err != nil {
		t.Fatal(err)
	}
	engine, err := NewPropositionalEngineWithServices(manager, globals.Context, globals)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	}()
	enabled := global
	if local != nil {
		enabled = *local
		params := globals.Context.EmptyValue()
		name := "datalog.magic_sets_for_queries"
		if err = globals.Context.SetBoolName(&params, &name, *local); err != nil {
			t.Fatal(err)
		}
		if err = engine.SetQueryParameters(&params); err != nil {
			t.Fatal(err)
		}
		if err = globals.Context.Del(&params); err != nil {
			t.Fatal(err)
		}
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	declare := func(name string) ast.ASTHandle {
		t.Helper()
		symbol, x := manager.Symbols.Intern(name)
		if x != nil {
			t.Fatal(x)
		}
		declaration, x := manager.MkFuncDecl(symbol, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
		if x != nil {
			t.Fatal(x)
		}
		if x = engine.RegisterPredicate(declaration); x != nil {
			t.Fatal(x)
		}
		return declaration
	}
	extensional := declare("magic-query-E")
	goalRelation := declare("magic-query-P")
	truth, err := manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	if err = engine.AddFact(extensional, []ast.ASTHandle{truth}); err != nil {
		t.Fatal(err)
	}
	variable, err := manager.MkVar(0, boolean)
	if err != nil {
		t.Fatal(err)
	}
	extensionalApp, err := manager.MkApp(extensional, []ast.ASTHandle{variable})
	if err != nil {
		t.Fatal(err)
	}
	goalApp, err := manager.MkApp(goalRelation, []ast.ASTHandle{variable})
	if err != nil {
		t.Fatal(err)
	}
	rule, err := manager.MkImplies(extensionalApp, goalApp)
	if err != nil {
		t.Fatal(err)
	}
	if err = engine.AddRule(rule, "magic-query-rule"); err != nil {
		t.Fatal(err)
	}
	query, err := manager.MkApp(goalRelation, []ast.ASTHandle{truth})
	if err != nil {
		t.Fatal(err)
	}
	queryRef, err := manager.NewRef(query)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := manager.DelRef(&queryRef); err != nil {
			t.Error(err)
		}
	}()
	clone, err := engine.newQueryClone()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := clone.Close(); err != nil {
			t.Error(err)
		}
	}()
	declaration, headRef, err := clone.makeQueryHead(query)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { fragmentMust(t, closeASTRefs(manager, []ast.ASTRef{headRef})) }()
	head, err := manager.RefGet(headRef)
	fragmentMust(t, err)
	if err = clone.RegisterPredicate(declaration); err != nil {
		t.Fatal(err)
	}
	queryRule, err := manager.MkImplies(query, head)
	if err != nil {
		t.Fatal(err)
	}
	if err = clone.AddRule(queryRule, "magic-query-wrapper"); err != nil {
		t.Fatal(err)
	}
	valid, err := clone.set.Finalize()
	if err != nil || !valid {
		t.Fatalf("finalize query rules: valid=%v err=%v", valid, err)
	}
	before := len(clone.predicateOrder)
	if err = clone.applyQueryMagicSets(declaration); err != nil {
		t.Fatal(err)
	}
	if changed := len(clone.predicateOrder) > before; changed != enabled {
		t.Fatalf("adorned predicates created=%v, enabled=%v", changed, enabled)
	}
	status, err := clone.QueryPredicate(declaration)
	if err != nil {
		t.Fatal(err)
	}
	if status != lbool.True {
		t.Fatalf("magic-set query status=%v, want true", status)
	}
	beforeRules, err := engine.set.Len()
	if err != nil {
		t.Fatal(err)
	}
	beforePredicates := len(engine.predicateOrder)
	for _, value := range []bool{true, false, true} {
		arg, err := manager.MkTrue()
		if !value {
			arg, err = manager.MkFalse()
		}
		if err != nil {
			t.Fatal(err)
		}
		expression, err := manager.MkApp(goalRelation, []ast.ASTHandle{arg})
		if err != nil {
			t.Fatal(err)
		}
		ref, err := manager.NewRef(expression)
		if err != nil {
			t.Fatal(err)
		}
		status, err := engine.QueryExpression(expression)
		closeErr := manager.DelRef(&ref)
		if err != nil || closeErr != nil {
			t.Fatal(err, closeErr)
		}
		if status != lbool.FromBool(value) {
			t.Fatalf("query P(%v) = %v", value, status)
		}
	}
	afterRules, err := engine.set.Len()
	if err != nil {
		t.Fatal(err)
	}
	if beforeRules != afterRules || beforePredicates != len(engine.predicateOrder) {
		t.Fatal("query preprocessing changed the persistent program")
	}

}
