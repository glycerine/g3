package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// arith_project_util::project, integer coefficient/divisor normalization.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// legacyIntegerLCM is out = lcm(abs(left), abs(right)) of integer
// rationals (zero when either is zero).
func legacyIntegerLCM(q *unumeral.RationalManager, left, right, out *unumeral.Rational) (resultErr error) {
	if left == nil || right == nil {
		return ast.ErrASTDomain
	}
	for _, v := range []*unumeral.Rational{left, right} {
		if isInt, err := q.IsInt(v); err != nil || !isInt {
			return errors.Join(err, ast.ErrASTDomain)
		}
	}
	a, b := unumeral.NewRational(), unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&a), q.Del(&b)) }()
	if err := errors.Join(q.Copy(&a, left), q.Abs(&a), q.Copy(&b, right), q.Abs(&b)); err != nil {
		return err
	}
	aZero, err := q.IsZero(&a)
	if err != nil {
		return err
	}
	bZero, err := q.IsZero(&b)
	if err != nil {
		return err
	}
	if aZero || bZero {
		return q.SetInt32(out, 0)
	}
	return q.LCM(&a, &b, out)
}

func (p *legacyArithmeticProjection) replaceLegacyTerm(literal *legacyLinearLiteral, term ast.ASTHandle) error {
	replacement, err := p.syntax.manager.NewRef(term)
	if err != nil {
		return err
	}
	if err = p.syntax.manager.DelRef(&literal.term); err != nil {
		_ = p.syntax.manager.DelRef(&replacement)
		return err
	}
	literal.term = replacement
	return nil
}

// normalizeIntegers computes m_lcm_coeffs and m_lcm_divs into
// lcmCoefficients and lcmDivisors (rationals of q()) and normalizes the
// literals' coefficients, divisors and terms.
func (p *legacyArithmeticProjection) normalizeIntegers(literals []*legacyLinearLiteral, lcmCoefficients, lcmDivisors *unumeral.Rational) (resultErr error) {
	if p == nil || p.syntax == nil {
		return ast.ErrASTHandle
	}
	q := p.q()
	if err := q.SetInt32(lcmCoefficients, 1); err != nil {
		return err
	}
	for _, literal := range literals {
		if literal == nil || literal.manager != p.syntax.manager {
			return ast.ErrASTDomain
		}
		zero, err := q.IsZero(&literal.coefficient)
		if err != nil {
			return err
		}
		isInt, err := q.IsInt(&literal.coefficient)
		if err != nil {
			return err
		}
		if zero || !isInt {
			return ast.ErrASTDomain
		}
		if err = legacyIntegerLCM(q, lcmCoefficients, &literal.coefficient, lcmCoefficients); err != nil {
			return err
		}
	}
	if err := q.SetInt32(lcmDivisors, 1); err != nil {
		return err
	}
	// rational k = m_lcm_coeffs / abs(a);
	factor := unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&factor)) }()
	for _, literal := range literals {
		if err := errors.Join(q.Copy(&factor, &literal.coefficient), q.Abs(&factor)); err != nil {
			return err
		}
		if err := q.Div(lcmCoefficients, &factor, &factor); err != nil {
			return err
		}
		one, err := q.IsOne(&factor)
		if err != nil {
			return err
		}
		if !one {
			term, err := p.syntax.manager.RefGet(literal.term)
			if err != nil {
				return err
			}
			// if (!factor.is_one () && !a.is_zero (m_terms.get (i)))
			// (AUDIT.md qe_02-F7).
			zero, err := p.syntax.manager.ArithmeticNumeralIsZero(term)
			if err != nil {
				return err
			}
			if !zero {
				scaled, err := p.multiplyLegacy(&factor, term)
				if err != nil {
					return err
				}
				if err = p.replaceLegacyTerm(literal, scaled); err != nil {
					return err
				}
			}
		}
		positive, err := q.IsPos(&literal.coefficient)
		if err != nil {
			return err
		}
		if err = q.Copy(&literal.coefficient, lcmCoefficients); err != nil {
			return err
		}
		if !positive {
			if err = q.Neg(&literal.coefficient); err != nil {
				return err
			}
		}
		divisorZero, err := q.IsZero(&literal.divisor)
		if err != nil {
			return err
		}
		if !divisorZero {
			if err = q.Mul(&literal.divisor, &factor, &literal.divisor); err != nil {
				return err
			}
			if err = legacyIntegerLCM(q, lcmDivisors, &literal.divisor, lcmDivisors); err != nil {
				return err
			}
		}
	}
	return legacyIntegerLCM(q, lcmDivisors, lcmCoefficients, lcmDivisors)
}

