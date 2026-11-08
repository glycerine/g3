package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogSparseRepresentationUsesConcreteStorage(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	b, err := m.MkBoolSort()
	must(err)
	declare := func(name string) ast.ASTHandle {
		n, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(n, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p, q := declare("actual-sparse-P"), declare("actual-sparse-Q")
	no, err := m.MkFalse()
	must(err)
	yes, err := m.MkTrue()
	must(err)
	must(e.SetPredicateRepresentation(p, []string{"tr_hashtable"}))
	must(e.AddFact(p, []ast.ASTHandle{no}))
	prior := e.predicates[p].table
	must(e.SetPredicateRepresentation(p, []string{"tr_sparse"}))
	must(e.SetPredicateRepresentation(q, []string{"tr_sparse"}))
	for _, d := range []ast.ASTHandle{p, q} {
		if _, ok := e.predicates[d].table.(*rel.SparseTable); !ok {
			t.Fatal("sparse representation is still an alias", d)
		}
		stored, err := e.relations.GetFiniteRelation(d)
		must(err)
		if stored != e.predicates[d].table {
			t.Fatal("manager and engine do not share selected owner")
		}
		if _, err := e.relations.GetRelation(d); err == nil {
			t.Fatal("Hash-only accessor silently aliased sparse storage")
		}
	}
	if _, err := prior.Len(); err == nil {
		t.Fatal("replaced HashTable owner remained live")
	}
	must(e.AddFact(p, []ast.ASTHandle{yes}))
	x, err := m.MkVar(0, b)
	must(err)
	px, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	qx, err := m.MkApp(q, []ast.ASTHandle{x})
	must(err)
	rule, err := m.MkImplies(px, qx)
	must(err)
	must(e.addRuleEager(rule, "sparse-step"))
	for i := 0; i < 2; i++ {
		status, err := e.QueryPredicate(q)
		must(err)
		if status != lbool.True || !e.compiled {
			t.Fatal("concrete sparse query did not use compiled evaluator", status, e.compiled)
		}
		table, ok := e.predicates[q].table.(*rel.SparseTable)
		if !ok {
			t.Fatal("query replaced actual sparse storage")
		}
		facts, err := table.Facts()
		must(err)
		if len(facts) != 2 {
			t.Fatal("sparse query lost Boolean tuples", facts)
		}
		for _, row := range []rel.Fact{{0}, {1}} {
			found, err := table.Contains(row)
			must(err)
			if !found {
				t.Fatal("missing sparse tuple", row)
			}
		}
	}
	query, err := m.MkApp(q, []ast.ASTHandle{yes})
	must(err)
	status, err := e.QueryExpression(query)
	must(err)
	if status != lbool.True {
		t.Fatal("sparse expression query", status)
	}
	if _, ok := e.predicates[q].table.(*rel.SparseTable); !ok {
		t.Fatal("expression publication changed selected representation")
	}
	if _, ok := e.predicates[f.declarations["u"]].table.(*rel.HashTable); !ok {
		t.Fatal("default nullary storage changed")
	}
}

func TestDatalogSparseRepresentationRoundTripAndEmptySignatureGuard(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	if err := e.SetPredicateRepresentation(f.declarations["p"], []string{"tr_sparse"}); err == nil {
		t.Fatal("nullary sparse guard removed")
	}
	b, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("sparse-roundtrip")
	must(err)
	d, err := m.MkFuncDecl(name, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(d))
	must(e.SetPredicateRepresentation(d, []string{"tr_sparse"}))
	yes, err := m.MkTrue()
	must(err)
	must(e.AddFact(d, []ast.ASTHandle{yes}))
	sparse := e.predicates[d].table
	must(e.SetPredicateRepresentation(d, []string{"tr_hashtable"}))
	if _, ok := e.predicates[d].table.(*rel.HashTable); !ok {
		t.Fatal("explicit HashTable selection failed")
	}
	if _, err := sparse.Len(); err == nil {
		t.Fatal("replaced SparseTable owner remained live")
	}
	found, err := e.predicates[d].table.Contains(rel.Fact{1})
	must(err)
	if !found {
		t.Fatal("representation roundtrip lost existing tuple")
	}
}
