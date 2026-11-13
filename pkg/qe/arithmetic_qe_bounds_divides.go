package qe

// Source: src/qe/qe_arith_plugin.cpp bounds_proc::get_divides.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *ArithmeticBoundsProcessor) appendDivisibility(atom ast.ASTHandle, term ast.ASTRef, coefficient, divisor *big.Rat, negated, nested bool, z ast.ASTRef) error {
	atomRef, err := p.manager.NewRef(atom)
	if err != nil {
		err = errors.Join(err, p.manager.DelRef(&term))
		if z != (ast.ASTRef{}) {
			err = errors.Join(err, p.manager.DelRef(&z))
		}
		return err
	}
	p.divisibilities = append(p.divisibilities, &ArithmeticDivisibility{manager: p.manager, Term: term, Atom: atomRef, Z: z,
		Coefficient: new(big.Rat).Set(coefficient), Divisor: new(big.Rat).Set(divisor), Negated: negated, Nested: nested})
	return nil
}

func (p *ArithmeticBoundsProcessor) addDivides(variable *ast.ContainsApp, expression ast.ASTHandle) (bool, error) {
	divisor, polynomial, matched, err := p.util.MatchDivides(expression)
	atom := expression
	negated := false
	if err != nil {
		return false, err
	}
	if !matched {
		atom, divisor, polynomial, matched, err = p.util.MatchNotDivides(expression)
		negated = matched
		if err != nil || !matched {
			return false, err
		}
	}
	coefficient, rest, found, err := p.util.Coefficient(variable, polynomial)
	if err != nil || !found {
		if rest != (ast.ASTRef{}) {
			err = errors.Join(err, p.manager.DelRef(&rest))
		}
		return false, err
	}
	if err = p.appendDivisibility(atom, rest, coefficient, divisor, negated, false, ast.ASTRef{}); err != nil {
		return false, err
	}
	return true, nil
}
