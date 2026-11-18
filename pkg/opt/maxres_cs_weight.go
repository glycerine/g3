package opt

// Source: src/opt/maxres.cpp process_sat minimum correction weight.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// correctionWeight is process_sat's minimum weight over the correction
// set; the caller owns the result.
func (e *maxResEngine) correctionWeight(correction []ast.ASTHandle) (numeral.Rational, error) {
	answer := numeral.NewRational()
	if len(correction) == 0 {
		return answer, errMaxResState
	}
	minimum := e.weights[correction[0]]
	if minimum == nil || e.sign(minimum) <= 0 {
		return answer, errMaxResState
	}
	q := e.q()
	if err := q.Copy(&answer, minimum); err != nil {
		return answer, err
	}
	for _, expression := range correction[1:] {
		weight := e.weights[expression]
		if weight == nil || e.sign(weight) <= 0 {
			return answer, errMaxResState
		}
		if e.cmp(weight, &answer) < 0 {
			if err := q.Copy(&answer, weight); err != nil {
				return answer, err
			}
		}
	}
	return answer, e.ratErr
}
