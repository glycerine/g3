package interp

// Source checkpoint: src/interp/iz3translate.cpp arithmetic normalization
// helpers used by retained proof-rule translation.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (t *FullTranslation) ownedSimplify(term ast.ASTHandle) (ast.ASTHandle, error) {
	ref, err := t.base.manager.Simplify(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := t.base.manager.ast.RefGet(ref)
	if err != nil {
		return ast.ASTHandle{}, errors.Join(err, t.base.manager.ast.DelRef(&ref))
	}
	t.owned = append(t.owned, ref)
	return result, nil
}

func (t *FullTranslation) ownedIntegerDivision(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	ref, err := t.base.manager.MakeIDiv(left, right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := t.base.manager.ast.RefGet(ref)
	if err != nil {
		return ast.ASTHandle{}, errors.Join(err, t.base.manager.ast.DelRef(&ref))
	}
	t.owned = append(t.owned, ref)
	return result, nil
}

// linearCoefficients returns rationals the caller releases.
func (t *FullTranslation) linearCoefficients(term ast.ASTHandle) (_ []unumeral.Rational, resultErr error) {
	op, err := t.base.manager.Operator(term)
	if err != nil {
		return nil, err
	}
	terms := []ast.ASTHandle{term}
	if op == OperatorPlus {
		terms, err = t.base.manager.Args(term)
		if err != nil {
			return nil, err
		}
	}
	result := newRationals(len(terms))
	for i, value := range terms {
		if e := t.base.manager.Coefficient(value, &result[i]); e != nil {
			return nil, errors.Join(e, t.base.manager.releaseRationals(result))
		}
	}
	return result, nil
}

// rationalGCD sets value to gcd(value, other): the gcd of the numerators'
// absolute values over the lcm of the denominators.
func (t *FullTranslation) rationalGCD(value, other *unumeral.Rational) (err error) {
	q := t.base.manager.Rationals()
	parts := newRationals(3)
	defer func() { err = errors.Join(err, t.base.manager.releaseRationals(parts)) }()
	numerator, otherPart, denominator := &parts[0], &parts[1], &parts[2]
	err = errors.Join(q.NumeratorRational(value, numerator), q.NumeratorRational(other, otherPart),
		q.GCD(numerator, otherPart, numerator), q.Abs(numerator))
	if err == nil {
		err = errors.Join(q.DenominatorRational(value, denominator), q.DenominatorRational(other, otherPart),
			q.LCM(denominator, otherPart, denominator), q.Div(numerator, denominator, value))
	}
	return err
}

func (t *FullTranslation) GCDOfCoefficients(term ast.ASTHandle) (ast.ASTHandle, error) {
	if err := t.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	coefficients, err := t.linearCoefficients(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var result ast.ASTHandle
	err = t.base.manager.withRational(func(value *unumeral.Rational) error {
		q := t.base.manager.Rationals()
		if err := q.SetInt32(value, 1); err != nil {
			return err
		}
		if len(coefficients) != 0 {
			if err := errors.Join(q.Copy(value, &coefficients[0]), q.Abs(value)); err != nil {
				return err
			}
			for i := 1; i < len(coefficients); i++ {
				if err := t.rationalGCD(value, &coefficients[i]); err != nil {
					return err
				}
			}
		}
		var err error
		result, err = t.base.manager.MakeInt(value)
		return err
	})
	return result, errors.Join(err, t.base.manager.releaseRationals(coefficients))
}

func (t *FullTranslation) BoundedVariable(inequality ast.ASTHandle) (variable ast.ASTHandle, lower bool, err error) {
	normalized, err := t.NormalizeInequality(inequality)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	left, err := t.base.manager.Arg(normalized, 0)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	op, err := t.base.manager.Operator(left)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if op == OperatorUninterpreted {
		return left, false, nil
	}
	if op != OperatorTimes {
		return ast.ASTHandle{}, false, ProofError{}
	}
	coefficientTerm, err := t.base.manager.Arg(left, 0)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	var one, minusOne bool
	numeral := false
	err = t.base.manager.withRational(func(coefficient *unumeral.Rational) error {
		var e error
		numeral, e = t.base.manager.Numeral(coefficientTerm, coefficient)
		one, minusOne = coefficient.IsOne(), coefficient.IsMinusOne()
		return e
	})
	if err != nil || !numeral {
		return ast.ASTHandle{}, false, ProofError{}
	}
	switch {
	case one:
		lower = false
	default:
		if !minusOne {
			return ast.ASTHandle{}, false, ProofError{}
		}
		lower = true
	}
	variable, err = t.base.manager.Arg(left, 1)
	return variable, lower, err
}

// TermCoefficient is get_term_coefficient into out (a rational of
// Rationals()).
func (t *FullTranslation) TermCoefficient(inequality, variable ast.ASTHandle, out *unumeral.Rational) error {
	normalized, err := t.NormalizeInequality(inequality)
	if err != nil {
		return err
	}
	left, err := t.base.manager.Arg(normalized, 0)
	if err != nil {
		return err
	}
	terms := []ast.ASTHandle{left}
	if op, e := t.base.manager.Operator(left); e != nil {
		return e
	} else if op == OperatorPlus {
		terms, err = t.base.manager.Args(left)
		if err != nil {
			return err
		}
	}
	for _, term := range terms {
		candidate, e := t.base.manager.LinearVariable(term)
		if e != nil {
			return e
		}
		if candidate == variable {
			return t.base.manager.Coefficient(term, out)
		}
	}
	return t.base.manager.Rationals().SetInt32(out, 0)
}

// firstCoefficient writes the coefficient into coefficient (zero when the
// term has no variable).
func (t *FullTranslation) firstCoefficient(term ast.ASTHandle, coefficient *unumeral.Rational) (ast.ASTHandle, error) {
	terms := []ast.ASTHandle{term}
	if op, err := t.base.manager.Operator(term); err != nil {
		return ast.ASTHandle{}, err
	} else if op == OperatorPlus {
		terms, err = t.base.manager.Args(term)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	var best ast.ASTHandle
	var bestID uint64 = ^uint64(0)
	if err := t.base.manager.Rationals().SetInt32(coefficient, 0); err != nil {
		return ast.ASTHandle{}, err
	}
	for _, value := range terms {
		if numeral, err := t.base.manager.IsNumeral(value); err != nil {
			return ast.ASTHandle{}, err
		} else if numeral {
			continue
		}
		variable, err := t.base.manager.LinearVariable(value)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		id, err := t.base.manager.ASTID(variable)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if uint64(id) < bestID {
			best, bestID = variable, uint64(id)
			if err = t.base.manager.Coefficient(value, coefficient); err != nil {
				return ast.ASTHandle{}, err
			}
		}
	}
	return best, nil
}

func (t *FullTranslation) DivideInequalities(first, second ast.ASTHandle) (_ ast.ASTHandle, resultErr error) {
	firstLeft, err := t.base.manager.Arg(first, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	secondLeft, err := t.base.manager.Arg(second, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	// rational coeff1, coeff2, ratio
	values := newRationals(2)
	defer func() { resultErr = errors.Join(resultErr, t.base.manager.releaseRationals(values)) }()
	firstCoefficient, secondCoefficient := &values[0], &values[1]
	firstVariable, err := t.firstCoefficient(firstLeft, firstCoefficient)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	secondVariable, err := t.firstCoefficient(secondLeft, secondCoefficient)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if firstCoefficient.IsZero() || secondCoefficient.IsZero() || firstVariable != secondVariable {
		return ast.ASTHandle{}, ProofError{}
	}
	if err = t.base.manager.Rationals().Div(firstCoefficient, secondCoefficient, firstCoefficient); err != nil {
		return ast.ASTHandle{}, err
	}
	if !firstCoefficient.IsInt() {
		return ast.ASTHandle{}, ProofError{}
	}
	return t.base.manager.MakeInt(firstCoefficient)
}

func (t *FullTranslation) RHSNormalizeInequality(inequality ast.ASTHandle) (ast.ASTHandle, error) {
	if err := t.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	zero, err := t.base.manager.MakeIntText("0")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := t.base.manager.Make(OperatorLE, zero, zero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	one, err := t.base.manager.MakeIntText("1")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err = t.base.manager.LinearCombination(result, one, inequality, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return t.base.manager.SimplifyInequality(result)
}

func (t *FullTranslation) CheckFarkas(premises []ast.ASTHandle, conclusion ast.ASTHandle) (bool, error) {
	if err := t.open(); err != nil {
		return false, err
	}
	one, err := t.base.manager.MakeIntText("1")
	if err != nil {
		return false, err
	}
	negativeOne, err := t.base.manager.MakeIntText("-1")
	if err != nil {
		return false, err
	}
	coefficients := make([]ast.ASTHandle, len(premises)+1)
	inequalities := make([]ast.ASTHandle, len(premises)+1)
	for i := range premises {
		coefficients[i] = one
		inequalities[i] = premises[i]
	}
	coefficients[len(premises)] = negativeOne
	inequalities[len(premises)] = conclusion
	combined, err := t.base.manager.SumInequalities(coefficients, inequalities, false)
	if err != nil {
		return false, err
	}
	right, err := t.base.manager.Arg(combined, 1)
	if err != nil {
		return false, err
	}
	right, err = t.ownedSimplify(right)
	if err != nil {
		return false, err
	}
	return t.base.manager.NumeralIsZero(right)
}

func (t *FullTranslation) NormalizeInequality(inequality ast.ASTHandle) (ast.ASTHandle, error) {
	if err := t.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := t.RHSNormalizeInequality(inequality)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	op, err := t.base.manager.Operator(result)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, err := t.base.manager.Arg(result, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := t.base.manager.Arg(result, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	numeral, err := t.base.manager.IsNumeral(right)
	if err != nil || numeral {
		return result, err
	}
	zero, err := t.base.manager.MakeIntText("0")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightOp, err := t.base.manager.Operator(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if rightOp == OperatorPlus {
		arguments, e := t.base.manager.Args(right)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		constant := zero
		firstNonconstant := 0
		if len(arguments) != 0 {
			if matched, e := t.base.manager.IsNumeral(arguments[0]); e != nil {
				return ast.ASTHandle{}, e
			} else if matched {
				constant = arguments[0]
				firstNonconstant = 1
			}
		}
		if firstNonconstant < len(arguments) {
			nonconstant, e := t.base.manager.Make(OperatorPlus, arguments[firstNonconstant:]...)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			left, e = t.base.manager.Make(OperatorSub, left, nonconstant)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		right = constant
	} else {
		positive, e := t.base.manager.Make(OperatorPlus, right)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		left, e = t.base.manager.Make(OperatorSub, left, positive)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		right = zero
	}
	left, err = t.ownedSimplify(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err = t.ownedSimplify(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return t.base.manager.Make(op, left, right)
}
