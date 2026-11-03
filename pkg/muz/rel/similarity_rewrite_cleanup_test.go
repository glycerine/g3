package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestSimilarityRewriteConstructionCleanupKeepsNestedOwnersForRetry(t *testing.T) {
	f := newExplanationNegativeFixture(t, "hashtable")
	p, q, auxiliary := f.declare("rewrite-cleanup-P", 2), f.declare("rewrite-cleanup-Q", 2), f.declare("rewrite-cleanup-aux", 1)
	x := f.own(f.ast.MkVar(7, f.boolean))
	head := f.own(f.ast.MkApp(p, []ast.ASTHandle{x, f.truth}))
	tail := f.own(f.ast.MkApp(q, []ast.ASTHandle{x, f.truth}))
	source, err := f.rules.NewRule(head, []ast.ASTHandle{tail}, nil, "rewrite-cleanup-step")
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, source.Close()) })
	positions := []similarityConstant{{tail: -1, argument: 1, parent: -1}, {tail: 0, argument: 1, parent: 0}}
	rewritten, err := similarityRewriteRule(f.rules, source, positions, auxiliary)
	astTestMust(t, err)
	newRoot := func(name string) (ast.ASTHandle, ast.ASTRef) {
		t.Helper()
		symbol, err := f.ast.Symbols.Intern(name)
		astTestMust(t, err)
		term, err := f.ast.MkConst(symbol, f.boolean)
		astTestMust(t, err)
		ref, err := f.ast.NewRef(term)
		astTestMust(t, err)
		return term, ref
	}
	kept, keptRef := newRoot("rewrite-cleanup-kept")
	_, staleRef := newRoot("rewrite-cleanup-stale")
	_, trailingRef := newRoot("rewrite-cleanup-trailing")
	consumed := staleRef
	astTestMust(t, f.ast.DelRef(&consumed))
	inner := &similarityRewriteOwner{manager: f.ast, refs: []ast.ASTRef{keptRef, staleRef, trailingRef}}
	cause := errors.New("similarity construction failed")
	innerFailure := finishSimilarityRewriteConstruction(cause, inner)
	var innerReceipt memory.ConstructionFailure
	if !errors.As(innerFailure, &innerReceipt) || len(inner.refs) != 2 {
		t.Fatal("native cleanup refusal lost its unconsumed temporary frontier", innerFailure, len(inner.refs))
	}
	outer := &similarityRewriteOwner{manager: f.ast, rule: rewritten}
	failure := finishSimilarityRewriteConstruction(innerFailure, outer)
	var receipt memory.ConstructionFailure
	if !errors.As(failure, &receipt) || receipt == innerReceipt || outer.rule != rewritten || outer.nested != innerReceipt {
		t.Fatal("rewrite failure lost its exact nested receipt or unpublished rule", failure)
	}
	if receipt.RuntimeOwner() != f.ast.NumeralService().RuntimeOwner() || !errors.Is(receipt.OriginalCause(), cause) || receipt.CleanupCause() == nil {
		t.Fatal("rewrite receipt lost native runtime or separate failure causes")
	}
	shown, exposed := receipt.Error(), failure.(interface{ Unwrap() error }).Unwrap()
	if err = receipt.Cleanup(); err == nil || receipt.Error() != shown || failure.(interface{ Unwrap() error }).Unwrap() != exposed || outer.rule != rewritten {
		t.Fatal("refused nested retry changed the error or released the outer rule", err)
	}
	if _, err = rewritten.Head(); err != nil {
		t.Fatal("nested cleanup refusal retired the unpublished rewritten rule", err)
	}
	if _, err = f.ast.GetKind(kept); err != nil {
		t.Fatal("nested cleanup refusal lost a still-owned AST root", err)
	}
	// The copied stale reference already consumed its native owner. Mark that
	// receipt consumed, then resume through the remaining real AST/rule roots.
	inner.refs[1] = ast.ASTRef{}
	astTestMust(t, receipt.Cleanup())
	astTestMust(t, receipt.Cleanup())
	if receipt.CleanupCause() != nil || outer.rule != nil || outer.nested != nil || len(inner.refs) != 0 {
		t.Fatal("successful rewrite cleanup left an owner frontier")
	}
	if _, err = f.ast.GetKind(kept); err == nil {
		t.Fatal("successful rewrite cleanup did not release the final temporary")
	}
	// Retry the live transformation with unchanged source owners and check the
	// fresh-variable/equal-column semantics, rather than only the receipt state.
	retry, err := similarityRewriteRule(f.rules, source, positions, auxiliary)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, retry.Close()) })
	retryHead, err := retry.Head()
	astTestMust(t, err)
	variable, err := f.ast.GetArg(retryHead, 1)
	astTestMust(t, err)
	index, err := f.ast.GetVarIndex(variable)
	astTestMust(t, err)
	auxiliaryTail, negative, err := retry.Tail(1)
	astTestMust(t, err)
	argument, err := f.ast.GetArg(auxiliaryTail, 0)
	astTestMust(t, err)
	// RuleManager normalizes the original x7 to x0, so the new orphan
	// constant receives x1 and every equal occurrence must share it.
	if index != 1 || argument != variable || negative {
		t.Fatal("semantic retry lost variable freshness or equal-column sharing", index, argument, variable, negative)
	}
	original, err := source.Head()
	astTestMust(t, err)
	constant, err := f.ast.GetArg(original, 1)
	astTestMust(t, err)
	if constant != f.truth {
		t.Fatal("cleanup/retry changed the original source rule")
	}
}
