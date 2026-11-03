package fp

// Source: muz/base/dl_context.cpp, m_rule_fmls/get_raw_rule_formulas and trails.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type rawRuleFormula struct {
	formula ast.ASTRef
	name    symbol.Symbol
	bound   uint32
	pending bool
	// Half-open origin range in the eager rule set, meaningful only while pending.
	// Query flush precedes transforms; fresh updates shift the pending suffix.
	first, end int
}

// RawRules returns owning, universally bound input formulas in source archive order,
// with their original names and recursion bounds. They precede normalization
// into the rule set used by other fixedpoint backends.
func (e *PropositionalEngine) RawRules() (forms []ast.ASTRef, names []string, bounds []uint32, resultErr error) {
	forms, labels, bounds, err := e.RawRulesWithSymbols()
	if err != nil {
		return nil, nil, nil, err
	}
	names, err = e.ruleNameStrings(labels)
	if err != nil {
		a := &contextFormulaAttempt{manager: e.manager, references: forms}
		return nil, nil, nil, a.finish(err)
	}
	return forms, names, bounds, nil
}

// RawRulesWithSymbols returns owning bound formulas and the original source
// symbols and recursion bounds, without projecting label identities to text.
func (e *PropositionalEngine) RawRulesWithSymbols() (forms []ast.ASTRef, names []symbol.Symbol, bounds []uint32, resultErr error) {
	if err := e.open(); err != nil {
		return nil, nil, nil, err
	}
	var acquired []ast.ASTRef
	defer func() {
		if resultErr != nil {
			for _, ref := range acquired {
				e.retiredRawRules = append(e.retiredRawRules, rawRuleFormula{formula: ref})
			}
			resultErr = errors.Join(resultErr, e.cleanupRetiredRawRules())
		}
	}()
	for _, rule := range e.rawRules {
		h, err := e.manager.RefGet(rule.formula)
		if err != nil {
			return nil, nil, nil, err
		}
		bound, err := e.binder.Bind(h, true)
		if bound != (ast.ASTRef{}) {
			acquired = append(acquired, bound)
		}
		if err != nil {
			return nil, nil, nil, err
		}
		names = append(names, rule.name)
		bounds = append(bounds, rule.bound)
	}
	return acquired, names, bounds, nil
}

func (e *PropositionalEngine) copyRawRules() (out []rawRuleFormula, result error) {
	var acquired []rawRuleFormula
	defer func() {
		if result != nil {
			e.retiredRawRules = append(e.retiredRawRules, acquired...)
			result = errors.Join(result, e.cleanupRetiredRawRules())
		}
	}()
	for _, rule := range e.rawRules {
		copy, err := e.manager.CopyRef(rule.formula)
		if err != nil {
			return nil, err
		}
		rule.formula = copy
		acquired = append(acquired, rule)
	}
	return acquired, nil
}

func (e *PropositionalEngine) cleanupRetiredRawRules() error {
	for len(e.retiredRawRules) > 0 {
		i := len(e.retiredRawRules) - 1
		err := e.manager.DelRef(&e.retiredRawRules[i].formula)
		if e.retiredRawRules[i].formula == (ast.ASTRef{}) {
			e.retiredRawRules = e.retiredRawRules[:i]
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// flushPendingRules rebuilds the pending source clauses under the current
// context parameters before query validation and backend construction.
func (e *PropositionalEngine) flushPendingRules() error {
	return e.normalizePendingRules(true)
}

// flushRawDisplayRules records a successfully reconstructed source boundary.
// The original archive remains available to Duality and scoped restoration.
func (e *PropositionalEngine) flushRawDisplayRules() {
	for i := range e.rawRules {
		e.rawRules[i].pending = false
	}
}
