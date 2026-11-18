package opt

// Source: src/opt/opt_context.cpp get_lower_as_num/get_upper_as_num for O_MAXSMT.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// SoftBounds returns caller-owned copies of the objective's soft bounds.
func (s *ArithmeticSolver) SoftBounds(result *ArithmeticResult, objective int) (lower, upper numeral.Rational, err error) {
	lower, upper = numeral.NewRational(), numeral.NewRational()
	if s == nil || result == nil || objective < 0 || objective >= len(s.objectives) || objective >= len(result.Values) || !s.objectives[objective].soft || !result.Values[objective].Soft {
		return lower, upper, ast.ErrASTHandle
	}
	value := &result.Values[objective]
	q := s.rationalManager()
	err = errors.Join(q.Copy(&lower, &value.SoftLower), q.Copy(&upper, &value.SoftUpper))
	return lower, upper, err
}
