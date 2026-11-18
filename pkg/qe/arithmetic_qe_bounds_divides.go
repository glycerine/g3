package qe

// Source: src/qe/qe_arith_plugin.cpp bounds_proc::get_divides.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// appendDivisibility pushes term, divisor, coefficient (both copied, as
// push_back) and atom.
func (p *ArithmeticBoundsProcessor) appendDivisibility(atom ast.ASTHandle, term ast.ASTRef, coefficient, divisor *unumeral.Rational, negated, nested bool, z ast.ASTRef) error {
	atomRef, err := p.manager.NewRef(atom)
	if err != nil {
		err = errors.Join(err, p.manager.DelRef(&term))
		if z != (ast.ASTRef{}) {
			err = errors.Join(err, p.manager.DelRef(&z))
		}
		return err
	}
	q := p.util.q()
	d := &ArithmeticDivisibility{manager: p.manager, Term: term, Atom: atomRef, Z: z,
		Coefficient: unumeral.NewRational(), Divisor: unumeral.NewRational(), Negated: negated, Nested: nested}
	if err = errors.Join(q.Copy(&d.Divisor, divisor), q.Copy(&d.Coefficient, coefficient)); err != nil {
		return errors.Join(err, d.Close())
	}
	p.divisibilities = append(p.divisibilities, d)
	return nil
}

// addDivides is get_divides: numeral k, k2 live for the whole call.
func (p *ArithmeticBoundsProcessor) addDivides(variable *ast.ContainsApp, expression ast.ASTHandle) (_ bool, resultErr error) {
	q := p.util.q()
	divisor, coefficient := unumeral.NewRational(), unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&divisor), q.Del(&coefficient)) }()
	polynomial, matched, err := p.util.MatchDivides(expression, &divisor)
	atom := expression
	negated := false
	if err != nil {
		return false, err
	}
	if !matched {
		atom, polynomial, matched, err = p.util.MatchNotDivides(expression, &divisor)
		negated = matched
		if err != nil || !matched {
			return false, err
		}
	}
	rest, found, err := p.util.Coefficient(variable, polynomial, &coefficient)
	if err != nil || !found {
		if rest != (ast.ASTRef{}) {
			err = errors.Join(err, p.manager.DelRef(&rest))
		}
		return false, err
	}
	if err = p.appendDivisibility(atom, rest, &coefficient, &divisor, negated, false, ast.ASTRef{}); err != nil {
		return false, err
	}
	return true, nil
}
