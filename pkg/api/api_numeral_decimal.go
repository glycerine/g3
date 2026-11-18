package api

// Source: src/api/api_numeral.cpp:231-263.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// NumeralDecimalString renders arithmetic rational and irrational algebraic
// numerals using the source decimal algorithms. Integer, bit-vector and
// finite-domain numerals keep their exact rational spelling.
func (c *SMTContext) NumeralDecimalString(value ast.ASTHandle, precision uint32) (result string, resultErr error) {
	manager, err := c.Manager()
	if err != nil {
		return "", err
	}
	// rational val;
	q := manager.NumeralService().Manager
	val := numeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&val)) }()
	if _, matched, readErr := manager.ArithmeticNumeralRational(value, &val); readErr != nil {
		return "", readErr
	} else if matched {
		return apiRationalDecimalString(q, &val, precision)
	}
	var number polynomial.Algebraic
	matched, err := manager.ArithmeticAlgebraicNumeralValue(value, &number)
	if err != nil {
		return "", err
	}
	if matched {
		return apiAlgebraicDecimalString(manager, &number, precision)
	}
	if matched, err := c.NumeralRational(value, &val); err != nil {
		return "", err
	} else if matched {
		return q.String(&val)
	}
	return "", errAPINumeralArgument
}
