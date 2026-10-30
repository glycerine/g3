package base

// Source: src/muz/base/hnf.cpp, the proof-carrying disjunctive antecedent
// split in hnf::imp::mk_horn.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/qe"
)

// AddFormulaWithProof ingests a formula and proof through the source Horn
// normalization worklist, carrying proofs through each transformation.
func (m *RuleManager) AddFormulaWithProof(set *RuleSet, formula, proof ast.ASTHandle, name string) (resultErr error) {
	return m.addFormulaWithProof(set, formula, proof, name, nil, false)
}

// AddFormulaWithProofAndUnboundFix applies the source post-ingestion rule
// rewrite after transferring the formula proof to each normalized rule.
func (m *RuleManager) AddFormulaWithProofAndUnboundFix(set *RuleSet, formula, proof ast.ASTHandle, name string, lite *qe.Lite, enabled bool) error {
	if lite == nil {
		return fmt.Errorf("muz/base: nil unbound-variable eliminator")
	}
	return m.addFormulaWithProof(set, formula, proof, name, lite, enabled)
}

func (m *RuleManager) addFormulaWithProof(set *RuleSet, formula, proof ast.ASTHandle, name string, lite *qe.Lite, fixUnbound bool) (resultErr error) {
	if proof.IsNull() {
		return m.addFormula(set, formula, name, lite, fixUnbound)
	}
	if err := m.open(); err != nil {
		return err
	}
	if set == nil || set.manager != m {
		return ErrRuleSetOwner
	}
	oldFact, err := m.manager.GetProofFact(proof)
	if err != nil {
		return err
	}
	if oldFact != formula {
		return fmt.Errorf("muz/base: HNF proof does not prove the supplied formula")
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
	formulaRef, err := m.manager.NewRef(formula)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, m.manager.DelRef(&formulaRef)) }()
	proofRef, err := m.manager.NewRef(proof)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, m.manager.DelRef(&proofRef)) }()
	labels, err := rewriter.NewLabelRewriter(m.manager)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, labels.Close()) }()
	if err = labels.RemoveLabels(formulaRef, proofRef); err != nil {
		return err
	}
	formula, err = m.manager.RefGet(formulaRef)
	if err != nil {
		return err
	}
	proof, err = m.manager.RefGet(proofRef)
	if err != nil {
		return err
	}
	normalizer, err := NewHornNormalizer(m.manager, name)
	if err != nil {
		return err
	}
	normalized, err := normalizer.NormalizeWithProof(formula, proof)
	if normalized != nil {
		defer func() { resultErr = errors.Join(resultErr, normalized.Close()) }()
	}
	if err != nil {
		return err
	}
	clauses := make([]ast.ASTHandle, len(normalized.Formulas))
	for i, ref := range normalized.Formulas {
		clauses[i], err = m.manager.RefGet(ref)
		if err != nil {
			return err
		}
	}
	clauseProofs := make([]ast.ASTHandle, len(normalized.Proofs))
	for i, ref := range normalized.Proofs {
		clauseProofs[i], err = m.manager.RefGet(ref)
		if err != nil {
			return err
		}
	}
	for _, reference := range normalized.FreshPredicates {
		declaration, e := m.manager.RefGet(reference)
		if e != nil {
			return e
		}
		if e = m.RegisterPredicate(declaration); e != nil {
			return e
		}
	}
	for i, clause := range clauses {
		clauseProof := clauseProofs[i]
		rule, e := m.NewRuleFromFormulaWithProof(clause, clauseProof, name)
		if e != nil {
			return e
		}
		if lite != nil {
			fixed, _, fixErr := m.FixUnboundVariables(rule, lite, fixUnbound)
			if fixErr != nil {
				return errors.Join(fixErr, rule.Close())
			}
			if fixErr = m.MkRuleRewriteProof(rule, fixed); fixErr != nil {
				return errors.Join(fixErr, fixed.Close(), rule.Close())
			}
			if fixErr = rule.Close(); fixErr != nil {
				return errors.Join(fixErr, fixed.Close())
			}
			rule = fixed
		}
		if e = set.Add(rule); e != nil {
			return errors.Join(e, rule.Close())
		}
		if e = rule.Close(); e != nil {
			return e
		}
	}
	return nil
}
