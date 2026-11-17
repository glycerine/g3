package fp

// Source: dl_context.cpp add_rule. Go-only query clones (and the clone's
// query rule) need their rules visible to the clone's own rule-set readers at
// once; they get the eager clauses, and the clone's query flush then
// rebuilds them from the raw formulas as for the parent.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func (e *PropositionalEngine) addRuleNow(formula ast.ASTHandle, name string) error {
	if err := e.AddRule(formula, name); err != nil {
		return err
	}
	return e.materializePendingRules()
}

// materializePendingRules builds the Go-only eager clauses of queued rules
// for readers outside the query flush (rule enumeration, display, facts,
// covers). The query path instead runs flush_add_rules from the raw formulas.
func (e *PropositionalEngine) materializePendingRules() error {
	for i := range e.rawRules {
		if !e.rawRules[i].pending || e.rawRules[i].materialized {
			continue
		}
		if err := e.materializePendingRule(i); err != nil {
			return err
		}
	}
	return nil
}

func (e *PropositionalEngine) materializePendingRule(index int) (resultErr error) {
	raw := &e.rawRules[index]
	finalized, err := e.set.IsFinalized()
	if err != nil {
		return err
	}
	if finalized {
		if err = e.set.Reopen(); err != nil {
			return err
		}
	}
	first, err := e.set.Len()
	if err != nil {
		return err
	}
	formula, err := e.manager.RefGet(raw.formula)
	if err != nil {
		return err
	}
	bound, err := e.binder.Bind(formula, true)
	if err != nil {
		return err
	}
	boundFormula, getErr := e.manager.RefGet(bound)
	if getErr == nil {
		getErr = e.addFormula(e.set, boundFormula, raw.name)
	}
	deferHead := onlyInvalidRuleHead(getErr)
	end := first
	if getErr == nil || deferHead {
		var lenErr error
		end, lenErr = e.set.Len()
		if lenErr != nil {
			getErr = errors.Join(getErr, lenErr)
			deferHead = false
		}
	}
	cleanupErr := e.manager.DelRef(&bound)
	if cleanupErr != nil {
		deferHead = false
	}
	if getErr == nil || deferHead {
		// AddRule already advanced the rule revision; building the eager
		// view of a queued rule is not a new mutation.
		raw.first, raw.end, raw.materialized = first, end, true
		getErr = nil
	}
	return errors.Join(getErr, cleanupErr)
}

// AddRuleObject is context::add_rule(rule_ref&) for a rule built by
// rule_manager::mk(head, n, body, is_neg) (the datalog parser's add_rule):
// the rule goes straight into m_rule_set, keeping the body order and
// without mk_rule's label removal and Horn normalization. Queued formulas
// (m_rule_fmls) are still flushed after it at the next query.
func (e *PropositionalEngine) AddRuleObject(head ast.ASTHandle, body []ast.ASTHandle, isNeg []bool) (resultErr error) {
	if err := e.open(); err != nil {
		return err
	}
	// release-build rule_manager::mk: check_valid_rule is DEBUG_CODE.
	rule, err := e.rules.NewRuleWithSymbolUnchecked(head, body, isNeg, symbol.Symbol{})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, rule.Close()) }()
	finalized, err := e.set.IsFinalized()
	if err != nil {
		return err
	}
	if finalized {
		if err = e.set.Reopen(); err != nil {
			return err
		}
	}
	if err = e.set.Add(rule); err != nil {
		return err
	}
	// Queued formulas are flushed after the rules already in the set.
	for i := range e.rawRules {
		if e.rawRules[i].pending && !e.rawRules[i].materialized {
			e.rawRules[i].first++
			e.rawRules[i].end++
		}
	}
	e.advanceRuleRevision()
	return nil
}
