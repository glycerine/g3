package fp

// Source: src/muz/base/dl_context.cpp:153-195. The trail undoes background
// assertions, raw formulas, and then rules. A returning Go cleanup failure
// keeps the current trail step and its owners reachable until retry.

import "github.com/glycerine/g3/pkg/ast"

type contextPopPhase uint8

const (
	contextPopNone contextPopPhase = iota
	contextPopAssertions
	contextPopRawRules
	contextPopRules
	contextPopSnapshot
)

func (e *PropositionalEngine) resumeContextPop() error {
	last := len(e.scopes) - 1
	if last < 0 || len(e.rawRuleScopes) != last+1 || len(e.assertionScopes) != last+1 {
		return ErrPropositionalEngine
	}
	if e.popPhase == contextPopAssertions {
		for len(e.assertions) > e.assertionScopes[last] {
			i := len(e.assertions) - 1
			err := e.manager.DelRef(&e.assertions[i])
			if e.assertions[i] == (ast.ASTRef{}) {
				e.assertions = e.assertions[:i]
			}
			if err != nil {
				return err
			}
		}
		e.retiredRawRules = append(e.retiredRawRules, e.rawRules...)
		e.rawRules = e.rawRuleScopes[last]
		e.rawRuleScopes[last] = nil
		e.popPhase = contextPopRawRules
	}
	if e.popPhase == contextPopRawRules {
		if err := e.cleanupRetiredRawRules(); err != nil {
			return err
		}
		e.popPhase = contextPopRules
	}
	if e.popPhase == contextPopRules {
		if err := e.set.ReplaceRules(e.scopes[last]); err != nil {
			return err
		}
		e.popPhase = contextPopSnapshot
	}
	if e.popPhase != contextPopSnapshot {
		return ErrPropositionalEngine
	}
	if err := e.scopes[last].Close(); err != nil {
		return err
	}
	e.scopes[last] = nil
	e.scopes = e.scopes[:last]
	e.rawRuleScopes = e.rawRuleScopes[:last]
	e.assertionScopes = e.assertionScopes[:last]
	e.popPhase = contextPopNone
	return nil
}
