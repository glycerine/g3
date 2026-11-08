package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Source: dl_table_relation.cpp create_plugin_name prefixes concrete table
// names with tr_; rel_context::updt_params looks up relation plugins.
func TestDatalogCheckRelationSourceTableNames(t *testing.T) {
	for _, tc := range []struct{ name, want string }{{"tr_hashtable", "hashtable"}, {"tr_sparse", "sparse"}, {"hashtable", "hashtable"}, {"sparse", "sparse"}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newDirectFactFixture(t)
			e, m := f.engine, f.manager
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			must(e.SelectEngine("datalog"))
			must(e.globals.SourceSetName("fixedpoint.datalog.check_relation", tc.name))
			name, enabled, err := e.datalogCheckRelationName()
			must(err)
			if !enabled || name != tc.want {
				t.Fatal("canonical relation selection", name, enabled)
			}
			b, err := m.MkBoolSort()
			must(err)
			pName, err := m.Symbols.Intern("source-table-name-P")
			must(err)
			p, err := m.MkFuncDecl(pName, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
			must(err)
			must(e.RegisterPredicate(p))
			yes, err := m.MkTrue()
			must(err)
			must(e.AddFact(p, []ast.ASTHandle{yes}))
			px, err := m.MkApp(p, []ast.ASTHandle{yes})
			must(err)
			rule, err := m.MkImplies(px, f.applications["q"])
			must(err)
			must(e.addRuleEager(rule, "canonical-checker"))
			for i := 0; i < 2; i++ {
				status, err := e.QueryPredicate(f.declarations["q"])
				must(err)
				if status != lbool.True || e.compiled {
					t.Fatal("checked query", i, status, e.compiled)
				}
			}
			must(e.globals.SourceSetName("fixedpoint.datalog.check_relation", "missing-module-plugin"))
			override, err := e.params.NewValue()
			must(err)
			overrideKey := "datalog.check_relation"
			selected, err := m.Symbols.Intern(tc.name)
			must(err)
			must(e.params.SetSymbolName(&override, &overrideKey, selected))
			must(e.SetQueryParameters(&override))
			must(e.params.Del(&override))
			status, err := e.QueryPredicate(f.declarations["q"])
			must(err)
			if status != lbool.True || e.compiled {
				t.Fatal("local canonical relation override", status, e.compiled)
			}
			local, err := e.params.NewValue()
			must(err)
			key := "datalog.check_relation"
			unknown, err := m.Symbols.Intern("tr_missing-plugin")
			must(err)
			must(e.params.SetSymbolName(&local, &key, unknown))
			must(e.SetQueryParameters(&local))
			must(e.params.Del(&local))
			if _, _, err = e.datalogCheckRelationName(); err == nil {
				t.Fatal("unknown source plugin guard removed")
			}
		})
	}
}
