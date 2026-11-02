package fp

// Source: src/muz/base/dl_context.cpp get_rules_as_formulas and
// src/api/api_datalog.cpp Z3_fixedpoint_get_rules.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// contextFormulaRules returns a borrowed persistent set or a caller-owned
// snapshot excluding pending closed inputs. Pending handles remain borrowed.
func (e *PropositionalEngine) contextFormulaRules() (_ *base.RuleSet, pending []ast.ASTHandle, names []string, resultErr error) {
	if err := e.normalizePendingOpenRules(); err != nil {
		return nil, nil, nil, err
	}
	count, err := e.set.Len()
	if err != nil {
		return nil, nil, nil, err
	}
	omit := make([]bool, count)
	for _, raw := range e.rawRules {
		if !raw.pending {
			continue
		}
		h, err := e.manager.RefGet(raw.formula)
		if err != nil {
			return nil, nil, nil, err
		}
		if raw.first < 0 || raw.end > count || raw.first > raw.end {
			return nil, nil, nil, ErrPropositionalEngine
		}
		for i := raw.first; i < raw.end; i++ {
			omit[i] = true
		}
		pending = append(pending, h)
		names = append(names, raw.name)
	}
	if len(pending) == 0 {
		return e.set, pending, names, nil
	}
	snapshot, err := base.NewRuleSet(e.rules)
	if err != nil {
		return nil, nil, nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, snapshot.Close())
		}
	}()
	if err = snapshot.InheritPredicates(e.set); err != nil {
		return nil, nil, nil, err
	}
	for i := 0; i < count; i++ {
		if omit[i] {
			continue
		}
		rule, err := e.set.Rule(i)
		if err != nil {
			return nil, nil, nil, err
		}
		err = errors.Join(snapshot.Add(rule), rule.Close())
		if err != nil {
			return nil, nil, nil, err
		}
	}
	return snapshot, pending, names, nil
}

// RuleFormulas returns the source public rule enumeration: normalized clauses,
// pending closed inputs, then negated output queries. The caller owns each ref.
func (e *PropositionalEngine) RuleFormulas() (result []ast.ASTRef, resultErr error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	rules, pending, _, err := e.contextFormulaRules()
	if err != nil {
		return nil, err
	}
	var queries []ast.ASTRef
	defer func() {
		if rules != e.set {
			resultErr = errors.Join(resultErr, rules.Close())
		}
		for i := range queries {
			if queries[i] != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, e.manager.DelRef(&queries[i]))
			}
		}
		if resultErr != nil {
			for i := range result {
				resultErr = errors.Join(resultErr, e.manager.DelRef(&result[i]))
			}
			result = nil
		}
	}()
	var names []string
	if err = rules.AppendDisplayFormulas(&result, &queries, &names); err != nil {
		return result, err
	}
	for _, formula := range pending {
		ref, err := e.manager.NewRef(formula)
		if err != nil {
			return result, err
		}
		result = append(result, ref)
	}
	for i, query := range queries {
		h, err := e.manager.RefGet(query)
		if err != nil {
			return result, err
		}
		negated, err := e.manager.MkNot(h)
		if err != nil {
			return result, err
		}
		if err = e.manager.SetRef(query, negated); err != nil {
			return result, err
		}
		result = append(result, query)
		queries[i] = ast.ASTRef{}
	}
	return result, nil
}
