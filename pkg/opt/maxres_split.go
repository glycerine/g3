package opt

// Source: src/opt/maxres.cpp remove_core and split_core.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// splitCore is remove_core(core) followed by split_core(core): the core
// leaves m_asms (remove_soft swaps with the last element), and members
// heavier than the minimum come back as fresh assumptions with the excess.
// The caller owns the returned rational.
func (e *maxResEngine) splitCore(core []ast.ASTHandle) (minimum numeral.Rational, err error) {
	minimum = numeral.NewRational()
	// remove_core(core): only m_asms loses the members. m_asm2weight keeps
	// every weight it was given, so a later core of the same batch that
	// shares a member with a correction set processed in between
	// (process_unsat's pivot) still finds its weight (AUDIT.md FUZZ_J-F2).
	e.active = removeSoft(core, e.active)
	// split_core: if (core.empty()) return rational(0);
	if len(core) == 0 {
		return minimum, nil
	}
	q := e.q()
	// vector<rational> weights; rational w = get_weight(core[0])
	weights := make([]numeral.Rational, len(core))
	for i := range weights {
		weights[i] = numeral.NewRational()
	}
	var excess numeral.Rational = numeral.NewRational()
	defer func() {
		err = errors.Join(err, q.Del(&excess))
		for i := range weights {
			err = errors.Join(err, q.Del(&weights[i]))
		}
	}()
	for i, expression := range core {
		weight := e.weights[expression]
		if weight == nil {
			return minimum, errMaxResState
		}
		if err = q.Copy(&weights[i], weight); err != nil {
			return minimum, err
		}
		if i == 0 || e.cmp(weight, &minimum) < 0 {
			if err = q.Copy(&minimum, weight); err != nil {
				return minimum, err
			}
		}
	}
	for i, expression := range core {
		if e.cmp(&weights[i], &minimum) > 0 {
			if err = q.Sub(&weights[i], &minimum, &excess); err != nil {
				return minimum, err
			}
			if err = e.addAssumption(expression, &excess); err != nil {
				return minimum, err
			}
		}
	}
	return minimum, e.ratErr
}
