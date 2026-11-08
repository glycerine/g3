package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestPublicRuleFormulasAppendNegatedOutputsAfterPendingInputs(t *testing.T) {
	f := newPDRLifecycleFixture(t)
	e, m := f.engine, f.manager
	e.flushRawDisplayRules()
	fragmentMust(t, e.set.SetOutputPredicate(f.declarations["q"]))
	fragmentMust(t, e.addRuleEager(f.applications["u"], "pending"))
	before := m.Stats().LiveReferences
	formulas, err := e.RuleFormulas()
	fragmentMust(t, err)
	if len(formulas) != 3 {
		t.Fatal("public rules/query count", len(formulas))
	}
	wantNegative, err := m.MkNot(f.applications["p"])
	fragmentMust(t, err)
	for i, want := range []ast.ASTHandle{f.applications["p"], f.applications["u"], wantNegative} {
		got, err := m.RefGet(formulas[i])
		fragmentMust(t, err)
		if got != want {
			t.Fatal("public normalized/pending/negated-query order", i, got, want)
		}
	}
	// Internal compiler/proof enumeration must continue to expose Horn clauses.
	internal, names, err := e.NamedRules()
	fragmentMust(t, err)
	if len(internal) != 3 || len(names) != 3 {
		t.Fatal("internal enumeration changed")
	}
	qRule, err := m.RefGet(internal[1])
	fragmentMust(t, err)
	expected, err := m.MkImplies(f.applications["p"], f.applications["q"])
	fragmentMust(t, err)
	if qRule != expected {
		t.Fatal("internal output clause was converted to a query")
	}
	for i := range internal {
		fragmentMust(t, m.DelRef(&internal[i]))
	}
	for i := range formulas {
		fragmentMust(t, m.DelRef(&formulas[i]))
	}
	if after := m.Stats().LiveReferences; after != before {
		t.Fatal("enumeration leaked reference owners", before, after)
	}
}
