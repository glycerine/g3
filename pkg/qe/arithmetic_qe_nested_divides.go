package qe

// Source: src/qe/qe_arith_plugin.cpp bounds_proc::get_nested_divs.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *ArithmeticBoundsProcessor) addNestedDivides(variable *ast.ContainsApp, expression ast.ASTHandle) (bool, error) {
	target, err := variable.Target()
	if err != nil {
		return false, err
	}
	// m_mark is the bounds_proc mark get_bound also sets: a subterm seen
	// under an earlier atom is not revisited.
	todo := []ast.ASTHandle{expression}
	for len(todo) != 0 {
		current := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if p.marked[current] {
			continue
		}
		p.marked[current] = true
		contains, cause := variable.Check(current)
		if cause != nil {
			return false, cause
		}
		if !contains {
			continue
		}
		if current == target {
			return false, nil
		}
		if kind, cause := p.manager.GetKind(current); cause != nil || kind != ast.ASTApp {
			return false, cause
		}
		args, modular, cause := p.util.operation(current, p.util.family, 13)
		if cause != nil {
			return false, cause
		}
		if modular && len(args) == 2 {
			divisor, numeral, cause := p.util.numeral(args[1])
			if cause != nil {
				return false, cause
			}
			if numeral {
				coefficient, rest, found, cause := p.util.Coefficient(variable, args[0])
				if cause != nil {
					return false, cause
				}
				if found {
					// m_util.mk_bounded_var(k1, z_bv, z) before the pushes.
					z, cause := p.util.mkBoundedVar(divisor)
					var zRef ast.ASTRef
					if cause == nil {
						zRef, cause = p.manager.NewRef(z)
					}
					if cause != nil {
						return false, errors.Join(cause, p.manager.DelRef(&rest))
					}
					if cause = p.appendDivisibility(current, rest, coefficient, divisor, false, true, zRef); cause != nil {
						return false, cause
					}
					continue
				}
				if rest != (ast.ASTRef{}) {
					if cause = p.manager.DelRef(&rest); cause != nil {
						return false, cause
					}
				}
			}
		}
		count, cause := p.manager.GetNumArgs(current)
		if cause != nil {
			return false, cause
		}
		for i := uint32(0); i < count; i++ {
			argument, cause := p.manager.GetArg(current, i)
			if cause != nil {
				return false, cause
			}
			todo = append(todo, argument)
		}
	}
	return true, nil
}
