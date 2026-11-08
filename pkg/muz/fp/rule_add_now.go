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

func (e *PropositionalEngine) addRuleWithSymbolNow(formula ast.ASTHandle, name symbol.Symbol) error {
	if err := e.AddRuleWithSymbol(formula, name); err != nil {
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
