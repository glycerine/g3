package base_test

// Source: src/muz/base/dl_rule.cpp rule::set_proof and
// rule_manager::mk_rule_rewrite_proof.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestRuleProofOwnershipAndRewriteTransfer(t *testing.T) {
	f := newRuleSetFixture(t, "P")
	if err := f.manager.ToggleProofMode(ast.ASTProofEnabled); err != nil {
		t.Fatal(err)
	}
	oldRule, err := f.rules.NewRule(f.app["P"], nil, nil, "old")
	ruleMust(t, err)
	newRule, err := f.rules.NewRule(f.app["P"], nil, nil, "new")
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, oldRule.Close()) })
	t.Cleanup(func() { ruleMust(t, newRule.Close()) })
	formula, err := oldRule.ToFormula()
	ruleMust(t, err)
	fact, err := f.manager.RefGet(formula)
	ruleMust(t, err)
	asserted, err := f.manager.MkProofAsserted(fact)
	ruleMust(t, err)
	externalProof, err := f.manager.NewRef(asserted)
	ruleMust(t, err)
	ruleMust(t, oldRule.SetProof(asserted))
	ruleMust(t, f.manager.DelRef(&externalProof))
	ruleMust(t, f.manager.DelRef(&formula))

	retained, err := oldRule.Retain()
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, retained.Close()) })
	ruleMust(t, oldRule.Close())
	if _, found, getErr := retained.Proof(); getErr != nil || !found {
		t.Fatal("retained rule lost its owned proof", found, getErr)
	}

	if err = f.manager.ToggleProofMode(ast.ASTProofDisabled); err != nil {
		t.Fatal(err)
	}
	ruleMust(t, f.rules.MkRuleRewriteProof(retained, newRule))
	if mode, modeErr := f.manager.ProofMode(); modeErr != nil || mode != ast.ASTProofDisabled {
		t.Fatal("rewrite proof failed to restore proof mode", mode, modeErr)
	}
	proof, found, err := newRule.Proof()
	if err != nil || !found {
		t.Fatal("rewritten rule has no proof", found, err)
	}
	if isProof, proofErr := f.manager.IsProof(proof); proofErr != nil || !isProof {
		t.Fatal("rewrite transfer returned a non-proof", isProof, proofErr)
	}
	proofFact, err := f.manager.GetProofFact(proof)
	ruleMust(t, err)
	newFormula, err := newRule.ToFormula()
	ruleMust(t, err)
	newFormulaHandle, err := f.manager.RefGet(newFormula)
	ruleMust(t, err)
	equal, err := f.manager.AreEqual(proofFact, newFormulaHandle)
	ruleMust(t, err)
	if !equal {
		t.Fatal("transferred proof proves the wrong rule formula")
	}
	ruleMust(t, f.manager.DelRef(&newFormula))
	ruleMust(t, retained.Close())
}

func TestRuleSetProofCanBeClearedAndRejectsExpressions(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q")
	if err := f.manager.ToggleProofMode(ast.ASTProofEnabled); err != nil {
		t.Fatal(err)
	}
	rule, err := f.rules.NewRule(f.app["P"], nil, nil, "proof-clear")
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, rule.Close()) })
	if err = rule.SetProof(f.app["P"]); err == nil {
		t.Fatal("accepted a non-proof expression")
	}
	proof, err := f.manager.MkProofAsserted(f.app["P"])
	ruleMust(t, err)
	proofRoot, err := f.manager.NewRef(proof)
	ruleMust(t, err)
	proofQ, err := f.manager.MkProofAsserted(f.app["Q"])
	ruleMust(t, err)
	proofQRoot, err := f.manager.NewRef(proofQ)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, f.manager.DelRef(&proofRoot)) })
	t.Cleanup(func() { ruleMust(t, f.manager.DelRef(&proofQRoot)) })
	proofCount, err := f.manager.GetRefCount(proof)
	ruleMust(t, err)
	proofQCount, err := f.manager.GetRefCount(proofQ)
	ruleMust(t, err)
	ruleMust(t, rule.SetProof(proof))
	attachedCount, err := f.manager.GetRefCount(proof)
	ruleMust(t, err)
	if attachedCount != proofCount+1 {
		t.Fatalf("proof attach refcount %d, want %d", attachedCount, proofCount+1)
	}
	ruleMust(t, rule.SetProof(proofQ))
	releasedCount, err := f.manager.GetRefCount(proof)
	ruleMust(t, err)
	if releasedCount != proofCount {
		t.Fatalf("replaced proof refcount %d, want %d", releasedCount, proofCount)
	}
	attachedQCount, err := f.manager.GetRefCount(proofQ)
	ruleMust(t, err)
	if attachedQCount != proofQCount+1 {
		t.Fatalf("replacement proof refcount %d, want %d", attachedQCount, proofQCount+1)
	}
	ruleMust(t, rule.SetProof(ast.ASTHandle{}))
	releasedQCount, err := f.manager.GetRefCount(proofQ)
	ruleMust(t, err)
	if releasedQCount != proofQCount {
		t.Fatalf("cleared proof refcount %d, want %d", releasedQCount, proofQCount)
	}
	if got, found, getErr := rule.Proof(); getErr != nil || found || !got.IsNull() {
		t.Fatal("proof clear", got, found, getErr)
	}
}

func TestRuleRewriteProofNoOpsPreserveTargetState(t *testing.T) {
	f := newRuleSetFixture(t, "P")
	if err := f.manager.ToggleProofMode(ast.ASTProofEnabled); err != nil {
		t.Fatal(err)
	}
	oldRule, err := f.rules.NewRule(f.app["P"], nil, nil, "without-proof")
	ruleMust(t, err)
	emptyTarget, err := f.rules.NewRule(f.app["P"], nil, nil, "empty-target")
	ruleMust(t, err)
	provedTarget, err := f.rules.NewRule(f.app["P"], nil, nil, "proved-target")
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, oldRule.Close()) })
	t.Cleanup(func() { ruleMust(t, emptyTarget.Close()) })
	t.Cleanup(func() { ruleMust(t, provedTarget.Close()) })
	if err = f.rules.MkRuleRewriteProof(oldRule, emptyTarget); err != nil {
		t.Fatal(err)
	}
	if _, found, getErr := emptyTarget.Proof(); getErr != nil || found {
		t.Fatal("missing source proof changed the target", found, getErr)
	}
	existing, err := f.manager.MkProofAsserted(f.app["P"])
	ruleMust(t, err)
	ruleMust(t, provedTarget.SetProof(existing))
	before, found, err := provedTarget.Proof()
	if err != nil || !found {
		t.Fatal("target proof setup", found, err)
	}
	if err = f.rules.MkRuleRewriteProof(oldRule, provedTarget); err != nil {
		t.Fatal(err)
	}
	after, found, err := provedTarget.Proof()
	if err != nil || !found {
		t.Fatal("target proof was removed", found, err)
	}
	same, err := f.manager.AreEqual(before, after)
	ruleMust(t, err)
	if !same {
		t.Fatal("existing target proof was replaced despite the source no-op condition")
	}
	if err = f.rules.MkRuleRewriteProof(provedTarget, provedTarget); err != nil {
		t.Fatal(err)
	}
	sameRuleProof, found, err := provedTarget.Proof()
	if err != nil || !found {
		t.Fatal("same-rule no-op lost its proof", found, err)
	}
	same, err = f.manager.AreEqual(after, sameRuleProof)
	ruleMust(t, err)
	if !same {
		t.Fatal("same-rule no-op replaced its proof")
	}
}
