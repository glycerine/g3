package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func TestDatalogExplanationCheckedScopeConfiguredConcretePairs(t *testing.T) {
	for _, tc := range []struct{ primary, checker string }{
		{primary: "hashtable", checker: "hashtable"},
		{primary: "hashtable", checker: "sparse"},
		{primary: "sparse", checker: "hashtable"},
		{primary: "lazy_sparse", checker: "hashtable"},
		{primary: "hashtable", checker: "lazy_sparse"},
		{primary: "sparse", checker: "sparse"},
		{primary: "sparse", checker: "lazy_sparse"},
		{primary: "lazy_sparse", checker: "sparse"},
		{primary: "lazy_sparse", checker: "lazy_sparse"},
	} {
		t.Run(tc.primary+"-"+tc.checker, func(t *testing.T) {
			f := newDirectFactFixture(t)
			e := f.engine
			fragmentMust(t, e.globals.SourceSetName("fixedpoint.datalog.default_table", tc.primary))
			fragmentMust(t, e.globals.SourceSetName("fixedpoint.datalog.default_table_checked", "true"))
			fragmentMust(t, e.globals.SourceSetName("fixedpoint.datalog.default_table_checker", tc.checker))
			scope, enabled, err := e.beginDatalogExplanationCheckedScope()
			fragmentMust(t, err)
			if !enabled || scope == nil {
				t.Fatal("configured explanation checker scope disabled")
			}
			signature, err := rel.NewSignature([]uint64{2})
			fragmentMust(t, err)
			table, err := e.relations.NewDefaultFiniteTable(signature)
			fragmentMust(t, err)
			pair, ok := table.(*rel.FiniteCheckedPair)
			if !ok {
				t.Fatalf("factory table type = %T", table)
			}
			primary, checker, err := pair.ConcreteKinds()
			fragmentMust(t, err)
			if primary != tc.primary || checker != tc.checker {
				t.Fatal("concrete checked kinds", primary, checker)
			}
			fragmentMust(t, table.Close())
			boolean, err := f.manager.MkBoolSort()
			fragmentMust(t, err)
			factProduct, err := rel.NewFiniteExplanationProduct(e.relations, []ast.ASTHandle{boolean})
			fragmentMust(t, err)
			factPrimary, factChecker, factChecked, err := factProduct.CheckedConcreteKinds()
			fragmentMust(t, err)
			wantFactChecked := tc.primary != "hashtable" && tc.checker != "hashtable"
			if wantFactChecked {
				if !factChecked || factPrimary != tc.primary || factChecker != tc.checker {
					t.Fatal("functional explanation index was not checked", factPrimary, factChecker, factChecked)
				}
			} else if factChecked || factPrimary != "sparse" || factChecker != "" {
				t.Fatal("Hash functional explanation index did not fall back to ordinary Sparse", factPrimary, factChecker, factChecked)
			}
			fragmentMust(t, scope.requireProduct(factProduct))
			fragmentMust(t, factProduct.Close())
			nullary, err := rel.NewRelationExplanationProduct(e.relations, nil)
			fragmentMust(t, err)
			nullaryPrimary, nullaryChecker, nullaryChecked, err := nullary.CheckedConcreteKinds()
			fragmentMust(t, err)
			if tc.primary == "hashtable" && tc.checker == "hashtable" {
				if !nullaryChecked || nullaryPrimary != "hashtable" || nullaryChecker != "hashtable" {
					t.Fatal("hash/hash nullary was not checked", nullaryPrimary, nullaryChecker, nullaryChecked)
				}
			} else if nullaryChecked || nullaryPrimary != "hashtable" {
				t.Fatal("unsupported nullary pair did not fall back to ordinary hash", nullaryPrimary, nullaryChecker, nullaryChecked)
			}
			fragmentMust(t, scope.requireProduct(nullary))
			fragmentMust(t, nullary.Close())
			fragmentMust(t, scope.Close())
			ordinary, err := e.relations.NewDefaultFiniteTable(signature)
			fragmentMust(t, err)
			if _, checked := ordinary.(*rel.FiniteCheckedPair); checked {
				t.Fatal("checked factory escaped explanation scope")
			}
			fragmentMust(t, ordinary.Close())
			fragmentMust(t, scope.Close())
		})
	}
}
