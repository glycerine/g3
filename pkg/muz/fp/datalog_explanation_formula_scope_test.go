package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestExplanationFormulaScopeDispatchesSelectedBacking(t *testing.T) {
	f := newDirectFactFixture(t)
	e := f.engine
	scope, enabled, err := e.beginDatalogExplanationFormulaScope()
	fragmentMust(t, err)
	if scope != nil || enabled {
		t.Fatal("default unexpectedly installs formula checker")
	}
	for _, name := range []string{"tr_sparse", "tr_lazy_sparse"} {
		fragmentMust(t, e.globals.SourceSetName("fixedpoint.datalog.check_relation", name))
		scope, enabled, err = e.beginDatalogExplanationFormulaScope()
		if err != nil || scope == nil || !enabled {
			t.Fatal("selected finite checker was not dispatched", name, scope, enabled, err)
		}
		want := "sparse"
		if name == "tr_lazy_sparse" {
			want = "lazy_sparse"
		}
		if scope.backing != want {
			t.Fatal("selected finite backing changed", name, scope.backing, want)
		}
		fragmentMust(t, scope.Close())
	}
	fragmentMust(t, e.globals.SourceSetName("fixedpoint.datalog.check_relation", "doc"))
	scope, enabled, err = e.beginDatalogExplanationFormulaScope()
	fragmentMust(t, err)
	if !enabled || scope == nil || scope.backing != "doc" {
		t.Fatal("selected checked UDoc backing was not dispatched", scope, enabled)
	}
	factory, found, err := e.relations.RelationExplanationUDocFactory()
	fragmentMust(t, err)
	if !found || !factory.FormulaChecked() {
		t.Fatal("selected doc explanation factory is not formula checked", found, factory)
	}
	fragmentMust(t, scope.Close())
}

// AUDIT.md muz_fp_01-F4: rel_context::query(num_rels, rels) never applies
// mk_explanations, so a relation query ignores generate_explanations.
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
			if e.datalogExplanationAnswer != (ast.ASTRef{}) {
				t.Fatal("relation query built an explanation answer")
			}
		})
	}
}
