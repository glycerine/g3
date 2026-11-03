package fp

// Source: dl_context.cpp context::get_rules_as_formulas, pending free-variable
// normalization and swap removal. The eager Go clauses for pending inputs
// are replaced by fresh normalization using the current context parameters.

import "github.com/glycerine/g3/pkg/muz/base"

func (e *PropositionalEngine) normalizePendingOpenRules() (resultErr error) {
	remaining := append([]rawRuleFormula(nil), e.rawRules...)
	var promoted []rawRuleFormula
	for i := 0; i < len(remaining); {
		raw := remaining[i]
		if !raw.pending {
			i++
			continue
		}
		h, err := e.manager.RefGet(raw.formula)
		if err != nil {
			return err
		}
		free, err := e.manager.HasFreeVars(h)
		if err != nil {
			return err
		}
		if !free {
			i++
			continue
		}
		promoted = append(promoted, raw)
		last := len(remaining) - 1
		remaining[i] = remaining[last]
		// The source swaps formulas and names but only pops the bounds vector.
		remaining[i].bound = raw.bound
		remaining = remaining[:last]
	}
	if len(promoted) == 0 {
		return nil
	}
	count, err := e.set.Len()
	if err != nil {
		return err
	}
	pending := make([]bool, count)
	for _, raw := range e.rawRules {
		if !raw.pending {
			continue
		}
		if raw.first < 0 || raw.end > count || raw.first > raw.end {
			return ErrPropositionalEngine
		}
		for i := raw.first; i < raw.end; i++ {
			pending[i] = true
		}
	}
	replacement, err := base.NewRuleSet(e.rules)
	if err != nil {
		return err
	}
	a := &contextFormulaAttempt{manager: e.manager, sets: []*base.RuleSet{replacement}}
	defer func() { resultErr = a.finish(resultErr) }()
	next := 0
	add := func(i int) error {
		rule, err := e.set.Rule(i)
		if rule != nil {
			a.rules = append(a.rules, rule)
		}
		if err != nil {
			return err
		}
		err = replacement.Add(rule)
		if err == nil {
			next++
		}
		return err
	}
	for i := 0; i < count; i++ {
		if !pending[i] {
			if err = add(i); err != nil {
				return err
			}
		}
	}
	for _, raw := range promoted {
		formula, err := e.manager.RefGet(raw.formula)
		if err != nil {
			return err
		}
		// Source get_rules_as_formulas invokes mk_rule now, rather than
		// reusing clauses built under the parameter values at add_rule.
		if err = e.addFormula(replacement, formula, raw.name); err != nil {
			return err
		}
		next, err = replacement.Len()
		if err != nil {
			return err
		}
	}
	for j := range remaining {
		raw := &remaining[j]
		if !raw.pending {
			continue
		}
		first := next
		for i := raw.first; i < raw.end; i++ {
			if err = add(i); err != nil {
				return err
			}
		}
		raw.first, raw.end = first, next
	}
	if err = replacement.InheritPredicates(e.set); err != nil {
		return err
	}
	if err = e.rebindArithmeticRules(replacement); err != nil {
		return err
	}
	if err = a.closeRules(); err != nil {
		return err
	}
	prior := e.set
	e.set = replacement
	e.rawRules = remaining
	e.retiredRawRules = append(e.retiredRawRules, promoted...)
	// The replacement is now persistent; this scope instead owns the old
	// set until its last reference releases have completed.
	a.sets[0] = prior
	e.advanceRuleRevision()
	return e.cleanupRetiredRawRules()
}
