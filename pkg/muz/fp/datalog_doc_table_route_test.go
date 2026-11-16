package fp

// AUDIT.md FUZZ-F5 (relation plugins): a doc favourite program whose rule
// joins a udoc relation with a table_relation (Bool columns udoc does not
// handle) runs through the compiled pipeline, where relation_manager
// combines them in product relations. The expected results are what the
// fork's C API (Z3_fixedpoint_add_fact, Z3_fixedpoint_query_relations)
// gives for the same calls.

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func TestDatalogDocTableJoinSeededHeadUnion(t *testing.T) {
	for _, tableKind := range []string{"hashtable", "sparse"} {
		t.Run(tableKind, func(t *testing.T) {
			m, g := newPDRLifecycleServices(t)
			e, err := NewPropositionalEngineWithServices(m, g.Context, g)
			fragmentMust(t, err)
			defer func() { fragmentMust(t, e.Close()) }()
			fragmentMust(t, g.SourceSetName("fixedpoint.datalog.default_table", tableKind))
			fragmentMust(t, g.SourceSetName("fixedpoint.datalog.default_relation", "doc"))
			fragmentMust(t, e.SelectEngine("datalog"))
			boolean, err := m.MkBoolSort()
			fragmentMust(t, err)
			bits, err := m.BitVectorSort(2)
			fragmentMust(t, err)
			declare := func(name string, sorts ...ast.ASTHandle) ast.ASTHandle {
				n, err := m.Symbols.Intern(name)
				fragmentMust(t, err)
				d, err := m.MkFuncDecl(n, sorts, boolean, ast.ASTFuncDeclInfo{})
				fragmentMust(t, err)
				fragmentMust(t, e.RegisterPredicate(d))
				return d
			}
			doc := declare("doc-table-route-D", bits)
			table := declare("doc-table-route-T", boolean, bits)
			head := declare("doc-table-route-H", bits, boolean)
			fragmentMust(t, e.AddTableFact(doc, []uint32{1}))
			fragmentMust(t, e.AddTableFact(table, []uint32{1, 1}))
			fragmentMust(t, e.AddTableFact(head, []uint32{3, 0}))
			x, err := m.MkVar(1, bits)
			fragmentMust(t, err)
			b, err := m.MkVar(0, boolean)
			fragmentMust(t, err)
			d, err := m.MkApp(doc, []ast.ASTHandle{x})
			fragmentMust(t, err)
			tt, err := m.MkApp(table, []ast.ASTHandle{b, x})
			fragmentMust(t, err)
			h, err := m.MkApp(head, []ast.ASTHandle{x, b})
			fragmentMust(t, err)
			body, err := m.MkAndArgs([]ast.ASTHandle{d, tt})
			fragmentMust(t, err)
			rule, err := m.MkImplies(body, h)
			fragmentMust(t, err)
			fragmentMust(t, e.addRuleEager(rule, "doc-table-route"))
			status, err := e.QueryPredicate(head)
			// The seeded head is a table_relation; the product relation
			// the join produces has no single inner relation to union
			// into it.
			want := "trying to perform unsupported union operation on relations of kinds tr_" + tableKind + " and product_relation"
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatal("seeded table head union", status, err)
			}
			if _, ok := rel.AsRelation(e.predicates[doc].table); !ok {
				t.Fatal("the doc predicate is not the favourite plugin's relation")
			}
		})
	}
}
