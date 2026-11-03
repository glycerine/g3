package api

// Source: dl_context.cpp context::update_rule installs mk_rule's clauses before
// rejecting a non-single result. Native Z3 4.7.1 confirms this observable order.

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func checkAPIFailedUpdateRules(t *testing.T, f *apiFixedpointFixture, expected []ast.ASTHandle, expectedNames []symbol.Symbol) {
	t.Helper()
	formulas, err := f.fp.Rules()
	astTestMust(t, err)
	defer releaseAPIRefs(t, f.m, formulas)
	named, names, err := f.fp.command.FixedpointNamedRulesWithSymbols()
	astTestMust(t, err)
	defer releaseAPIRefs(t, f.m, named)
	if len(formulas) != len(expected) || len(named) != len(expected) || len(names) != len(expectedNames) {
		t.Fatalf("failed-update rules: public=%d named=%d names=%d, want %d", len(formulas), len(named), len(names), len(expected))
	}
	for i, want := range expected {
		formula, err := f.m.RefGet(formulas[i])
		astTestMust(t, err)
		namedFormula, err := f.m.RefGet(named[i])
		astTestMust(t, err)
		if formula != want || namedFormula != want || names[i] != expectedNames[i] {
			t.Fatalf("failed-update rule %d: formula=%v named=%v name=%v, want %v/%v", i, formula, namedFormula, names[i], want, expectedNames[i])
		}
	}
}

func apiFailedUpdateSplit(t *testing.T, f *apiFixedpointFixture) (formula, fromQ, fromP ast.ASTHandle) {
	t.Helper()
	disjunction := f.pin(f.m.MkOr(f.pAtom, f.qAtom))
	formula = f.pin(f.m.MkImplies(disjunction, f.uAtom))
	fromQ = f.pin(f.m.MkImplies(f.qAtom, f.uAtom))
	fromP = f.pin(f.m.MkImplies(f.pAtom, f.uAtom))
	return
}

func TestAPIFixedpointRejectedSplitUpdatePublishesClauses(t *testing.T) {
	for _, flushed := range []bool{false, true} {
		phase := "pending-seed"
		if flushed {
			phase = "flushed-seed"
		}
		t.Run(phase, func(t *testing.T) {
			f := newAPIFixedpointFixture(t)
			seed, err := f.m.Symbols.Intern("seed")
			astTestMust(t, err)
			split, err := f.m.Symbols.Intern("split")
			astTestMust(t, err)
			astTestMust(t, f.fp.AddRuleWithSymbol(f.pAtom, seed))
			if flushed {
				status, err := f.fp.Query(f.p)
				astTestMust(t, err)
				if status != lbool.True {
					t.Fatal("seed did not flush", status)
				}
			}
			snapshot, err := f.fp.Rules()
			astTestMust(t, err)
			defer releaseAPIRefs(t, f.m, snapshot)
			formula, fromQ, fromP := apiFailedUpdateSplit(t, f)
			err = f.fp.UpdateRuleWithSymbol(formula, split)
			if err == nil || !strings.Contains(err.Error(), "non-trivial body") {
				t.Fatal("split update must reject its two-clause result", err)
			}
			want := []ast.ASTHandle{fromQ, fromP, f.pAtom}
			names := []symbol.Symbol{split, split, seed}
			if flushed {
				want = []ast.ASTHandle{f.pAtom, fromQ, fromP}
				names = []symbol.Symbol{seed, split, split}
			}
			checkAPIFailedUpdateRules(t, f, want, names)
			if len(snapshot) != 1 {
				t.Fatal("pre-update snapshot changed length", len(snapshot))
			}
			previous, err := f.m.RefGet(snapshot[0])
			astTestMust(t, err)
			if previous != f.pAtom {
				t.Fatal("rejected update changed an owning prior snapshot")
			}
			status, err := f.fp.Query(f.u)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("query ignored clauses installed by the rejected update", status)
			}
			checkAPIFailedUpdateRules(t, f, want, names)
		})
	}
}

func TestAPIFixedpointRejectedSplitUpdatePrequeryPopRestoresRules(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	seed, err := f.m.Symbols.Intern("seed")
	astTestMust(t, err)
	split, err := f.m.Symbols.Intern("split")
	astTestMust(t, err)
	astTestMust(t, f.fp.AddRuleWithSymbol(f.pAtom, seed))
	astTestMust(t, f.fp.Push())
	formula, fromQ, fromP := apiFailedUpdateSplit(t, f)
	err = f.fp.UpdateRuleWithSymbol(formula, split)
	if err == nil || !strings.Contains(err.Error(), "non-trivial body") {
		t.Fatal("split update must reject its two-clause result", err)
	}
	checkAPIFailedUpdateRules(t, f, []ast.ASTHandle{fromQ, fromP, f.pAtom}, []symbol.Symbol{split, split, seed})
	astTestMust(t, f.fp.Pop())
	checkAPIFailedUpdateRules(t, f, []ast.ASTHandle{f.pAtom}, []symbol.Symbol{seed})
	status, err := f.fp.Query(f.u)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("popped split clauses still derive the query", status)
	}
	checkAPIFailedUpdateRules(t, f, []ast.ASTHandle{f.pAtom}, []symbol.Symbol{seed})
}
