package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// arith_project_util::project, integer virtual substitution paths.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// legacyIntegerMod is out = mod(value, abs(modulus)) of integer rationals.
func legacyIntegerMod(q *unumeral.RationalManager, value, modulus, out *unumeral.Rational) (resultErr error) {
	if value == nil || modulus == nil {
		return ast.ErrASTDomain
	}
	for _, v := range []*unumeral.Rational{value, modulus} {
		if isInt, err := q.IsInt(v); err != nil || !isInt {
			return errors.Join(err, ast.ErrASTDomain)
		}
	}
	if zero, err := q.IsZero(modulus); err != nil || zero {
		return errors.Join(err, ast.ErrASTDomain)
	}
	m := unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&m)) }()
	if err := errors.Join(q.Copy(&m, modulus), q.Abs(&m)); err != nil {
		return err
	}
	return q.Mod(value, &m, out)
}

func (p *legacyArithmeticProjection) integerDivisibility(term ast.ASTHandle, divisor *unumeral.Rational) (ast.ASTRef, error) {
	modulus, err := p.syntax.manager.ArithmeticNumeralFromRational(divisor, true)
	if err != nil {
		return ast.ASTRef{}, err
	}
	modular, err := p.syntax.manager.MkFamilyApp(p.syntax.family, 13, nil, []ast.ASTHandle{term, modulus})
	if err != nil {
		return ast.ASTRef{}, err
	}
	zero, err := p.syntax.manager.ArithmeticIntegerNumeralText("0")
	if err != nil {
		return ast.ASTRef{}, err
	}
	equality, err := p.syntax.manager.MkEq(modular, zero)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return p.syntax.manager.NewRef(equality)
}

