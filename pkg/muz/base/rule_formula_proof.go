package base

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// NewRuleFromFormulaWithProof constructs an already-Horn rule and transfers a
// proof of the input formula to the rule formula, matching mk_horn_rule's
// rewrite and modus-ponens path. General formulas requiring HNF definitions
// remain the responsibility of AddFormula.
func (m *RuleManager) NewRuleFromFormulaWithProof(formula, proof ast.ASTHandle, name string) (result *Rule, resultErr error) {
	if proof.IsNull() {
		return m.NewRuleFromFormula(formula, name)
	}
	rule, err := m.NewRuleFromFormula(formula, name)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, rule.Close())
			result = nil
		}
	}()
	oldFact, err := m.manager.GetProofFact(proof)
	if err != nil {
		return nil, err
	}
	newFormula, err := rule.ToFormula()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.manager.DelRef(&newFormula)) }()
	newFact, err := m.manager.RefGet(newFormula)
	if err != nil {
		return nil, err
	}
	if oldFact == newFact {
		if err = rule.SetProof(proof); err != nil {
			return nil, err
		}
		result = rule
		return rule, nil
	}
	mode, err := m.manager.ProofMode()
	if err != nil {
		return nil, err
	}
	if mode == ast.ASTProofDisabled {
		if err = m.manager.ToggleProofMode(ast.ASTProofEnabled); err != nil {
			return nil, err
		}
		defer func() { resultErr = errors.Join(resultErr, m.manager.ToggleProofMode(mode)) }()
	}
	rewrite, err := m.manager.MkProofRewrite(oldFact, newFact)
	if err != nil {
		return nil, err
	}
	translated, err := m.manager.MkProofModusPonens(proof, rewrite)
	if err != nil {
		return nil, err
	}
	if err = rule.SetProof(translated); err != nil {
		return nil, err
	}
	result = rule
	return rule, nil
}
