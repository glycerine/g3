package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogDefaultFiniteTableSelectionAndLifetime(t *testing.T) {
	for _, tc := range []struct {
		name, module, local, explicit string
		sparse                        bool
	}{
		{name: "source-default", sparse: true},
		{name: "module-hash", module: "hashtable"},
		{name: "local-sparse", module: "hashtable", local: "sparse", sparse: true},
		{name: "explicit-hash", explicit: "tr_hashtable"},
		{name: "explicit-sparse", module: "hashtable", explicit: "tr_sparse", sparse: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newDirectFactFixture(t)
			e, m := f.engine, f.manager
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			must(e.SelectEngine("datalog"))
			b, err := m.MkBoolSort()
			must(err)
			name, err := m.Symbols.Intern("default-table-P")
			must(err)
			d, err := m.MkFuncDecl(name, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
			must(err)
			must(e.RegisterPredicate(d)) // Registration precedes parameter selection.
			if tc.module != "" {
				must(e.globals.SourceSetName("fixedpoint.datalog.default_table", tc.module))
			}
			if tc.local != "" {
				value, err := e.params.NewValue()
				must(err)
				key := "datalog.default_table"
				symbol, err := m.Symbols.Intern(tc.local)
				must(err)
				must(e.params.SetSymbolName(&value, &key, symbol))
				must(e.SetQueryParameters(&value))
				must(e.params.Del(&value))
			}
			if tc.explicit != "" {
				must(e.SetPredicateRepresentation(d, []string{tc.explicit}))
			}
			yes, err := m.MkTrue()
			must(err)
			must(e.AddFact(d, []ast.ASTHandle{yes}))
			check := func() {
				_, sparse := e.predicates[d].table.(*rel.SparseTable)
				if sparse != tc.sparse {
					t.Fatal("configured concrete storage", sparse, tc.sparse)
				}
				if _, ok := e.predicates[f.declarations["p"]].table.(*rel.HashTable); !ok {
					t.Fatal("nullary default lost Hash fallback")
				}
				found, err := e.predicates[d].table.Contains(rel.Fact{1})
				must(err)
				if !found {
					t.Fatal("default migration lost seed tuple")
				}
			}
			check()
			// Source engine favourites survive later updt_params. The Go clone must
			// copy this lifetime and the explicit predicate override independently.
			must(e.globals.SourceSetName("fixedpoint.datalog.default_table", "hashtable"))
			must(e.SetQueryParameters(nil))
			query, err := m.MkApp(d, []ast.ASTHandle{yes})
			must(err)
			status, err := e.QueryExpression(query)
			must(err)
			if status != lbool.True {
				t.Fatal("configured default expression query", status)
			}
			check()
			if e.queryClone.predicates[d].explicitFiniteRepresentation != (tc.explicit != "") {
				t.Fatal("clone lost explicit representation marker")
			}
		})
	}
}
