package fp

// Source: dl_context.cpp context::get_rules_as_formulas, pending free-variable
// normalization and swap removal. The eager Go clauses for pending inputs
// are replaced by fresh normalization using the current context parameters.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func (e *PropositionalEngine) normalizePendingOpenRules() error {
	return e.normalizePendingRules(false)
}

func (e *PropositionalEngine) normalizePendingRules(flush bool) (resultErr error) {
	remaining := append([]rawRuleFormula(nil), e.rawRules...)
	var promoted []rawRuleFormula
	for i := 0; i < len(remaining); {
		raw := remaining[i]
		if !raw.pending {
			i++
			continue
		}
		if flush {
			promoted = append(promoted, raw)
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
		// Source mk_rule binds registered variable constants before label
		// removal and Horn normalization, including during a pending flush.
		bound, err := e.binder.Bind(formula, true)
		if bound != (ast.ASTRef{}) {
			a.references = append(a.references, bound)
		}
		if err != nil {
			return err
		}
		formula, err = e.manager.RefGet(bound)
		if err != nil {
			return err
		}
		// Both source flush_add_rules and get_rules_as_formulas invoke
		// mk_rule now, using the current context parameter values.
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
		if flush || !raw.pending {
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
	if flush {
		// A query flush advances the source pending head but keeps the
		// original formula/name/bound archive for raw-rule consumers.
		e.flushRawDisplayRules()
	} else {
		e.retiredRawRules = append(e.retiredRawRules, promoted...)
	}
	// The replacement is now persistent; this scope instead owns the old
	// set until its last reference releases have completed.
	a.sets[0] = prior
	e.advanceRuleRevision()
	return e.cleanupRetiredRawRules()
}
