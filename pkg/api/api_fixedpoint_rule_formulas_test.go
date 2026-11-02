package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestAPIFixedpointRulesPreservePendingInputsAndFlushExpansion(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	boolean := f.pin(f.m.MkBoolSort())
	p, err := f.fp.DeclareRelation("enumerated-p", []ast.ASTHandle{boolean})
	astTestMust(t, err)
	x := f.pin(f.m.MkVar(0, boolean))
	atom := f.pin(f.m.MkApp(p, []ast.ASTHandle{x}))
	original := f.pin(f.m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{astTestSymbol(t, f.m, "enumerated-original-binder")}, Body: atom, Weight: 7}))
	split := f.pin(f.m.MkImplies(f.pin(f.m.MkOr(f.pAtom, f.qAtom)), f.uAtom))
	astTestMust(t, f.fp.AddRule(original, "quantified"))
	astTestMust(t, f.fp.AddRule(split, "split"))
	snapshot, err := f.fp.Rules()
	astTestMust(t, err)
	if len(snapshot) != 2 {
		t.Fatal("pending rule expanded during public enumeration", len(snapshot))
	}
	defer func() {
		for i := range snapshot {
			astTestMust(t, f.m.DelRef(&snapshot[i]))
		}
	}()
	for i, want := range []ast.ASTHandle{original, split} {
		got, err := f.m.RefGet(snapshot[i])
		astTestMust(t, err)
		if got != want {
			t.Fatal("pending formula identity/annotations changed", i, got, want)
		}
	}
	// Enumeration must not flush the pending source queue or alter scope state.
	astTestMust(t, f.fp.Push())
	astTestMust(t, f.fp.AddRule(f.pAtom, "scoped"))
	astTestMust(t, f.fp.Pop())
	before := f.m.Stats().LiveReferences
	again, err := f.fp.Rules()
	astTestMust(t, err)
	if len(again) != 2 {
		t.Fatal("scope changed enumeration", len(again))
	}
	for i := range again {
		astTestMust(t, f.m.DelRef(&again[i]))
	}
	if got := f.m.Stats().LiveReferences; got != before {
		t.Fatal("enumeration temporary reference leak", got, before)
	}
	status, err := f.fp.Query(p)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("quantified predicate query", status)
	}
	flushed, err := f.fp.Rules()
	astTestMust(t, err)
	defer func() {
		for i := range flushed {
			astTestMust(t, f.m.DelRef(&flushed[i]))
		}
	}()
	if len(flushed) != 3 {
		t.Fatal("flushed normalized clauses missing", len(flushed))
	}
	normalized, err := f.m.RefGet(flushed[0])
	astTestMust(t, err)
	if normalized == original {
		t.Fatal("query failed to normalize pending quantified formula")
	}
	astTestMust(t, f.fp.Close())
	for i, want := range []ast.ASTHandle{original, split} {
		got, err := f.m.RefGet(snapshot[i])
		astTestMust(t, err)
		if got != want {
			t.Fatal("enumeration snapshot lost its owner", i)
		}
	}
	for _, ref := range flushed {
		_, err := f.m.RefGet(ref)
		astTestMust(t, err)
	}
}
