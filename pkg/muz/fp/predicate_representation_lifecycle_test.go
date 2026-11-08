package fp

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

// Source: dl_context.cpp set_predicate_representation ensures the configured
// engine, then sends relation plugins only to the relational engine.
func TestPredicateRepresentationConfiguredEngineLifecycle(t *testing.T) {
	for _, backend := range []string{"datalog", "pdr", "qpdr", "spacer", "bmc", "qbmc", "tab", "clp", "duality", "ddnf"} {
		t.Run(backend, func(t *testing.T) {
			f := newDirectFactFixture(t)
			e := f.engine
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			must(e.globals.SourceSetName("fixedpoint.engine", backend))
			must(e.Push())
			must(e.SetPredicateRepresentation(f.declarations["p"], nil))
			if e.engineSelectionSet {
				t.Fatal("empty request selected an engine")
			}
			must(e.Pop())
			must(e.Push())
			kind := "unknown-table-plugin"
			if backend == "datalog" {
				kind = "tr_hashtable"
			}
			must(e.SetPredicateRepresentation(f.declarations["p"], []string{kind}))
			selected, err := e.SelectedEngine()
			must(err)
			if selected.String() != backend {
				t.Fatalf("selected %s, want %s", selected, backend)
			}
			must(e.globals.SourceSetName("fixedpoint.engine", "ddnf"))
			must(e.SelectConfiguredEngine())
			selected, err = e.SelectedEngine()
			must(err)
			if selected.String() != backend {
				t.Fatal("representation request failed to cache engine selection")
			}
			err = e.Pop()
			if backend == "duality" {
				must(err)
			} else if err == nil {
				t.Fatal("instantiated backend allowed pop")
			}
		})
	}
}

func TestPredicateRepresentationDatalogGuardsInstantiateBeforeFailure(t *testing.T) {
	for _, kind := range []string{"unknown-table-plugin", "tr_sparse", "tr_lazy_sparse"} {
		t.Run(kind, func(t *testing.T) {
			f := newDirectFactFixture(t)
			e := f.engine
			fragmentMust(t, e.Push())
			if err := e.SetPredicateRepresentation(f.declarations["p"], []string{kind}); err == nil {
				t.Fatal("invalid representation accepted")
			}
			if !e.engineSelectionSet {
				t.Fatal("failed representation did not select engine")
			}
			if err := e.Pop(); err == nil {
				t.Fatal("failed representation did not instantiate engine")
			}
			if len(e.scopes) != 1 {
				t.Fatal("refused pop consumed scope")
			}
		})
	}
}

func TestPredicateRepresentationNonDatalogInfiniteDomain(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.globals.SourceSetName("fixedpoint.engine", "pdr"))
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	integer, err := m.GetSort(zero)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("representation-infinite")
	must(err)
	d, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(d))
	if e.predicates[d].table != nil {
		t.Fatal("expected infinite-domain predicate without finite table")
	}
	must(e.SetPredicateRepresentation(d, []string{"not-a-datalog-plugin"}))
	fact, err := m.MkApp(d, []ast.ASTHandle{zero})
	must(err)
	must(e.addRuleEager(fact, "fact"))
	status, err := e.QueryPredicate(d)
	must(err)
	if status != lbool.True {
		t.Fatal("PDR fact query", status)
	}
	revision := e.revision
	must(e.SetPredicateRepresentation(d, []string{"still-not-a-datalog-plugin"}))
	if e.revision != revision {
		t.Fatal("ignored representation invalidated solved program")
	}
	must(e.addRuleEager(f.applications["q"], "new-fact"))
	must(e.SetPredicateRepresentation(d, []string{"ignored-after-rule-update"}))
}

func TestPredicateRepresentationDatalogLateGuard(t *testing.T) {
	f := newDirectFactFixture(t)
	e := f.engine
	fragmentMust(t, e.SetPredicateRepresentation(f.declarations["p"], []string{"tr_bitvector"}))
	fragmentMust(t, e.AddFact(f.declarations["p"], nil))
	status, err := e.QueryPredicate(f.declarations["p"])
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("Datalog fact query", status)
	}
	if err := e.SetPredicateRepresentation(f.declarations["p"], []string{"tr_hashtable"}); err == nil {
		t.Fatal("late Datalog representation guard removed")
	}
	fragmentMust(t, e.SetPredicateRepresentation(f.declarations["p"], nil))
}
