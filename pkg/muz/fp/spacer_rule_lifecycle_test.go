package fp

// Source: src/muz/spacer/spacer_dl_interface.cpp check_reset/updt_params.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/spacer"
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
	// Exercise live predicate transformers: default inlining can eliminate u,
	// and source init_rules does not manufacture transformers for that registry entry.
	for _, key := range []string{"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear"} {
		must(e.globals.SourceSetName(key, "false"))
	}
	u := f.applications["u"]
	recursive, err := m.MkImplies(u, u)
	must(err)
	must(e.addRuleEager(recursive, "spacer-lifecycle-recursive"))
	addSpacerLifecycleIntegerRules(t, e, u)
	query := f.declarations["u"]
	status, err := e.QueryPredicate(query)
	must(err)
	if status != lbool.False {
		t.Fatal("negative integer must be unreachable", status)
	}
	inductive, err := e.Model(query)
	must(err)
	for auxiliary := range e.auxiliary {
		present, err := inductive.Core().HasInterpretation(auxiliary)
		must(err)
		if present {
			t.Fatal("synthetic query predicate escaped into the model")
		}
	}
	must(inductive.Close())
	original := e.spacer
	previous, found, err := original.Transformer(e.spacerPredicate(query))
	must(err)
	if !found {
		t.Fatal("missing first-query transformer")
	}
	// Install the already-established unreachable invariant explicitly. A fresh
	// synthetic query may reset and reprove it; a newly reachable fact must
	// invalidate it. Direct context update tests separately verify frame copying.
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
			must(e.addRuleEager(rule, "spacer-lifecycle-stronger"))
		}
		status, err = e.QueryPredicate(query)
		must(err)
		if status != lbool.False {
			t.Fatal("covered rules changed the unreachable result", strengthen, status)
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
		invariantRef, err := current.Formulas(spacer.InfinityLevel, false)
		must(err)
		invariant, err := m.RefGet(invariantRef)
		must(err)
		if !m.IsFalse(invariant) {
			t.Fatal("unreachable query did not reestablish its invariant")
		}
		must(m.DelRef(&invariantRef))
		previous = current
	}
	// The old recursive body cannot subsume a fresh fact. Keeping its learned
	// false invariant would incorrectly report this reachable predicate UNSAT.
	must(e.addRuleEager(u, "spacer-lifecycle-new-fact"))
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

func addSpacerLifecycleIntegerRules(t *testing.T, e *PropositionalEngine, u ast.ASTHandle) {
	t.Helper()
	m := e.manager
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	// Keep u in the live rule graph: cone elimination can discard a bare
	// u -> u cycle before Spacer sees it. This induction requires real frames.
	var roots []ast.ASTRef
	t.Cleanup(func() {
		for i := len(roots) - 1; i >= 0; i-- {
			must(m.DelRef(&roots[i]))
		}
	})
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		must(err)
		r, err := m.NewRef(h)
		must(err)
		roots = append(roots, r)
		return h
	}
	zero := own(m.ArithmeticIntegerNumeralText("0"))
	one := own(m.ArithmeticIntegerNumeralText("1"))
	integer, err := m.GetSort(zero)
	must(err)
	boolean := own(m.MkBoolSort())
	predicateName, err := m.Symbols.Intern("spacer-lifecycle-integer")
	must(err)
	pred := own(m.MkFuncDecl(predicateName, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{}))
	must(e.RegisterPredicate(pred))
	x := own(m.MkVar(0, integer))
	atX := own(m.MkApp(pred, []ast.ASTHandle{x}))
	atZero := own(m.MkApp(pred, []ast.ASTHandle{zero}))
	atNext := own(m.MkApp(pred, []ast.ASTHandle{own(m.MkArithmeticAdd(x, one))}))
	must(e.addRuleEager(atZero, "lifecycle-base"))
	must(e.addRuleEager(own(m.MkImplies(atX, atNext)), "lifecycle-step"))
	negative := own(m.MkArithmeticLT(x, zero))
	must(e.addRuleEager(own(m.MkImplies(own(m.MkAnd(atX, negative)), u)), "lifecycle-negative"))
}
