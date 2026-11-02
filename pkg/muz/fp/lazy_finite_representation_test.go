package fp

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestDatalogLazySparseMigrationAndExpressionContextLifetime(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	b, err := m.MkBoolSort()
	must(err)
	sym, err := m.Symbols.Intern("lazy-lifetime-P")
	must(err)
	d, err := m.MkFuncDecl(sym, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(d))
	yes, err := m.MkTrue()
	must(err)
	must(e.AddFact(d, []ast.ASTHandle{yes}))
	previous := e.predicates[d].table
	must(e.SetPredicateRepresentation(d, []string{"tr_lazy_sparse"}))
	if _, err := previous.Len(); err == nil {
		t.Fatal("migration left old table alive")
	}
	if !e.lazyFinite.Owns(e.predicates[d].table) {
		t.Fatal("lazy factory does not own selected predicate")
	}
	factory, selected := e.lazyFinite, e.predicates[d].table
	must(e.SetPredicateRepresentation(d, []string{"tr_lazy_sparse"}))
	if e.predicates[d].table != selected {
		t.Fatal("same representation replaced lazy owner")
	}
	must(e.SetPredicateRepresentation(d, []string{"tr_hashtable"}))
	if _, err := selected.Len(); err == nil {
		t.Fatal("leaving lazy representation kept the old handle live")
	}
	must(e.SetPredicateRepresentation(d, []string{"tr_lazy_sparse"}))
	if e.lazyFinite != factory {
		t.Fatal("representation changes replaced context factory")
	}
	query, err := m.MkApp(d, []ast.ASTHandle{yes})
	must(err)
	ref, err := m.NewRef(query)
	must(err)
	defer func() { must(m.DelRef(&ref)) }()
	for repeat := 0; repeat < 2; repeat++ {
		status, err := e.QueryExpression(query)
		must(err)
		if status != lbool.True {
			t.Fatal(status)
		}
		clone := e.queryClone
		if clone.lazyFinite == nil || clone.lazyFinite == e.lazyFinite || !clone.lazyFinite.Owns(clone.predicates[d].table) || e.lazyFinite.Owns(clone.predicates[d].table) {
			t.Fatal("expression clone shares lazy engine ownership")
		}
		if !e.lazyFinite.Owns(e.predicates[d].table) {
			t.Fatal("query publication imported clone engine owner")
		}
		must(e.clearQueryResources())
		found, err := e.predicates[d].table.Contains(rel.Fact{1})
		must(err)
		if !found {
			t.Fatal("closing query clone destroyed published row")
		}
	}
	// A separately owned clone must also survive parent destruction.
	clone, err := e.newQueryClone()
	must(err)
	defer func() { must(clone.Close()) }()
	must(e.Close())
	found, err := clone.predicates[d].table.Contains(rel.Fact{1})
	must(err)
	if !found {
		t.Fatal("parent destruction destroyed independent clone row")
	}
}
