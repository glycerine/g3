package fp

// Source: src/muz/spacer/spacer_dl_interface.cpp check_reset/updt_params.

import (
	"github.com/glycerine/g3/pkg/muz/spacer"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSpacerRepeatedQueriesReuseContextAndResetUncoveredRules(t *testing.T) {
	f := newPDRLifecycleFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(e.SelectEngine("spacer"))
	u := f.applications["u"]
	recursive, err := m.MkImplies(u, u)
	must(err)
	must(e.AddRule(recursive, "spacer-lifecycle-recursive"))
	query := f.declarations["u"]
	status, err := e.QueryPredicate(query)
	must(err)
	if status != lbool.False {
		t.Fatal("recursive-only predicate must be unreachable", status)
	}
	original := e.spacer
	previous, found, err := original.Transformer(e.spacerPredicate(query))
	must(err)
	if !found {
		t.Fatal("missing first-query transformer")
	}
	// Install the already-established unreachable invariant explicitly so the
	// test detects accidental loss on covered rules and reuse on a fresh fact.
	falsehood, err := m.MkFalse()
	must(err)
	_, err = previous.AddLemma(falsehood, spacer.InfinityLevel)
	must(err)
	for _, strengthen := range []bool{false, true} {
		if strengthen {
			body, err := m.MkAnd(u, f.applications["p"])
			must(err)
			rule, err := m.MkImplies(body, u)
			must(err)
			must(e.AddRule(rule, "spacer-lifecycle-stronger"))
		}
		status, err = e.QueryPredicate(query)
		must(err)
		if status != lbool.False {
			t.Fatal("covered rules changed the recursive-only result", strengthen, status)
		}
		if e.spacer != original {
			t.Fatal("source check_reset must preserve the context for covered rules", strengthen)
		}
		current, found, err := e.spacer.Transformer(e.spacerPredicate(query))
		must(err)
		if !found || current == previous {
			t.Fatal("update_rules did not replace its transformer")
		}
		if _, err := previous.Head(); err == nil {
			t.Fatal("old transformer survived rule update")
		}
		invariant, err := current.Formulas(spacer.InfinityLevel, false)
		must(err)
		if !m.IsFalse(invariant) {
			t.Fatal("covered rule update lost its known invariant")
		}
		previous = current
	}
	// The old recursive body cannot subsume a fresh fact. Keeping its learned
	// false invariant would incorrectly report this reachable predicate UNSAT.
	must(e.AddRule(u, "spacer-lifecycle-new-fact"))
	status, err = e.QueryPredicate(query)
	must(err)
	if status != lbool.True {
		t.Fatal("uncovered new fact did not discard the old blocking invariant", status)
	}
	if e.spacer != original {
		t.Fatal("rule reset must preserve the source context/manager object")
	}
	if e.spacerRules == nil {
		t.Fatal("query did not own its source-rule snapshot")
	}
	// Parameter updates use the distinct source updt_params branch and
	// reconstruct the context rather than keeping constructor policy snapshots.
	parameters, err := e.params.NewValue()
	must(err)
	defer func() { must(e.params.Del(&parameters)) }()
	name := "spacer.native_mbp"
	must(e.params.SetBoolName(&parameters, &name, true))
	must(e.SetQueryParameters(&parameters))
	if e.spacer != nil || e.spacerRules != nil {
		t.Fatal("parameter update kept the previous Spacer context or rule snapshot")
	}
	status, err = e.QueryPredicate(query)
	must(err)
	if status != lbool.True || e.spacer == original {
		t.Fatal("parameter update did not construct a fresh usable Spacer context", status)
	}
	must(e.SelectEngine("pdr"))
	status, err = e.QueryPredicate(f.declarations["q"])
	must(err)
	if status != lbool.True {
		t.Fatal("PDR query after Spacer", status)
	}
	if e.spacer != nil || e.spacerRules != nil {
		t.Fatal("switching to PDR left Spacer state alive")
	}
}

func TestSpacerRuleSnapshotOwnsOriginalRulesBeforePreprocessing(t *testing.T) {
	f := newPDRLifecycleFixture(t)
	e := f.engine
	if err := e.SelectEngine("spacer"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"q", "p", "q"} {
		status, err := e.QueryPredicate(f.declarations[name])
		if err != nil || status != lbool.True {
			t.Fatal("query across preprocessing roots", name, status, err)
		}
		count, err := e.spacerRules.Len()
		if err != nil || count != 2 {
			t.Fatal("snapshot must cover both original rules before query slicing", count, err)
		}
		for i := 0; i < count; i++ {
			rule, err := e.spacerRules.Rule(i)
			if err != nil {
				t.Fatal(err)
			}
			head, err := rule.HeadDeclaration()
			if err != nil {
				t.Fatal(err)
			}
			if head != f.declarations["p"] && head != f.declarations["q"] {
				t.Fatal("snapshot contained a transformed declaration", head)
			}
			if err = rule.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}
