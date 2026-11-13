package fp

// Source: spacer_dl_interface.cpp saves old_rules and performs query
// preprocessing on a private rule-set copy.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (e *PropositionalEngine) copyQueryRules(clone *PropositionalEngine) (resultErr error) {
	if e.backend != "datalog" {
		return e.copyQueryRuleObjects(clone)
	}
	formulas, names, err := e.NamedRulesWithSymbols()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, closeASTRefs(e.manager, formulas)) }()
	for i := range formulas {
		formula, x := e.manager.RefGet(formulas[i])
		if x != nil {
			return x
		}
		if x = clone.addRuleWithSymbolNow(formula, names[i]); x != nil {
			return x
		}
	}
	return nil
}

// copyQueryRuleObjects is the source rule_set copy (old_rules(rules0) and the
// engines' private rule sets): the clone shares the flushed rule objects,
// their proofs included, without rebuilding any rule formula.
func (e *PropositionalEngine) copyQueryRuleObjects(clone *PropositionalEngine) (resultErr error) {
	if err := e.materializePendingRules(); err != nil {
		return err
	}
	if err := e.open(); err != nil {
		return err
	}
	count, err := e.set.Len()
	if err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		rule, err := e.set.Rule(i)
		if err != nil {
			return err
		}
		err = clone.set.Add(rule)
		if err = errors.Join(err, rule.Close()); err != nil {
			return err
		}
	}
	if count != 0 {
		clone.advanceRuleRevision()
	}
	return nil
}

// mkQueryRule is the tail of rule_manager::mk_query: under the
// generate_proof_trace proof mode, mk_rule(rule_expr, asserted(rule_expr))
// writes the query rule straight into the rule set; no raw formula is
// queued for a later flush.
func (e *PropositionalEngine) mkQueryRule(formula ast.ASTHandle) (resultErr error) {
	proofOwner := &pendingFlushProofOwner{manager: e.manager}
	defer func() { resultErr = proofOwner.finish(resultErr) }()
	enabled, err := e.pendingProofEnabled()
	if err != nil {
		return err
	}
	mode := ast.ASTProofDisabled
	if enabled {
		mode = ast.ASTProofEnabled
	}
	proofOwner.scope, err = ast.NewScopedProofMode(e.manager, mode)
	if err != nil {
		return err
	}
	ref, err := e.manager.NewRef(formula)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&ref)) }()
	finalized, err := e.set.IsFinalized()
	if err != nil {
		return err
	}
	if finalized {
		if err = e.set.Reopen(); err != nil {
			return err
		}
	}
	err = e.flushPendingSourceRule(rawRuleFormula{formula: ref, bound: ^uint32(0)})
	e.advanceRuleRevision()
	return err
}
