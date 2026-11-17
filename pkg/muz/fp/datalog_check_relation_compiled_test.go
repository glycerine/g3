package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func TestDatalogFormulaCheckScopeDoesNotEnableCompiledExchange(t *testing.T) {
	f := newDirectFactFixture(t)
	e := f.engine
	fragmentMust(t, e.globals.SourceSetName("fixedpoint.datalog.check_relation", "hashtable"))
	check, err := e.beginDatalogCheckRelation()
	fragmentMust(t, err)
	compiled, supported, err := check.beginCompiled()
	if err != nil || supported || compiled != nil {
		t.Fatal("formula checker enabled compiled pair scope", compiled, supported, err)
	}
	fragmentMust(t, check.complete(false))
}

// datalog.default_table_checked makes check_table_plugin the favourite table
// plugin (relation_manager::register_plugin): a predicate's table is a check
// table from its creation on, the compiler's temporaries are check tables,
// and the query runs without a checked scope.
func TestDatalogDefaultTableCheckedTablesFromCreation(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.globals.SourceSetName("fixedpoint.datalog.default_table_checked", "true"))
	must(e.globals.SourceSetName("fixedpoint.datalog.default_table", "sparse"))
	must(e.globals.SourceSetName("fixedpoint.datalog.default_table_checker", "hashtable"))
	b, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("checked-from-creation")
	must(err)
	predicate, err := m.MkFuncDecl(name, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(predicate))
	yes, err := m.MkTrue()
	must(err)
	must(e.AddFact(predicate, []ast.ASTHandle{yes}))
	stored, err := e.relations.GetFiniteRelation(predicate)
	must(err)
	pair, ok := stored.(*rel.FiniteCheckedPair)
	if !ok {
		t.Fatalf("predicate table is not a check table: %T", stored)
	}
	primary, checker, err := pair.ConcreteKinds()
	if err != nil || primary != "sparse" || checker != "hashtable" {
		t.Fatal("check table kinds", primary, checker, err)
	}
	if found, err := pair.Contains(rel.Fact{1}); err != nil || !found {
		t.Fatal("check table facts", found, err)
	}
	signature, err := pair.Signature()
	must(err)
	temporary, err := e.relations.NewDefaultFiniteTable(signature)
	must(err)
	if _, ok = temporary.(*rel.FiniteCheckedPair); !ok {
		t.Fatalf("compiler temporary is not a check table: %T", temporary)
	}
	must(temporary.Close())
	check, err := e.beginDatalogCheckRelation()
	must(err)
	if check != nil {
		t.Fatal("default_table_checked made a checked scope")
	}
}