func (p *legacyArithmeticProjection) substituteIntegerLiterals(literals []*legacyLinearLiteral, originals []ast.ASTHandle, xTerm ast.ASTHandle, selected int) (_ *legacyRealProjection, resultErr error) {
	if p == nil || p.syntax == nil || len(literals) != len(originals) || selected < 0 || selected >= len(literals) {
		return nil, ast.ErrASTHandle
	}
	result := &legacyRealProjection{manager: p.syntax.manager}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	zero, err := p.syntax.manager.ArithmeticIntegerNumeralText("0")
	if err != nil {
		return nil, err
	}
	for index, literal := range literals {
		if literal == nil || literal.manager != p.syntax.manager {
			return nil, ast.ErrASTDomain
		}
		if index == selected {
			replacement, err := p.constantLegacy(true)
			if err != nil {
				return nil, err
			}
			p.appendLegacyReplacement(result, originals[index], replacement)
			continue
		}
		term, err := p.syntax.manager.RefGet(literal.term)
		if err != nil {
			return nil, err
		}
		var combined ast.ASTHandle
		negative, err := p.q().IsNeg(&literal.coefficient)
		if err != nil {
			return nil, err
		}
		if negative {
			combined, err = p.syntax.manager.MkArithmeticSub(term, xTerm)
		} else {
			combined, err = p.syntax.manager.MkArithmeticAdd(term, xTerm)
		}
		if err != nil {
			return nil, err
		}
		var replacement ast.ASTRef
		divisorZero, err := p.q().IsZero(&literal.divisor)
		if err != nil {
			return nil, err
		}
		if divisorZero {
			var expression ast.ASTHandle
			switch {
			case literal.equality:
				expression, err = p.syntax.manager.MkEq(combined, zero)
			case literal.strict:
				expression, err = p.syntax.manager.MkFamilyApp(p.syntax.family, 4, nil, []ast.ASTHandle{combined, zero})
			default:
				expression, err = p.syntax.manager.MkArithmeticLE(combined, zero)
			}
			if err == nil {
				replacement, err = p.rewriteLegacy(expression)
			}
		} else {
			rewritten, rewriteErr := p.rewriteLegacy(combined)
			if rewriteErr != nil {
				return nil, rewriteErr
			}
			rewrittenHandle, rewriteErr := p.syntax.manager.RefGet(rewritten)
			if rewriteErr != nil {
				_ = p.syntax.manager.DelRef(&rewritten)
				return nil, rewriteErr
			}
			modulus, rewriteErr := p.syntax.manager.ArithmeticNumeralFromRational(&literal.divisor, true)
			if rewriteErr == nil {
				var modular ast.ASTHandle
				modular, rewriteErr = p.syntax.manager.MkFamilyApp(p.syntax.family, 13, nil, []ast.ASTHandle{rewrittenHandle, modulus})
				if rewriteErr == nil {
					var expression ast.ASTHandle
					expression, rewriteErr = p.syntax.manager.MkEq(modular, zero)
					if rewriteErr == nil {
						replacement, rewriteErr = p.syntax.manager.NewRef(expression)
					}
				}
			}
			err = errors.Join(rewriteErr, p.syntax.manager.DelRef(&rewritten))
		}
		if err != nil {
			return nil, err
		}
		p.appendLegacyReplacement(result, originals[index], replacement)
	}
	return result, nil
}
