package fp

// Source: dl_context.cpp:479-491 appends normalized clauses before rejecting
// an update whose formula did not produce exactly one rule.

import "github.com/glycerine/g3/pkg/muz/base"

// publishRejectedUpdate inserts completed clauses into the normalized prefix.
// Go keeps eager copies of pending inputs in the same set, so they move after
// the new clauses while their raw formula/name/bound archive stays unchanged.
// The persistent RuleSet and cached query state keep their source identities.
func (e *PropositionalEngine) publishRejectedUpdate(clauses *base.RuleSet) (resultErr error) {
	added, err := clauses.Len()
	if err != nil || added == 0 {
		return err
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
	attempt := &contextFormulaAttempt{manager: e.manager, sets: []*base.RuleSet{replacement}}
	defer func() { resultErr = attempt.finish(resultErr) }()
	appendRule := func(set *base.RuleSet, index int) error {
		rule, err := set.Rule(index)
		if rule != nil {
			attempt.rules = append(attempt.rules, rule)
		}
		if err != nil {
			return err
		}
		return replacement.Add(rule)
	}
	for i := 0; i < count; i++ {
		if !pending[i] {
			if err = appendRule(e.set, i); err != nil {
				return err
			}
		}
	}
	for i := 0; i < added; i++ {
		if err = appendRule(clauses, i); err != nil {
			return err
		}
	}
	for i := 0; i < count; i++ {
		if pending[i] {
			if err = appendRule(e.set, i); err != nil {
				return err
			}
		}
	}
	if err = replacement.InheritPredicates(e.set); err != nil {
		return err
	}
	if err = attempt.closeRules(); err != nil {
		return err
	}
	if err = e.set.ExchangeContents(replacement); err != nil {
		return err
	}
	for i := range e.rawRules {
		if e.rawRules[i].pending {
			e.rawRules[i].first += added
			e.rawRules[i].end += added
		}
	}
	// Source update_rule leaves prior answers and engine state available.
	// The next query consumes the changed persistent set through its normal
	// rule preparation; no update-only raw formula is added to the archive.
	return nil
}
