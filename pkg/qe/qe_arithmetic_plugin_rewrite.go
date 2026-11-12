package qe

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// errArithmeticBranchUnsupported reports an arithmetic branch whose bound
// resolution is not yet implemented by the concrete plugin.
var errArithmeticBranchUnsupported = errors.New("qe: arithmetic branch resolution is not implemented")

func (p *ArithmeticPlugin) replaceAtom(result ast.ASTRef, atom ast.ASTRef, replacement ast.ASTHandle) error {
	a, err := p.manager.RefGet(atom)
	if err != nil {
		return err
	}
	return p.replacer.ApplySubstitution(a, replacement, result)
}

func (p *ArithmeticPlugin) rewriteInfinity(entry *arithmeticPluginBounds, lower bool, result ast.ASTRef) error {
	truth, err := p.manager.MkTrue()
	if err != nil {
		return err
	}
	falsity, err := p.manager.MkFalse()
	if err != nil {
		return err
	}
	selected := ArithmeticUpperBound
	if lower {
		selected = ArithmeticLowerBound
	}
	opposite := ArithmeticLowerBound
	if lower {
		opposite = ArithmeticUpperBound
	}
	// mk_non_bounds(strict), mk_non_bounds(non-strict), then
	// mk_non_resolve(strict), mk_non_resolve(non-strict): each atom's
	// constraint is added before the atom is replaced in the result.
	for _, strict := range []bool{true, false} {
		for _, b := range entry.processor.Bounds(strict, selected) {
			a, cause := p.manager.RefGet(b.Atom)
			if cause != nil {
				return cause
			}
			negated, cause := p.mkNot(a)
			if cause != nil {
				return cause
			}
			if err = p.appendConstraint(true, negated); err != nil {
				return err
			}
			if err = p.replaceAtom(result, b.Atom, falsity); err != nil {
				return err
			}
		}
	}
	for _, strict := range []bool{true, false} {
		for _, b := range entry.processor.Bounds(strict, opposite) {
			a, cause := p.manager.RefGet(b.Atom)
			if cause != nil {
				return cause
			}
			if err = p.appendConstraint(true, a); err != nil {
				return err
			}
			if err = p.replaceAtom(result, b.Atom, truth); err != nil {
				return err
			}
		}
	}
	return nil
}

// mkNot is arith_plugin::mk_not: the argument of a negation, otherwise
// m.mk_not(e).
func (p *ArithmeticPlugin) mkNot(e ast.ASTHandle) (ast.ASTHandle, error) {
	isNot, arg, err := p.manager.IsNot(e)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if isNot {
		return arg, nil
	}
	return p.manager.MkNot(e)
}

func (p *ArithmeticPlugin) sameSide(selected, current *ArithmeticBound, lower, strict bool) (ast.ASTRef, error) {
	a := new(big.Rat).Set(selected.Coefficient)
	b := new(big.Rat).Set(current.Coefficient)
	if lower {
		a.Neg(a)
		b.Neg(b)
	}
	t, err := p.manager.RefGet(selected.Term)
	if err != nil {
		return ast.ASTRef{}, err
	}
	s, err := p.manager.RefGet(current.Term)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return p.util.MakeBound(a, t, b, s, strict)
}

// rewriteFinite is the finite branch of arith_plugin::assign: mk_bounds
// (strict, then non-strict) over the selected side and mk_resolve (strict,
// then non-strict) over the opposite side. Each constraint is added through
// add_constraint in the source order relative to the result's substitutions.
func (p *ArithmeticPlugin) rewriteFinite(entry *arithmeticPluginBounds, target ast.ASTHandle, selection ArithmeticBranchSelection, result ast.ASTRef) (err error) {
	m := p.manager
	selected := selection.Bound
	if selected == nil {
		return ast.ErrASTDomain
	}
	selectedTerm, err := m.RefGet(selected.Term)
	if err != nil {
		return err
	}
	integer, err := p.util.isInteger(target)
	if err != nil {
		return err
	}
	real := !integer
	direction := selected.Direction
	opposite := ArithmeticLowerBound
	if direction == ArithmeticLowerBound {
		opposite = ArithmeticUpperBound
	}
	isStrictCtx := selected.Strict
	isEqCtx := real && selection.Kind == ArithmeticEqualityBranch
	for _, strict := range []bool{true, false} {
		if err = p.mkBounds(entry, target, strict, isEqCtx, isStrictCtx, real, selection, selectedTerm, result); err != nil {
			return err
		}
	}
	// t = x_t.mk_term(a, t); a = x_t.mk_coeff(a): the Cooper substitution
	// has already been applied to the stored bounds.
	strictReal := !isEqCtx && real && !isStrictCtx
	for _, strict := range []bool{true, false} {
		if err = p.mkResolve(entry, target, strict || isStrictCtx || strictReal, strict, opposite, selected, selectedTerm, result); err != nil {
			return err
		}
	}
	return nil
}

