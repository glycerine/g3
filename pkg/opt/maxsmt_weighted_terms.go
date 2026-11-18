package opt

// Source: src/opt/sortmax.cpp unsigned-weight input expansion.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// maxSMTWeightedTerm is one m_soft/m_weights entry of sortmax: weight is
// an owned rational; units is its get_unsigned() once unscaleTerms has
// checked is_unsigned().
type maxSMTWeightedTerm struct {
	expression ast.ASTHandle
	weight     numeral.Rational
	units      uint32
}

// maxSMTRationalTerms gives sortmax's m_soft/m_weights: the objective's
// terms with copies of their rational weights.
func maxSMTRationalTerms(manager *ast.ASTManager, objective *arithmeticObjective) ([]maxSMTWeightedTerm, error) {
	q := manager.NumeralService().Manager
	terms := make([]maxSMTWeightedTerm, len(objective.softTerms))
	for i := range terms {
		terms[i].weight = numeral.NewRational()
	}
	for i := range objective.softTerms {
		expression, err := manager.RefGet(objective.softTerms[i])
		if err == nil {
			err = q.Copy(&terms[i].weight, &objective.softWeights[i])
		}
		if err != nil {
			return nil, errors.Join(err, releaseMaxSMTTerms(q, terms))
		}
		terms[i].expression = expression
	}
	return terms, nil
}

func releaseMaxSMTTerms(q *numeral.RationalManager, terms []maxSMTWeightedTerm) (err error) {
	for i := range terms {
		err = errors.Join(err, q.Del(&terms[i].weight))
	}
	return err
}

// maxSMTUnsigned is rational::is_unsigned/get_unsigned.
func maxSMTUnsigned(q *numeral.RationalManager, weight *numeral.Rational) (uint32, bool, error) {
	ok, err := q.IsUnsigned(weight)
	if err != nil || !ok {
		return 0, false, err
	}
	value, err := q.Unsigned(weight)
	return value, err == nil, err
}

func maxSMTExpandTerms(terms []maxSMTWeightedTerm, capacity int) []ast.ASTHandle {
	inputs := make([]ast.ASTHandle, 0, capacity)
	for i := range terms {
		for count := terms[i].units; count > 0; count-- {
			inputs = append(inputs, terms[i].expression)
		}
	}
	return inputs
}
