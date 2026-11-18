package qe

// Source: src/qe/qe_arith_plugin.cpp bounds_proc inequality collection.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// normalizeRest is the real case of get_le_bound/get_lt_bound:
// mk_div(rest, abs(k), rest) and k = sign(k), updating coefficient in place.
func (p *ArithmeticBoundsProcessor) normalizeRest(variable *ast.ContainsApp, coefficient *unumeral.Rational, rest ast.ASTRef) (ast.ASTRef, error) {
	target, err := variable.Target()
	if err != nil {
		return rest, err
	}
	integer, err := p.util.isInteger(target)
	if err != nil || integer {
		return rest, err
	}
	restHandle, err := p.manager.RefGet(rest)
	if err != nil {
		return rest, err
	}
	q := p.util.q()
	abs := unumeral.NewRational()
	if err = errors.Join(q.Copy(&abs, coefficient), q.Abs(&abs)); err != nil {
		return rest, errors.Join(err, q.Del(&abs))
	}
	denominator, err := p.util.numeralFor(&abs, restHandle)
	if err = errors.Join(err, q.Del(&abs)); err != nil {
		return rest, err
	}
	divided, err := p.manager.MkArithmeticDiv(restHandle, denominator)
	if err != nil {
		return rest, err
	}
	normalized, err := p.manager.NewRef(divided)
	if err != nil {
		return rest, err
	}
	if err = p.manager.DelRef(&rest); err != nil {
		return normalized, err
	}
	// arith_qe_util::mk_div ends with simplify(result); the division is
	// held only by result while it is rewritten.
	if p.util.theory == nil {
		return normalized, ast.ErrASTHandle
	}
	if err = p.util.theory.Rewrite(divided, normalized); err != nil {
		return normalized, err
	}
	// k = k.is_pos()?rational::one():rational::minus_one();
	positive, err := q.IsPos(coefficient)
	if err != nil {
		return normalized, err
	}
	if positive {
		return normalized, q.SetInt32(coefficient, 1)
	}
	return normalized, q.SetInt32(coefficient, -1)
}

// appendBound pushes term, coefficient (copied, as push_back(k)) and atom.
func (p *ArithmeticBoundsProcessor) appendBound(atom ast.ASTHandle, term ast.ASTRef, coefficient *unumeral.Rational, strict bool) error {
	q := p.util.q()
	negative, err := q.IsNeg(coefficient)
	if err != nil {
		return errors.Join(err, p.manager.DelRef(&term))
	}
	atomRef, err := p.manager.NewRef(atom)
	if err != nil {
		return errors.Join(err, p.manager.DelRef(&term))
	}
	direction := ArithmeticUpperBound
	if negative {
		direction = ArithmeticLowerBound
	}
	bound := &ArithmeticBound{manager: p.manager, Term: term, Atom: atomRef, Coefficient: unumeral.NewRational(), Strict: strict, Direction: direction}
	if err = q.Copy(&bound.Coefficient, coefficient); err != nil {
		return errors.Join(err, bound.Close())
	}
	p.bounds = append(p.bounds, bound)
	return nil
}

// boundFrom is the shared tail of get_le_bound / get_lt_bound once
// get_coeff matched: rational k lives for the whole call.
func (p *ArithmeticBoundsProcessor) boundFrom(variable *ast.ContainsApp, atom, polynomial ast.ASTHandle, strict bool) (_ bool, resultErr error) {
	q := p.util.q()
	coefficient := unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&coefficient)) }()
	rest, matched, err := p.util.Coefficient(variable, polynomial, &coefficient)
	if err != nil || !matched {
		if rest != (ast.ASTRef{}) {
			err = errors.Join(err, p.manager.DelRef(&rest))
		}
		return false, err
	}
	rest, err = p.normalizeRest(variable, &coefficient, rest)
	if err != nil {
		if rest != (ast.ASTRef{}) {
			err = errors.Join(err, p.manager.DelRef(&rest))
		}
		return false, err
	}
	if err = p.appendBound(atom, rest, &coefficient, strict); err != nil {
		return false, err
	}
	return true, nil
}

func (p *ArithmeticBoundsProcessor) addNonStrict(variable *ast.ContainsApp, atom ast.ASTHandle) (_ bool, resultErr error) {
	polynomial, relation, err := p.util.MatchLE(atom)
	if err != nil || !relation {
		return false, err
	}
	polynomialHandle, err := p.manager.RefGet(polynomial)
	if err != nil {
		return false, errors.Join(err, p.manager.DelRef(&polynomial))
	}
	// expr_ref p lives to the end of get_le_bound, through mk_div's
	// simplify of rest.
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&polynomial)) }()
	return p.boundFrom(variable, atom, polynomialHandle, false)
}

func (p *ArithmeticBoundsProcessor) addStrict(variable *ast.ContainsApp, atom ast.ASTHandle) (_ bool, resultErr error) {
	polynomial, relation, err := p.util.MatchLT(atom)
	if err != nil {
		return false, err
	}
	if !relation {
		inner, negative, cause := p.util.Negated(atom)
		if cause != nil || !negative {
			return false, cause
		}
		polynomial, relation, err = p.util.MatchGE(inner)
		if err != nil || !relation {
			return false, err
		}
	}
	target, err := variable.Target()
	if err != nil {
		return false, errors.Join(err, p.manager.DelRef(&polynomial))
	}
	integer, err := p.util.isInteger(target)
	if err != nil || integer {
		return false, errors.Join(err, p.manager.DelRef(&polynomial))
	}
	polynomialHandle, err := p.manager.RefGet(polynomial)
	if err != nil {
		return false, errors.Join(err, p.manager.DelRef(&polynomial))
	}
	// expr_ref p lives to the end of get_lt_bound, through mk_div's
	// simplify of rest.
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&polynomial)) }()
	return p.boundFrom(variable, atom, polynomialHandle, true)
}
