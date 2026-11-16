package fp

import "github.com/glycerine/g3/pkg/muz/base"

// Arithmetic stores borrow the current source program. Retire their old
// result before committing a replacement, keeping facts independently owned.
func (e *PropositionalEngine) rebindArithmeticRules(replacement *base.RuleSet) error {
	if e.karrDatalog == nil && e.boundDatalog == nil {
		return nil
	}
	e.invalidateQueryForMutation()
	if e.karrDatalog != nil {
		e.karrDatalog.rules = replacement
		e.karrDatalog.results = nil
	}
	if e.boundDatalog != nil {
		e.boundDatalog.rules = replacement
		e.boundDatalog.evaluated = nil
	}
	return nil
}
