package base

// Source: src/muz/base/dl_rule.cpp rule_manager::mk_rule_rewrite_proof and
// rule::get_proof/set_proof.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

// MkRuleRewriteProof transfers a proof of oldRule's formula to newRule by
// rewriting the proved fact to the new rule formula and applying modus ponens.
func (m *RuleManager) MkRuleRewriteProof(oldRule, newRule *Rule) (resultErr error) {
	if err := m.open(); err != nil {
		return err
	}
	if oldRule == nil || newRule == nil {
		return ErrRuleOwner
	}
	oldManager, err := oldRule.ASTManager()
	if err != nil {
		return err
	}
	newManager, err := newRule.ASTManager()
	if err != nil {
		return err
	}
	if oldManager != m.manager || newManager != m.manager {
		return fmt.Errorf("muz/base: rewrite-proof rules belong to another AST manager")
	}
	same, err := oldRule.Same(newRule)
	if err != nil || same {
		return err
	}
	oldProof, hasOldProof, err := oldRule.Proof()
	if err != nil || !hasOldProof {
		return err
	}
	_, hasNewProof, err := newRule.Proof()
	if err != nil || hasNewProof {
		return err
	}
	mode, err := m.manager.ProofMode()
	if err != nil {
		return err
	}
	if mode == ast.ASTProofDisabled {
		if err = m.manager.ToggleProofMode(ast.ASTProofEnabled); err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, m.manager.ToggleProofMode(mode)) }()
	}
	formula, err := newRule.ToFormula()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, m.manager.DelRef(&formula)) }()
	newFact, err := m.manager.RefGet(formula)
	if err != nil {
		return err
	}
	oldFact, err := m.manager.GetProofFact(oldProof)
	if err != nil {
		return err
	}
	rewriteProof, err := m.manager.MkProofRewrite(oldFact, newFact)
	if err != nil {
		return err
	}
	proof, err := m.manager.MkProofModusPonens(oldProof, rewriteProof)
	if err != nil {
		return err
	}
	return newRule.SetProof(proof)
}
