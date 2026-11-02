package fp

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestDatalogCheckedCompiledConstructorsAndMixedTargets(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	for key, value := range map[string]string{"default_table": "hashtable", "default_table_checked": "true", "default_table_checker": "sparse", "unbound_compressor": "false"} {
		must(e.globals.SourceSetName("fixedpoint.datalog."+key, value))
	}
	b, err := m.MkBoolSort()
	must(err)
	declare := func(name string) ast.ASTHandle {
		sym, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(sym, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p, q, r := declare("checked-total"), declare("checked-singleton"), declare("checked-join")
	must(e.SetPredicateRepresentation(r, []string{"tr_sparse"}))
	x, err := m.MkVar(0, b)
	must(err)
	yes, err := m.MkTrue()
	must(err)
	app := func(d, v ast.ASTHandle) ast.ASTHandle { h, err := m.MkApp(d, []ast.ASTHandle{v}); must(err); return h }
	must(e.AddRule(app(p, x), "total"))
	must(e.AddRule(app(q, yes), "singleton"))
	body, err := m.MkAnd(app(p, x), app(q, x))
	must(err)
	rule, err := m.MkImplies(body, app(r, x))
	must(err)
	must(e.AddRule(rule, "join"))
	rule, err = m.MkImplies(app(r, yes), f.applications["u"])
	must(err)
	must(e.AddRule(rule, "nullary-output"))
	for iteration := 0; iteration < 2; iteration++ {
		status, err := e.QueryPredicate(f.declarations["u"])
		must(err)
		if status != lbool.True || !e.compiled {
			t.Fatal("checked query failed compiled execution", status, e.compiled)
		}
		for _, spec := range []struct {
			d     ast.ASTHandle
			count int
		}{{p, 2}, {q, 1}, {r, 1}, {f.declarations["u"], 1}} {
			rows, err := e.predicates[spec.d].table.Len()
			must(err)
			if rows != spec.count {
				t.Fatal("checked constructor/join tuple count", rows, spec.count)
			}
			if _, checked := e.predicates[spec.d].table.(*rel.FiniteCheckedPair); checked {
				t.Fatal("checked owner escaped query scope")
			}
		}
		if _, sparse := e.predicates[r].table.(*rel.SparseTable); !sparse {
			t.Fatal("mixed target lost explicit sparse representation")
		}
		found, err := e.predicates[r].table.Contains(rel.Fact{0})
		must(err)
		if found {
			t.Fatal("join invented false tuple")
		}
		signature, err := e.predicates[q].table.Signature()
		must(err)
		temporary, err := e.relations.NewDefaultFiniteTable(signature)
		must(err)
		if _, hash := temporary.(*rel.HashTable); !hash {
			t.Fatal("query left checked temporary factory installed")
		}
		must(temporary.Close())
	}
}
