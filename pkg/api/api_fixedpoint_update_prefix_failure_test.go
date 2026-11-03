package api

// Source: dl_context.cpp passes the live rule set to mk_rule. The public
// 4.7.1 memory-limit oracle preserves completed clauses on a later failure.
// Here the exposed AST manager limit interrupts the second clause's QE-lite
// traversal. It is not the API context's query-only rlimit option.

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func apiUpdatePrefixFailureFormula(t *testing.T, f *apiFixedpointFixture) (formula, fromQ, fromGuardedP ast.ASTHandle) {
	t.Helper()
	parameters, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, parameters.SetBool("xform.fix_unbound_vars", true))
	astTestMust(t, f.fp.SetParameters(parameters))
	astTestMust(t, parameters.Close())
	boolean := f.pin(f.m.MkBoolSort())
	opaqueName, err := f.m.Symbols.Intern("opaque-prefix")
	astTestMust(t, err)
	opaque := f.pin(f.m.MkFuncDecl(opaqueName, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{}))
	x := f.pin(f.m.MkVar(0, boolean))
	guard := f.pin(f.m.MkApp(opaque, []ast.ASTHandle{x}))
	guarded := f.pin(f.m.MkAnd(f.pAtom, guard))
	body := f.pin(f.m.MkOr(guarded, f.qAtom))
	implication := f.pin(f.m.MkImplies(body, f.uAtom))
	formula = f.pin(f.m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean},
		Names: []symbol.Symbol{symbol.Numerical(0)}, Body: implication,
	}))
	fromQ = f.pin(f.m.MkImplies(f.qAtom, f.uAtom))
	// The completed second clause existentially binds the opaque guard's
	// variable, which is absent from all registered predicate arguments.
	// qe.cpp extract_vars uses the first fresh x constant; reconstruction
	// copies that declaration's name and resets the quantifier weight to one.
	freshName, err := f.m.Symbols.Intern("x!0")
	astTestMust(t, err)
	exists := f.pin(f.m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: false, Sorts: []ast.ASTHandle{boolean},
		Names: []symbol.Symbol{freshName}, Body: guard, Weight: 1,
	}))
	truth := f.pin(f.m.MkTrue())
	wrapped := f.pin(f.m.MkEq(exists, truth))
	fixedBody := f.pin(f.m.MkAnd(f.pAtom, wrapped))
	fromGuardedP = f.pin(f.m.MkImplies(fixedBody, f.uAtom))
	return
}

func interruptAPIFixedpointUpdateAfterPrefix(t *testing.T, f *apiFixedpointFixture, formula ast.ASTHandle, name symbol.Symbol) {
	t.Helper()
	limit := f.m.ResourceLimit()
	// The pure Q=>U clause is completed before the second clause's reduction
	// reaches this limit; 13 interrupts its first reduction checkpoint.
	astTestMust(t, limit.Push(13))
	cause := f.fp.UpdateRuleWithSymbol(formula, name)
	limit.Pop()
	if cause == nil || !strings.Contains(cause.Error(), "reduce unbound variables") ||
		!strings.Contains(cause.Error(), "max. resource limit exceeded") ||
		strings.Contains(cause.Error(), "non-trivial body") {
		t.Fatal("update did not preserve its late QE interruption", cause)
	}
}

func TestAPIFixedpointInterruptedUpdatePublishesPrefixAndRetries(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	formula, fromQ, fromGuardedP := apiUpdatePrefixFailureFormula(t, f)
	seed, err := f.m.Symbols.Intern("pending-seed")
	astTestMust(t, err)
	label := symbol.Numerical(7)
	astTestMust(t, f.fp.AddRuleWithSymbol(f.qAtom, seed))
	before, err := f.fp.Rules()
	astTestMust(t, err)
	defer releaseAPIRefs(t, f.m, before)
	interruptAPIFixedpointUpdateAfterPrefix(t, f, formula, label)
	checkAPIFailedUpdateRules(t, f, []ast.ASTHandle{fromQ, f.qAtom}, []symbol.Symbol{label, seed})
	prefix, err := f.fp.Rules()
	astTestMust(t, err)
	defer releaseAPIRefs(t, f.m, prefix)
	err = f.fp.UpdateRuleWithSymbol(formula, label)
	if err == nil || !strings.Contains(err.Error(), "non-trivial body") || strings.Contains(err.Error(), "resource limit") {
		t.Fatal("unlimited retry did not complete both clauses", err)
	}
	checkAPIFailedUpdateRules(t, f,
		[]ast.ASTHandle{fromQ, fromQ, fromGuardedP, f.qAtom},
		[]symbol.Symbol{label, label, label, seed})
	astTestMust(t, f.fp.Close())
	for _, snapshot := range []struct {
		refs []ast.ASTRef
		want []ast.ASTHandle
	}{{before, []ast.ASTHandle{f.qAtom}}, {prefix, []ast.ASTHandle{fromQ, f.qAtom}}} {
		if len(snapshot.refs) != len(snapshot.want) {
			t.Fatal("owning snapshot changed size")
		}
		for i, ref := range snapshot.refs {
			value, err := f.m.RefGet(ref)
			astTestMust(t, err)
			if value != snapshot.want[i] {
				t.Fatal("owning snapshot changed after retry and fixedpoint close", i)
			}
		}
	}
}

func TestAPIFixedpointInterruptedUpdatePrequeryPopRestoresPrefix(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	formula, fromQ, _ := apiUpdatePrefixFailureFormula(t, f)
	seed, err := f.m.Symbols.Intern("pending-seed")
	astTestMust(t, err)
	label := symbol.Numerical(7)
	astTestMust(t, f.fp.AddRuleWithSymbol(f.qAtom, seed))
	astTestMust(t, f.fp.Push())
	interruptAPIFixedpointUpdateAfterPrefix(t, f, formula, label)
	checkAPIFailedUpdateRules(t, f, []ast.ASTHandle{fromQ, f.qAtom}, []symbol.Symbol{label, seed})
	astTestMust(t, f.fp.Pop())
	checkAPIFailedUpdateRules(t, f, []ast.ASTHandle{f.qAtom}, []symbol.Symbol{seed})
	status, err := f.fp.Query(f.u)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("popped interrupted-update prefix still derives the query", status)
	}
	checkAPIFailedUpdateRules(t, f, []ast.ASTHandle{f.qAtom}, []symbol.Symbol{seed})
}

func TestAPIFixedpointUpdateAcceptsCanceledHNFSingleClause(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	formula, fromQ, _ := apiUpdatePrefixFailureFormula(t, f)
	seed, err := f.m.Symbols.Intern("pending-seed")
	astTestMust(t, err)
	label := symbol.Numerical(7)
	astTestMust(t, f.fp.AddRuleWithSymbol(f.qAtom, seed))
	limit := f.m.ResourceLimit()
	// One step earlier, HNF stops normally after producing its first clause;
	// conversion succeeds and update_rule sees a valid single-rule result.
	astTestMust(t, limit.Push(12))
	cause := f.fp.UpdateRuleWithSymbol(formula, label)
	limit.Pop()
	astTestMust(t, cause)
	checkAPIFailedUpdateRules(t, f, []ast.ASTHandle{fromQ, f.qAtom}, []symbol.Symbol{label, seed})
	status, err := f.fp.Query(f.u)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("successful partial-HNF rule did not derive the query", status)
	}
}
