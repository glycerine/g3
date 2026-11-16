package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Source: dl_table_relation.cpp create_plugin_name prefixes concrete table
// names with tr_; rel_context::updt_params looks the check_relation base up
// with get_relation_plugin(symbol), which finds tr_hashtable but no plugin
// named hashtable or an unknown name (check_relation then has no base).
func TestDatalogCheckRelationSourceTableNames(t *testing.T) {
	for _, tc := range []struct {
		name string
		base bool
	}{{"tr_hashtable", true}, {"tr_sparse", true}, {"hashtable", false}, {"sparse", false}, {"tr_missing-plugin", false}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newDirectFactFixture(t)
			e, m := f.engine, f.manager
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			must(e.SelectEngine("datalog"))
			must(e.globals.SourceSetName("fixedpoint.datalog.check_relation", tc.name))
			name, enabled, err := e.datalogCheckRelationBase()
			must(err)
			if !enabled || name != tc.name {
				t.Fatal("check_relation name", name, enabled)
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
			if !e.compiledRelationPlugins {
				t.Fatal("check_relation did not select the relation plugins")
			}
			check := e.relations.GetRelationPlugin("check_relation").(*rel.CheckRelationPlugin)
			if got := check.BasePlugin() != nil; got != tc.base {
				t.Fatal("check_relation base", got, tc.base)
			}
			if got := check.BasePlugin() != nil && check.BasePlugin().Name() == tc.name; got != tc.base {
				t.Fatal("check_relation base name", check.BasePlugin())
			}
			_, isRelation := rel.AsRelation(e.predicates[p].table)
			if isRelation != tc.base {
				t.Fatal("predicate relation kind", isRelation, tc.base)
			}
			if tc.base {
				// The checked query runs in cmd/g3 (plugins4_check_tr_* vectors).
				return
			}
			px, err := m.MkApp(p, []ast.ASTHandle{yes})
			must(err)
			rule, err := m.MkImplies(px, f.applications["q"])
			must(err)
			must(e.addRuleEager(rule, "canonical-checker"))
			for i := 0; i < 2; i++ {
				status, err := e.QueryPredicate(f.declarations["q"])
				must(err)
				if status != lbool.True {
					t.Fatal("query", i, status)
				}
			}
		})
	}
}
