package opt

// Source: src/opt/maxres.cpp new_assumption/add_soft.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// addAssumption borrows weight; m_asm2weight keeps its own copy.
func (e *maxResEngine) addAssumption(expression ast.ASTHandle, weight *numeral.Rational) error {
	if weight == nil || e.sign(weight) <= 0 {
		return errMaxResState
	}
	// new_assumption: IF_VERBOSE(13, "new assumption " << mk_pp(e, m) <<
	// " " << w) (AUDIT.md FUZZ-F5; g3trace builds only).
	if enabled, err := optProgressEnabled(13); err != nil {
		return err
	} else if enabled {
		text, err := e.solver.mkPPString(expression)
		if err != nil {
			return err
		}
		if err = optProgress(13, "new assumption "+text+" "+e.solver.ratText(weight)+"\n"); err != nil {
			return err
		}
	}
	// new_assumption: m_asm2weight.insert(e, w) replaces a retained weight
	// (AUDIT.md FUZZ_J-F2); m_asms.push_back(e); m_trail.push_back(e)
	if _, err := e.trail.hold(expression, nil); err != nil {
		return err
	}
	q := e.q()
	stored := e.weights[expression]
	if stored == nil {
		fresh := numeral.NewRational()
		stored = &fresh
	}
	if err := q.Copy(stored, weight); err != nil {
		return err
	}
	e.weights[expression] = stored
	e.active = append(e.active, expression)
	e.stats.Assumptions++
	return nil
}

// removeAssumption drops expression from m_asms and m_asm2weight; the
// caller owns the returned weight.
func (e *maxResEngine) removeAssumption(expression ast.ASTHandle) (*numeral.Rational, error) {
	weight := e.weights[expression]
	if weight == nil {
		return nil, errMaxResState
	}
	delete(e.weights, expression)
	for i, candidate := range e.active {
		if candidate == expression {
			e.active[i] = e.active[len(e.active)-1]
			e.active = e.active[:len(e.active)-1]
			return weight, nil
		}
	}
	return nil, errMaxResState
}
