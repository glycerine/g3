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

// rewriteFinite is the finite branch of arith_plugin::assign: mk_bounds
// (strict, then non-strict) over the selected side and mk_resolve (strict,
// then non-strict) over the opposite side. Each constraint is added through
// add_constraint in the source order relative to the result's substitutions.
func (p *ArithmeticPlugin) rewriteFinite(entry *arithmeticPluginBounds, target ast.ASTHandle, selection ArithmeticBranchSelection, xt *arithmeticBranchSubstitution, result ast.ASTRef) (err error) {
	m := p.manager
	selected := selection.Bound
	if selected == nil {
		return ast.ErrASTDomain
	}
	selectedTerm, err := m.RefGet(selected.Term)
	if err != nil {
		return err
	}
	// expr_ref t(bounds.exprs(is_strict, is_lower)[index], m) holds the
	// selected term through mk_bounds and mk_resolve.
	t, err := m.NewRef(selectedTerm)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&t)) }()
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
	// t = x_t.mk_term(a, t); a = x_t.mk_coeff(a);
	term, err := xt.mkTerm(p, selected.Coefficient, selectedTerm)
	if err != nil {
		return err
	}
	if err = m.SetRef(t, term); err != nil {
		return err
	}
	a := xt.mkCoeff(selected.Coefficient)
	strictReal := !isEqCtx && real && !isStrictCtx
	for _, strict := range []bool{true, false} {
		if err = p.mkResolve(entry, target, xt, strict || isStrictCtx || strictReal, strict, opposite, a, term, result); err != nil {
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
	// expr_ref tmp(m), eq(m);
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
		if err = p.boundOne(x, uint32(i), current, sameStrict, nonStrictReal, isEqCtx, isStrict, selection, t, tmp, eq, result); err != nil {
			return err
		}
	}
	return nil
}

// boundOne is one mk_bounds iteration; its expr_ref s holds the bound's
// term for the iteration.
func (p *ArithmeticPlugin) boundOne(x ast.ASTHandle, i uint32, current *ArithmeticBound, sameStrict, nonStrictReal, isEqCtx, isStrict bool, selection ArithmeticBranchSelection, t ast.ASTHandle, tmp, eq, result ast.ASTRef) (err error) {
	m := p.manager
	selected := selection.Bound
	e, err := m.RefGet(current.Atom)
	if err != nil {
		return err
	}
	sh, err := m.RefGet(current.Term)
	if err != nil {
		return err
	}
	s, err := m.NewRef(sh)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&s)) }()
	if sameStrict && i == selection.Index {
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
				err = p.appendConstraintNot(true, h)
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
		return err
	}
	resultStrict := (nonStrictReal && isEqCtx && isStrict) || (sameStrict && i < selection.Index)
	if err = p.mkBound(resultStrict, selection.Lower, selected.Coefficient, t, current.Coefficient, sh, tmp); err != nil {
		return err
	}
	h, err := m.RefGet(tmp)
	if err != nil {
		return err
	}
	if err = p.replacer.ApplySubstitution(e, h, result); err != nil {
		return err
	}
	return p.addConstraintNotVar(true, e, h)
}

// appendConstraintNot is m_ctx.add_constraint(useVariable, mk_not(a)) with
// the mk_not temporary alive for the call.
func (p *ArithmeticPlugin) appendConstraintNot(useVariable bool, a ast.ASTHandle) (err error) {
	m := p.manager
	negated, err := p.mkNot(a)
	if err != nil {
		return err
	}
	temporary, err := m.NewRef(negated)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&temporary)) }()
	return p.appendConstraint(useVariable, negated)
}

// mkBound is arith_plugin::mk_bound: the strict or non-strict bound with
// the lower-bound arguments swapped, then m_util.simplify(result).
func (p *ArithmeticPlugin) mkBound(strict, lower bool, a *big.Rat, t ast.ASTHandle, b *big.Rat, s ast.ASTHandle, result ast.ASTRef) error {
	var bound ast.ASTRef
	var err error
	if lower {
		// b*t > a*s (strict) or b*t >= a*s
		bound, err = p.util.MakeBound(b, s, a, t, strict)
	} else {
		bound, err = p.util.MakeBound(a, t, b, s, strict)
	}
	if err != nil {
		return err
	}
	if err = p.assignOwned(result, bound); err != nil {
		return err
	}
	return p.simplify(result)
}

// mkEqBound is arith_qe_util::mk_eq: mk_eq(mk_add(mk_mul(a, x), t), 0),
// where mk_mul returns x for a unit a.
func (p *ArithmeticPlugin) mkEqBound(a *big.Rat, x, t ast.ASTHandle) (ast.ASTHandle, error) {
	m := p.manager
	ax := x
	if a.Cmp(big.NewRat(1, 1)) != 0 {
		k, err := p.util.numeralFor(a, x)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if ax, err = m.MkArithmeticMul(k, x); err != nil {
			return ast.ASTHandle{}, err
		}
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
func (p *ArithmeticPlugin) mkResolve(entry *arithmeticPluginBounds, x ast.ASTHandle, xt *arithmeticBranchSubstitution, strictResolve, isStrict bool, opposite ArithmeticBoundDirection, a *big.Rat, t ast.ASTHandle, result ast.ASTRef) (err error) {
	m := p.manager
	tmp, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&tmp)) }()
	for _, current := range entry.processor.Bounds(isStrict, opposite) {
		if err = p.resolveOne(x, xt, strictResolve, a, t, current, tmp, result); err != nil {
			return err
		}
	}
	return nil
}

// resolveOne is one mk_resolve iteration: expr_ref s(...);
// s = x_t.mk_term(b, s); b = x_t.mk_coeff(b); m_util.mk_resolve(..., tmp);
// expr_ref save_result(result); the substitution of e by tmp and the
// constraint (not e) or tmp. save_result and then s die at its end.
func (p *ArithmeticPlugin) resolveOne(x ast.ASTHandle, xt *arithmeticBranchSubstitution, strictResolve bool, a *big.Rat, t ast.ASTHandle, current *ArithmeticBound, tmp, result ast.ASTRef) (err error) {
	m := p.manager
	e, err := m.RefGet(current.Atom)
	if err != nil {
		return err
	}
	s0, err := m.RefGet(current.Term)
	if err != nil {
		return err
	}
	s, err := m.NewRef(s0)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&s)) }()
	term, err := xt.mkTerm(p, current.Coefficient, s0)
	if err != nil {
		return err
	}
	if err = m.SetRef(s, term); err != nil {
		return err
	}
	b := xt.mkCoeff(current.Coefficient)
	replacement, handled, err := p.util.ResolveUnitOrReal(x, strictResolve, a, t, b, term)
	if err != nil {
		return err
	}
	if !handled {
		return errArithmeticBranchUnsupported
	}
	if err = p.assignOwned(tmp, replacement); err != nil {
		return err
	}
	saved, err := m.CopyRef(result)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&saved)) }()
	h, err := m.RefGet(tmp)
	if err != nil {
		return err
	}
	if err = p.replacer.ApplySubstitution(e, h, result); err != nil {
		return err
	}
	return p.addConstraintNotVar(true, e, h)
}
