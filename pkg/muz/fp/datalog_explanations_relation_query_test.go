package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogRelationQueryIgnoresExplanations(t *testing.T) {
	for _, favourite := range []string{"", "doc"} {
		t.Run("favourite="+favourite, func(t *testing.T) {
			f := newDirectFactFixture(t)
			e, m := f.engine, f.manager
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			must(e.SelectEngine("datalog"))
			if favourite != "" {
				must(e.globals.SourceSetName("fixedpoint.datalog.default_relation", favourite))
			}
			must(e.globals.SourceSetName("fixedpoint.datalog.generate_explanations", "true"))
			must(e.globals.SourceSetName("fixedpoint.datalog.explanations_on_relation_level", "true"))
			must(e.AddFact(f.declarations["p"], nil))
			step, err := m.MkImplies(f.applications["p"], f.applications["q"])
			must(err)
			must(e.addRuleEager(step, "relation-step"))
			status, err := e.QueryPredicate(f.declarations["q"])
			must(err)
			if status != lbool.True {
				t.Fatal(status)
			}
			// rel_context::query(num_rels, rels) never runs mk_explanations:
			// the explanation plugin is not registered.
			if e.relations.GetRelationPlugin("relation_explanation") != nil {
				t.Fatal("relation query built an explanation answer")
			}
		})
	}
}