func (p *legacyArithmeticProjection) projectInteger(md *model.ModelCore, variable ast.ASTHandle, inputs []ast.ASTHandle) (result *legacyRealProjection, projected bool, resultErr error) {
	if p == nil || p.syntax == nil || md == nil || md.Manager() != p.syntax.manager {
		return nil, false, ast.ErrASTHandle
	}
	defer func() {
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
			projected = false
		}
	}()
	integer, arithmetic, err := p.syntax.arithmetic(variable)
	if err != nil {
		return nil, false, err
	}
	if !arithmetic || !integer {
		return nil, false, ast.ErrASTDomain
	}
	evaluator, err := newLegacyModelEvaluator(md, model.ModelEvaluatorConfig{
		Context: p.syntax.config.Context, Globals: p.syntax.config.Globals,
		Memory: p.syntax.manager.NumeralService().RuntimeOwner(), Cooperation: p.syntax.config.Cooperation, Trace: p.syntax.config.Trace,
	})
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	if err = evaluator.SetModelCompletion(true); err != nil {
		return nil, false, err
	}
	// rational values of the projection.
	q := p.q()
	variableValue, value, lcmCoefficients, lcmDivisors, xValue := unumeral.NewRational(), unumeral.NewRational(), unumeral.NewRational(), unumeral.NewRational(), unumeral.NewRational()
	defer func() {
		resultErr = errors.Join(resultErr, q.Del(&variableValue), q.Del(&value), q.Del(&lcmCoefficients), q.Del(&lcmDivisors), q.Del(&xValue))
	}()
	if err = p.evaluateRat(evaluator, variable, &variableValue); err != nil {
		return nil, false, err
	}
	literals := make([]*legacyLinearLiteral, 0, len(inputs))
	originals := make([]ast.ASTHandle, 0, len(inputs))
	defer func() { resultErr = errors.Join(resultErr, closeLegacyLinearLiterals(literals...)) }()
	equalityIndex := -1
	positive, negative := 0, 0
	for _, input := range inputs {
		dependent, err := p.contains(input, variable)
		if err != nil {
			return nil, false, err
		}
		if !dependent {
			continue
		}
		literal, err := p.linearLiteral(variable, input)
		if err != nil {
			return nil, false, err
		}
		if literal == nil {
			return nil, false, nil
		}
		// The source scoped literal vector owns a classified literal before
		// model evaluation and normalization can fail.
		literals = append(literals, literal)
		originals = append(originals, input)
		if zero, err := q.IsZero(&literal.coefficient); err != nil {
			return nil, false, err
		} else if zero {
			if err = literal.Close(); err != nil {
				return nil, false, err
			}
			literals = literals[:len(literals)-1]
			originals = originals[:len(originals)-1]
			continue
		}
		term, err := p.syntax.manager.RefGet(literal.term)
		if err != nil {
			return nil, false, err
		}
		coefficientTerm, err := p.multiplyLegacy(&literal.coefficient, variable)
		if err != nil {
			return nil, false, err
		}
		sum, err := p.syntax.manager.MkArithmeticAdd(coefficientTerm, term)
		if err != nil {
			return nil, false, err
		}
		if err = p.evaluateRat(evaluator, sum, &value); err != nil {
			return nil, false, err
		}
		valueSign, err := q.Sign(&value)
		if err != nil {
			return nil, false, err
		}
		if literal.equality {
			if equalityIndex < 0 && valueSign == 0 {
				equalityIndex = len(literals) - 1
			}
		} else {
			if literal.disequality {
				if valueSign > 0 {
					if err = q.Neg(&literal.coefficient); err != nil {
						return nil, false, err
					}
					negated, err := p.multiplyLegacyInt(-1, term)
					if err != nil {
						return nil, false, err
					}
					if err = p.replaceLegacyTerm(literal, negated); err != nil {
						return nil, false, err
					}
					valueSign = -valueSign
				}
				literal.strict = true
				literal.disequality = false
			}
			divisorZero, err := q.IsZero(&literal.divisor)
			if err != nil {
				return nil, false, err
			}
			if divisorZero && (valueSign < 0 || (!literal.strict && valueSign == 0)) {
				coefficientSign, err := p.sign(&literal.coefficient)
				if err != nil {
					return nil, false, err
				}
				if coefficientSign > 0 {
					positive++
				} else {
					negative++
				}
			}
		}
	}
	if len(literals) == 0 {
		return &legacyRealProjection{manager: p.syntax.manager}, true, nil
	}
	if err = p.normalizeIntegers(literals, &lcmCoefficients, &lcmDivisors); err != nil {
		return nil, false, err
	}
	attachDivisibility := func(result *legacyRealProjection, xTerm ast.ASTHandle) error {
		if one, err := q.IsOne(&lcmCoefficients); err != nil || one {
			return err
		}
		constraint, err := p.integerDivisibility(xTerm, &lcmCoefficients)
		if err == nil {
			result.divisibility = constraint
		}
		return err
	}
	if equalityIndex >= 0 {
		literal := literals[equalityIndex]
		xTerm, err := p.syntax.manager.RefGet(literal.term)
		if err != nil {
			return nil, false, err
		}
		if positive, e := q.IsPos(&literal.coefficient); e != nil {
			return nil, false, e
		} else if positive {
			xTerm, err = p.syntax.manager.MkArithmeticNeg(xTerm)
			if err != nil {
				return nil, false, err
			}
		}
		rewritten, err := p.rewriteLegacy(xTerm)
		if err != nil {
			return nil, false, err
		}
		xTerm, err = p.syntax.manager.RefGet(rewritten)
		if err != nil {
			_ = p.syntax.manager.DelRef(&rewritten)
			return nil, false, err
		}
		result, err = p.substituteIntegerLiterals(literals, originals, xTerm, equalityIndex)
		if err == nil {
			err = attachDivisibility(result, xTerm)
		}
		err = errors.Join(err, p.syntax.manager.DelRef(&rewritten))
		if err != nil {
			if result != nil {
				err = errors.Join(err, result.Close())
			}
			return nil, false, err
		}
		return result, true, nil
	}
	if positive == 0 || negative == 0 {
		if err = q.Mul(&lcmCoefficients, &variableValue, &xValue); err != nil {
			return nil, false, err
		}
		if err = legacyIntegerMod(q, &xValue, &lcmDivisors, &xValue); err != nil {
			return nil, false, err
		}
		xTerm, err := p.syntax.manager.ArithmeticNumeralFromRational(&xValue, true)
		if err != nil {
			return nil, false, err
		}
		result = &legacyRealProjection{manager: p.syntax.manager}
		for index, literal := range literals {
			var replacement ast.ASTRef
			divisorZero, err := q.IsZero(&literal.divisor)
			if err != nil {
				_ = result.Close()
				return nil, false, err
			}
			coefficientSign, err := p.sign(&literal.coefficient)
			if err != nil {
				_ = result.Close()
				return nil, false, err
			}
			if !divisorZero {
				term, err := p.syntax.manager.RefGet(literal.term)
				if err != nil {
					_ = result.Close()
					return nil, false, err
				}
				if coefficientSign > 0 {
					term, err = p.syntax.manager.MkArithmeticAdd(term, xTerm)
				} else {
					negated, negateErr := p.syntax.manager.MkArithmeticNeg(xTerm)
					if negateErr == nil {
						term, err = p.syntax.manager.MkArithmeticAdd(term, negated)
					} else {
						err = negateErr
					}
				}
				if err == nil {
					var rewritten ast.ASTRef
					rewritten, err = p.rewriteLegacy(term)
					if err == nil {
						term, err = p.syntax.manager.RefGet(rewritten)
					}
					if err == nil {
						replacement, err = p.integerDivisibility(term, &literal.divisor)
					}
					if rewritten != (ast.ASTRef{}) {
						err = errors.Join(err, p.syntax.manager.DelRef(&rewritten))
					}
				}
			} else {
				value := !(literal.equality || (positive == 0 && coefficientSign > 0) || (negative == 0 && coefficientSign < 0))
				replacement, err = p.constantLegacy(value)
			}
			if err != nil {
				err = errors.Join(err, result.Close())
				return nil, false, err
			}
			p.appendLegacyReplacement(result, originals[index], replacement)
		}
		return result, true, nil
	}
	usePositive := positive < negative
	selected, err := p.findMax(md, variable, literals, usePositive)
	if err != nil {
		return nil, false, err
	}
	selectedTerm, err := p.syntax.manager.RefGet(literals[selected].term)
	if err != nil {
		return nil, false, err
	}
	coefficientTerm, err := p.multiplyLegacy(&literals[selected].coefficient, variable)
	if err != nil {
		return nil, false, err
	}
	sum, err := p.syntax.manager.MkArithmeticAdd(coefficientTerm, selectedTerm)
	if err != nil {
		return nil, false, err
	}
	if err = p.evaluateRat(evaluator, sum, &value); err != nil {
		return nil, false, err
	}
	// offset = abs(value) (- 1 when strict) mod m_lcm_divs
	offset := &value
	if err = q.Abs(offset); err != nil {
		return nil, false, err
	}
	if literals[selected].strict {
		if err = q.Dec(offset); err != nil {
			return nil, false, err
		}
	}
	if err = legacyIntegerMod(q, offset, &lcmDivisors, offset); err != nil {
		return nil, false, err
	}
	offsetTerm, err := p.syntax.manager.ArithmeticNumeralFromRational(offset, true)
	if err != nil {
		return nil, false, err
	}
	xTerm, err := p.syntax.manager.MkArithmeticAdd(selectedTerm, offsetTerm)
	if err == nil && literals[selected].strict {
		one, oneErr := p.syntax.manager.ArithmeticIntegerNumeralText("1")
		if oneErr == nil {
			xTerm, err = p.syntax.manager.MkArithmeticAdd(xTerm, one)
		} else {
			err = oneErr
		}
	}
	selectedPositive := false
	if err == nil {
		selectedPositive, err = q.IsPos(&literals[selected].coefficient)
	}
	if err == nil && selectedPositive {
		xTerm, err = p.syntax.manager.MkArithmeticNeg(xTerm)
	}
	if err != nil {
		return nil, false, err
	}
	rewritten, err := p.rewriteLegacy(xTerm)
	if err != nil {
		return nil, false, err
	}
	xTerm, err = p.syntax.manager.RefGet(rewritten)
	if err != nil {
		_ = p.syntax.manager.DelRef(&rewritten)
		return nil, false, err
	}
	result, err = p.substituteIntegerLiterals(literals, originals, xTerm, selected)
	if err == nil {
		err = attachDivisibility(result, xTerm)
	}
	err = errors.Join(err, p.syntax.manager.DelRef(&rewritten))
	if err != nil {
		if result != nil {
			err = errors.Join(err, result.Close())
		}
		return nil, false, err
	}
	return result, true, nil
}
