package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/rel"
)

func TestDatalogExplanationCheckedScopeConfiguredConcretePairs(t *testing.T) {
	for _, tc := range []struct{ primary, checker string }{
		{primary: "hashtable", checker: "sparse"},
		{primary: "sparse", checker: "hashtable"},
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