// mkBounds is arith_plugin::mk_bounds for one strictness of the selected
// side.
func (p *ArithmeticPlugin) mkBounds(entry *arithmeticPluginBounds, x ast.ASTHandle, isStrict, isEqCtx, isStrictCtx, real bool, selection ArithmeticBranchSelection, t ast.ASTHandle, result ast.ASTRef) (err error) {
	m := p.manager
	selected := selection.Bound
	tmp, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	eq, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return errors.Join(err, m.DelRef(&tmp))
	}
	defer func() { err = errors.Join(err, m.DelRef(&eq), m.DelRef(&tmp)) }()
	sameStrict := isStrict == isStrictCtx
	nonStrictReal := real && !isStrictCtx
	for i, current := range entry.processor.Bounds(isStrict, selected.Direction) {
		e, err := m.RefGet(current.Atom)
		if err != nil {
			return err
		}
		if sameStrict && uint32(i) == selection.Index {
			if nonStrictReal {
				h, err := p.mkEqBound(selected.Coefficient, x, t)
				if err == nil {
					err = m.SetRef(eq, h)
				}
				if err != nil {
					return err
				}
				if isEqCtx {
					err = p.appendConstraint(true, h)
				} else {
					var neg ast.ASTHandle
					neg, err = p.mkNot(h)
					if err == nil {
						err = p.appendConstraint(true, neg)
					}
					if err == nil {
						err = p.appendConstraint(true, e)
					}
				}
			} else {
				err = p.appendConstraint(true, e)
			}
			if err != nil {
				return err
			}
			truth, err := m.MkTrue()
			if err == nil {
				err = p.replaceAtom(result, selected.Atom, truth)
			}
			if err != nil {
				return err
			}
			continue
		}
		resultStrict := (nonStrictReal && isEqCtx && isStrict) || (sameStrict && uint32(i) < selection.Index)
		bound, err := p.sameSide(selected, current, selection.Lower, resultStrict)
		if err != nil {
			return err
		}
		if err = p.assignOwned(tmp, bound); err != nil {
			return err
		}
		h, err := m.RefGet(tmp)
		if err == nil {
			err = p.replaceAtom(result, current.Atom, h)
		}
		var neg ast.ASTHandle
		if err == nil {
			neg, err = p.mkNot(e)
		}
		if err == nil {
			err = p.appendConstraint2(true, neg, h)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// mkEqBound is arith_qe_util::mk_eq: a*x + t = 0.
func (p *ArithmeticPlugin) mkEqBound(a *big.Rat, x, t ast.ASTHandle) (ast.ASTHandle, error) {
	m := p.manager
	integer, err := p.util.isInteger(x)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	k, err := m.ArithmeticNumeralRat(a, integer)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ax, err := m.MkArithmeticMul(k, x)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sum, err := m.MkArithmeticAdd(ax, t)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	zero, err := p.util.zero(x)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.MkEq(sum, zero)
}

// mkResolve is arith_plugin::mk_resolve for one strictness of the opposite
// side.
func (p *ArithmeticPlugin) mkResolve(entry *arithmeticPluginBounds, x ast.ASTHandle, strictResolve, isStrict bool, opposite ArithmeticBoundDirection, selected *ArithmeticBound, t ast.ASTHandle, result ast.ASTRef) (err error) {
	m := p.manager
	tmp, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&tmp)) }()
	for _, current := range entry.processor.Bounds(isStrict, opposite) {
		e, err := m.RefGet(current.Atom)
		if err != nil {
			return err
		}
		s, err := m.RefGet(current.Term)
		if err != nil {
			return err
		}
		replacement, handled, err := p.util.ResolveUnitOrReal(x, strictResolve, selected.Coefficient, t, current.Coefficient, s)
		if err != nil {
			return err
		}
		if !handled {
			return errArithmeticBranchUnsupported
		}
		if err = p.assignOwned(tmp, replacement); err != nil {
			return err
		}
		// expr_ref save_result(result) lives to the end of the iteration.
		saved, err := m.CopyRef(result)
		if err != nil {
			return err
		}
		h, err := m.RefGet(tmp)
		if err == nil {
			err = p.replaceAtom(result, current.Atom, h)
		}
		var neg ast.ASTHandle
		if err == nil {
			neg, err = p.mkNot(e)
		}
		if err == nil {
			err = p.appendConstraint2(true, neg, h)
		}
		if err = errors.Join(err, m.DelRef(&saved)); err != nil {
			return err
		}
	}
	return nil
}
