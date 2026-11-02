package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func TestDatalogCheckedCompiledScopeExchangesCurrentManagerOwner(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.globals.SourceSetName("fixedpoint.datalog.default_table_checked", "true"))
	must(e.globals.SourceSetName("fixedpoint.datalog.default_table", "sparse"))
	must(e.globals.SourceSetName("fixedpoint.datalog.default_table_checker", "hashtable"))
	b, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("compiled-checked-scope")
	must(err)
	predicate, err := m.MkFuncDecl(name, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(predicate))
	must(e.SetPredicateRepresentation(predicate, []string{"tr_sparse"}))
	yes, err := m.MkTrue()
	must(err)
	must(e.AddFact(predicate, []ast.ASTHandle{yes}))

	check, err := e.beginDatalogCheckRelation()
	must(err)
	compiled, supported, err := check.beginCompiled()
	must(err)
	if !supported {
		t.Fatal("checked table scope did not enable compilation")
	}
	stored, err := e.relations.GetFiniteRelation(predicate)
	must(err)
	pair, ok := stored.(*rel.FiniteCheckedPair)
	if !ok {
		t.Fatalf("manager did not receive checked pair: %T", stored)
	}
	signature, err := pair.Signature()
	must(err)
	temporary, err := e.relations.NewDefaultFiniteTable(signature)
	must(err)
	if _, ok = temporary.(*rel.FiniteCheckedPair); !ok {
		t.Fatalf("compiler temporary lost checking: %T", temporary)
	}
	must(temporary.Close())

	replacement, err := pair.CloneFinite()
	must(err)
	must(e.relations.StoreFiniteRelation(predicate, replacement))
	must(compiled.Close())
	stored, err = e.relations.GetFiniteRelation(predicate)
	must(err)
	if _, ok = stored.(*rel.SparseTable); !ok {
		t.Fatalf("original manager representation was not restored: %T", stored)
	}
	entryPair := check.entries[len(check.entries)-1].pair
	if entryPair != replacement {
		t.Fatal("current compiled owner was not transferred to checker scope")
	}
	if found, err := entryPair.Contains(rel.Fact{1}); err != nil || !found {
		t.Fatal("compiled result facts", found, err)
	}
	if e.predicates[predicate].table != entryPair {
		t.Fatal("query state did not retain current compiled result")
	}
	must(check.complete(false))
}

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

func TestDatalogCheckedCompiledScopeRetiresUnexpectedCurrentOwner(t *testing.T) {
	f := newDirectFactFixture(t)
	e := f.engine
	fragmentMust(t, e.globals.SourceSetName("fixedpoint.datalog.default_table_checked", "true"))
	fragmentMust(t, e.globals.SourceSetName("fixedpoint.datalog.default_table", "hashtable"))
	fragmentMust(t, e.globals.SourceSetName("fixedpoint.datalog.default_table_checker", "hashtable"))
	check, err := e.beginDatalogCheckRelation()
	fragmentMust(t, err)
	compiled, supported, err := check.beginCompiled()
	fragmentMust(t, err)
	if !supported {
		t.Fatal("checked scope did not enable compilation")
	}
	entry := &check.entries[0]
	signature, err := entry.pair.Signature()
	fragmentMust(t, err)
	unexpected := rel.NewHashTable(signature)
	fragmentMust(t, e.relations.StoreFiniteRelation(entry.declaration, unexpected))
	if err = compiled.Close(); !errors.Is(err, rel.ErrCheckTable) {
		t.Fatal("unexpected compiled owner was not diagnosed", err)
	}
	stored, err := e.relations.GetFiniteRelation(entry.declaration)
	fragmentMust(t, err)
	if stored != entry.original || check.compiled != nil {
		t.Fatal("unexpected owner cleanup did not restore manager scope")
	}
	if _, err = unexpected.Len(); err == nil {
		t.Fatal("unexpected displaced owner remained live")
	}
	fragmentMust(t, check.complete(false))
}

func TestDatalogCheckedCompilerFactoryUsesCachedFavourite(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.globals.SourceSetName("fixedpoint.datalog.default_table", "sparse"))
	must(e.globals.SourceSetName("fixedpoint.datalog.default_table_checked", "true"))
	must(e.globals.SourceSetName("fixedpoint.datalog.default_table_checker", "hashtable"))
	must(e.applyDatalogDefaultFiniteTable())

	local, err := e.params.NewValue()
	must(err)
	key := "datalog.default_table"
	hash, err := m.Symbols.Intern("hashtable")
	must(err)
	must(e.params.SetSymbolName(&local, &key, hash))
	must(e.SetQueryParameters(&local))
	must(e.params.Del(&local))

	check, err := e.beginDatalogCheckRelation()
	must(err)
	compiled, supported, err := check.beginCompiled()
	must(err)
	if !supported {
		t.Fatal("cached favourite checker did not enable compilation")
	}
	signature, err := rel.NewSignature([]uint64{2})
	must(err)
	temporary, err := e.relations.NewDefaultFiniteTable(signature)
	must(err)
	pair, ok := temporary.(*rel.FiniteCheckedPair)
	if !ok {
		t.Fatalf("checked temporary type: %T", temporary)
	}
	primary, checker, err := pair.ConcreteKinds()
	if err != nil || primary != "sparse" || checker != "hashtable" {
		t.Fatal("query-local update changed cached favourite", primary, checker, err)
	}
	must(temporary.Close())
	must(compiled.Close())
	must(check.complete(false))
}
