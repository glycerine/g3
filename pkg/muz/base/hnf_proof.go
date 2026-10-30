package base

// Source: src/muz/base/hnf.cpp, the proof-carrying disjunctive antecedent
// split in hnf::imp::mk_horn.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// AddFormulaWithProof ingests a source formula and proof through the retained
// direct disjunctive split and HNF definition-introduction paths. Other HNF
// proof transformations return an error instead of attaching an unrelated proof.
func (m *RuleManager) AddFormulaWithProof(set *RuleSet, formula, proof ast.ASTHandle, name string) (resultErr error) {
	if proof.IsNull() {
		return m.AddFormula(set, formula, name)
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
	normalized, err := normalizer.Normalize(formula)
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
	clauseProofs, err := m.hnfClauseProofs(formula, proof, clauses, normalized.FreshPredicates)
	if err != nil {
		return err
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
		if e = set.Add(rule); e != nil {
			return errors.Join(e, rule.Close())
		}
		if e = rule.Close(); e != nil {
			return e
		}
	}
	return nil
}

func (m *RuleManager) hnfClauseProofs(formula, proof ast.ASTHandle, clauses []ast.ASTHandle, freshRefs []ast.ASTRef) ([]ast.ASTHandle, error) {
	proofs := make([]ast.ASTHandle, len(clauses))
	if len(freshRefs) == 0 {
		if !m.isDisjunctiveAntecedent(formula) {
			return nil, fmt.Errorf("muz/base: proof transfer for this HNF transformation is not implemented")
		}
		disjunction, err := m.antecedentDisjunction(formula)
		if err != nil {
			return nil, err
		}
		count, err := m.manager.GetNumArgs(disjunction)
		if err != nil {
			return nil, err
		}
		if uint32(len(clauses)) != count {
			return nil, fmt.Errorf("muz/base: HNF split produced %d clauses for %d disjuncts", len(clauses), count)
		}
		conjunction, err := m.manager.MkAndArgs(clauses)
		if err != nil {
			return nil, err
		}
		equivalence, err := m.manager.MkIff(formula, conjunction)
		if err != nil {
			return nil, err
		}
		definition, err := m.manager.MkProofDefAxiom(equivalence)
		if err != nil {
			return nil, err
		}
		conjunctionProof, err := m.manager.MkProofModusPonens(proof, definition)
		if err != nil {
			return nil, err
		}
		for i, clause := range clauses {
			proofs[i], err = m.manager.MkProofAndElim(conjunctionProof, uint32(i))
			if err != nil {
				return nil, err
			}
			fact, e := m.manager.GetProofFact(proofs[i])
			if e != nil {
				return nil, e
			}
			if fact != clause {
				return nil, fmt.Errorf("muz/base: HNF proof clause %d has the wrong fact", i)
			}
		}
		return proofs, nil
	}

	fresh := make(map[ast.ASTHandle]struct{}, len(freshRefs))
	for _, reference := range freshRefs {
		declaration, err := m.manager.RefGet(reference)
		if err != nil {
			return nil, err
		}
		fresh[declaration] = struct{}{}
	}
	main := -1
	var definitions []ast.ASTHandle
	for index, clause := range clauses {
		head, err := m.hnfFormulaHead(clause)
		if err != nil {
			return nil, err
		}
		declaration, err := m.manager.GetDecl(head)
		if err != nil {
			return nil, fmt.Errorf("muz/base: generated HNF clause has no predicate head")
		}
		if _, isDefinition := fresh[declaration]; isDefinition {
			proofs[index], err = m.manager.MkProofDefIntro(clause)
			if err != nil {
				return nil, err
			}
			definitions = append(definitions, proofs[index])
		} else if main < 0 {
			main = index
		} else {
			return nil, fmt.Errorf("muz/base: HNF proof transfer requires one main clause")
		}
	}
	if main < 0 || len(definitions) == 0 {
		return nil, fmt.Errorf("muz/base: unsupported HNF definition proof shape")
	}
	application, err := m.manager.MkProofApplyDefs(formula, clauses[main], definitions)
	if err != nil {
		return nil, err
	}
	proofs[main], err = m.manager.MkProofModusPonens(proof, application)
	if err != nil {
		return nil, err
	}
	return proofs, nil
}

func (m *RuleManager) hnfFormulaHead(formula ast.ASTHandle) (ast.ASTHandle, error) {
	for {
		kind, err := m.manager.GetKind(formula)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if kind != ast.ASTQuantifier {
			break
		}
		formula, err = m.manager.GetQuantifierBody(formula)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	kind, err := m.manager.GetKind(formula)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if kind == ast.ASTApp {
		operator, e := m.manager.GetDeclKind(formula)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if operator == ast.ASTOpImplies {
			return m.manager.GetArg(formula, 1)
		}
	}
	return formula, nil
}

func (m *RuleManager) isDisjunctiveAntecedent(formula ast.ASTHandle) bool {
	for {
		kind, err := m.manager.GetKind(formula)
		if err != nil || kind != ast.ASTQuantifier {
			break
		}
		forall, err := m.manager.IsForall(formula)
		if err != nil || !forall {
			return false
		}
		formula, err = m.manager.GetQuantifierBody(formula)
		if err != nil {
			return false
		}
	}
	kind, err := m.manager.GetKind(formula)
	if err != nil || kind != ast.ASTApp {
		return false
	}
	operator, err := m.manager.GetDeclKind(formula)
	if err != nil || operator != ast.ASTOpImplies {
		return false
	}
	antecedent, err := m.manager.GetArg(formula, 0)
	if err != nil {
		return false
	}
	kind, err = m.manager.GetKind(antecedent)
	if err != nil || kind != ast.ASTApp {
		return false
	}
	operator, err = m.manager.GetDeclKind(antecedent)
	if err != nil || operator != ast.ASTOpOr {
		return false
	}
	head, err := m.manager.GetArg(formula, 1)
	if err != nil {
		return false
	}
	predicate, err := m.isPredicate(head)
	return err == nil && predicate
}

func (m *RuleManager) antecedentDisjunction(formula ast.ASTHandle) (ast.ASTHandle, error) {
	for {
		kind, err := m.manager.GetKind(formula)
		if err != nil || kind != ast.ASTQuantifier {
			break
		}
		formula, err = m.manager.GetQuantifierBody(formula)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return m.manager.GetArg(formula, 0)
}
