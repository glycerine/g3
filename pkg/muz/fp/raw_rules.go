package fp

// Source: muz/base/dl_context.cpp, m_rule_fmls/get_raw_rule_formulas and trails.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type rawRuleFormula struct {
	formula ast.ASTRef
	name    string
	bound   uint32
}

// RawRules returns owning, universally bound input formulas in insertion order,
// with their original names and recursion bounds. They precede normalization
// into the rule set used by other fixedpoint backends.
func (e *PropositionalEngine) RawRules() (forms []ast.ASTRef, names []string, bounds []uint32, resultErr error) {
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
		if err != nil {
			return nil, nil, nil, err
		}
		acquired = append(acquired, bound)
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
		acquired = append(acquired, rawRuleFormula{copy, rule.name, rule.bound})
	}
	return acquired, nil
}

func (e *PropositionalEngine) replaceRawRule(name string, reference ast.ASTRef, replace bool) error {
	bound := ^uint32(0)
	kept := e.rawRules[:0]
	for _, rule := range e.rawRules {
		if replace && rule.name == name {
			bound = rule.bound
			e.retiredRawRules = append(e.retiredRawRules, rule)
		} else {
			kept = append(kept, rule)
		}
	}
	e.rawRules = append(kept, rawRuleFormula{reference, name, bound})
	return e.cleanupRetiredRawRules()
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
