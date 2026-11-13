package qe

// Source: src/qe/qe_arith_plugin.cpp arith_plugin::assign_nested_divs,
// assign_divs and mk_div_equivs.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// applyCooper runs assign_nested_divs then assign_divs on result. The
// residues belong to the bounds processor, so every branch of one (x, fml)
// shares them. applied reports that div_z set x_t to d*x + z.
func (p *ArithmeticPlugin) applyCooper(variable ast.ASTHandle, processor *ArithmeticBoundsProcessor, result ast.ASTRef) (substitution *arithmeticBranchSubstitution, applied bool, resultErr error) {
	if err := p.assignNestedDivs(processor, result); err != nil {
		return nil, false, err
	}
	return p.assignDivs(variable, processor, result)
}

// addVar is m_util.add_var(v): the variable is published to the context.
func (p *ArithmeticPlugin) addVar(v ast.ASTHandle) error {
	ref, err := p.manager.NewRef(v)
	if err != nil {
		return err
	}
	p.pendingFresh = append(p.pendingFresh, ref)
	return nil
}

// divZRefs is `app_ref z(m), z_bv(m); bounds.div_z(d, z_bv, z)`: the
// locals hold the shared residue and its bit-vector for the caller's scope.
// The returned release drops z_bv and then z.
func (p *ArithmeticPlugin) divZRefs(processor *ArithmeticBoundsProcessor) (d *big.Rat, z, zBV ast.ASTHandle, ok bool, release func() error, err error) {
	m := p.manager
	release = func() error { return nil }
	d, z, ok, err = processor.commonDivisor()
	if err != nil || !ok {
		return d, z, zBV, ok, release, err
	}
	if zBV, err = m.GetArg(z, 0); err != nil {
		return d, z, zBV, false, release, err
	}
	zRef, err := m.NewRef(z)
	if err != nil {
		return d, z, zBV, false, release, err
	}
	zBVRef, err := m.NewRef(zBV)
	if err != nil {
		return d, z, zBV, false, release, errors.Join(err, m.DelRef(&zRef))
	}
	release = func() error { return errors.Join(m.DelRef(&zBVRef), m.DelRef(&zRef)) }
	return d, z, zBV, true, release, nil
}

func (p *ArithmeticPlugin) assignNestedDivs(processor *ArithmeticBoundsProcessor, result ast.ASTRef) (err error) {
	nested := processor.Divisibilities(true)
	if len(nested) == 0 {
		return nil
	}
	_, z, _, ok, release, err := p.divZRefs(processor)
	defer func() { err = errors.Join(err, release()) }()
	if err != nil {
		return err
	}
	if !ok {
		return ast.ErrASTDomain
	}
	for _, constraint := range nested {
		if err = p.assignNestedDiv(constraint, z, result); err != nil {
			return err
		}
	}
	return nil
}

// assignNestedDiv is one iteration of the assign_nested_divs loop: for
// mod_term = arg_0 mod k it asserts z1 < k and (arg_0 - z1) mod k = 0,
// replaces mod_term by z1 and conjoins (coeff*z1 + (rest - z)) mod k = 0.
func (p *ArithmeticPlugin) assignNestedDiv(constraint *ArithmeticDivisibility, z ast.ASTHandle, result ast.ASTRef) (err error) {
	m := p.manager
	atm, err := m.RefGet(constraint.Atom)
	if err != nil {
		return err
	}
	z1, err := m.RefGet(constraint.Z)
	if err != nil {
		return err
	}
	z1BV, err := m.GetArg(z1, 0)
	if err != nil {
		return err
	}
	if err = p.addVar(z1BV); err != nil {
		return err
	}
	// z1 <= k - 1
	upper, err := m.ArithmeticNumeralRat(new(big.Rat).Sub(constraint.Divisor, big.NewRat(1, 1)), true)
	if err != nil {
		return err
	}
	zLtK, err := m.MkArithmeticLE(z1, upper)
	if err != nil {
		return err
	}
	if err = p.appendConstraint(false, zLtK); err != nil {
		return err
	}
	arg0, err := m.GetArg(atm, 0)
	if err != nil {
		return err
	}
	k, err := m.GetArg(atm, 1)
	if err != nil {
		return err
	}
	e1, err := m.MkArithmeticSub(arg0, z1)
	if err != nil {
		return err
	}
	mod1, err := m.MkFamilyApp(p.family, 13, nil, []ast.ASTHandle{e1, k})
	if err != nil {
		return err
	}
	modTerm2, err := m.NewRef(mod1)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&modTerm2)) }()
	if err = p.simplify(modTerm2); err != nil {
		return err
	}
	simplified, err := m.RefGet(modTerm2)
	if err != nil {
		return err
	}
	zero, err := p.util.zero(simplified)
	if err != nil {
		return err
	}
	eq, err := m.MkEq(simplified, zero)
	if err != nil {
		return err
	}
	if err = p.appendConstraint(false, eq); err != nil {
		return err
	}
	if err = p.replacer.ApplySubstitution(atm, z1, result); err != nil {
		return err
	}

	// expr_ref mod_eq(m), tmp1(m), tmp2(m);
	modEq, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&modEq)) }()
	tmp1, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&tmp1)) }()
	tmp2, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&tmp2)) }()

	coefficient, err := m.ArithmeticNumeralRat(constraint.Coefficient, true)
	if err != nil {
		return err
	}
	if err = m.SetRef(tmp2, coefficient); err != nil {
		return err
	}
	product, err := m.MkArithmeticMul(coefficient, z1)
	if err != nil {
		return err
	}
	if err = m.SetRef(tmp1, product); err != nil {
		return err
	}
	term, err := m.RefGet(constraint.Term)
	if err != nil {
		return err
	}
	difference, err := m.MkArithmeticSub(term, z)
	if err != nil {
		return err
	}
	if err = m.SetRef(tmp2, difference); err != nil {
		return err
	}
	sum, err := m.MkArithmeticAdd(product, difference)
	if err != nil {
		return err
	}
	if err = m.SetRef(tmp2, sum); err != nil {
		return err
	}
	modular, err := m.MkFamilyApp(p.family, 13, nil, []ast.ASTHandle{sum, k})
	if err != nil {
		return err
	}
	if err = m.SetRef(tmp1, modular); err != nil {
		return err
	}
	zeroZ, err := p.util.zero(z)
	if err != nil {
		return err
	}
	equation, err := m.MkEq(modular, zeroZ)
	if err != nil {
		return err
	}
	if err = m.SetRef(modEq, equation); err != nil {
		return err
	}
	if err = p.simplify(modEq); err != nil {
		return err
	}
	simplifiedEq, err := m.RefGet(modEq)
	if err != nil {
		return err
	}
	root, err := m.RefGet(result)
	if err != nil {
		return err
	}
	conjunction, err := m.MkAnd(root, simplifiedEq)
	if err != nil {
		return err
	}
	return m.SetRef(result, conjunction)
}

func (p *ArithmeticPlugin) assignDivs(x ast.ASTHandle, processor *ArithmeticBoundsProcessor, result ast.ASTRef) (substitution *arithmeticBranchSubstitution, applied bool, err error) {
	d, z, zBV, ok, release, err := p.divZRefs(processor)
	defer func() { err = errors.Join(err, release()) }()
	if err != nil || !ok {
		return nil, false, err
	}
	m := p.manager
	if err = p.addVar(zBV); err != nil {
		return nil, false, err
	}
	d1 := new(big.Rat).Sub(d, big.NewRat(1, 1))
	// z < d
	upper, err := m.ArithmeticNumeralRat(d1, true)
	if err != nil {
		return nil, false, err
	}
	zLtD, err := m.MkArithmeticLE(z, upper)
	if err != nil {
		return nil, false, err
	}
	if err = p.appendConstraint(false, zLtD); err != nil {
		return nil, false, err
	}
	// result <- result & z <= d - 1
	tmp, err := p.util.arith.MkLe2(z, upper)
	if err != nil {
		return nil, false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&tmp)) }()
	tmpHandle, err := m.RefGet(tmp)
	if err != nil {
		return nil, false, err
	}
	root, err := m.RefGet(result)
	if err != nil {
		return nil, false, err
	}
	if err = p.util.boolRw.MkAnd([]ast.ASTHandle{root, tmpHandle}, result); err != nil {
		return nil, false, err
	}
	// d | (x - z)
	difference, err := m.MkArithmeticSub(x, z)
	if err != nil {
		return nil, false, err
	}
	t1, err := m.NewRef(difference)
	if err != nil {
		return nil, false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&t1)) }()
	newAtom, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&newAtom)) }()
	if err = p.util.makeDividesInto(d, difference, newAtom); err != nil {
		return nil, false, err
	}
	atom, err := m.RefGet(newAtom)
	if err != nil {
		return nil, false, err
	}
	if err = p.appendConstraint(false, atom); err != nil {
		return nil, false, err
	}
	if err = p.mkDivEquivs(processor, z, result); err != nil {
		return nil, false, err
	}
	// x_t maps x |-> d*x + z.
	return &arithmeticBranchSubstitution{coefficient: d, term: z}, true, nil
}

// mkDivEquivs is mk_div_equivs: (c | ax + t <-> c | az + t) for each
// direct divisor.
func (p *ArithmeticPlugin) mkDivEquivs(processor *ArithmeticBoundsProcessor, z ast.ASTHandle, result ast.ASTRef) (err error) {
	m := p.manager
	// expr_ref new_atom(m), t1(m);
	newAtom, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&newAtom)) }()
	t1, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&t1)) }()
	for _, constraint := range processor.Divisibilities(false) {
		atm, err := m.RefGet(constraint.Atom)
		if err != nil {
			return err
		}
		term, err := m.RefGet(constraint.Term)
		if err != nil {
			return err
		}
		// mk_mul(coeff, z) returns z for a unit coefficient.
		scaled := z
		if constraint.Coefficient.Cmp(big.NewRat(1, 1)) != 0 {
			coefficient, err := m.ArithmeticNumeralRat(constraint.Coefficient, true)
			if err != nil {
				return err
			}
			if scaled, err = m.MkArithmeticMul(coefficient, z); err != nil {
				return err
			}
		}
		sum, err := m.MkArithmeticAdd(scaled, term)
		if err != nil {
			return err
		}
		if err = m.SetRef(t1, sum); err != nil {
			return err
		}
		if err = p.util.makeDividesInto(constraint.Divisor, sum, newAtom); err != nil {
			return err
		}
		h, err := m.RefGet(newAtom)
		if err != nil {
			return err
		}
		if err = p.replacer.ApplySubstitution(atm, h, result); err != nil {
			return err
		}
		if err = p.addConstraintNot(atm, h); err != nil {
			return err
		}
		if err = p.addConstraintNot(h, atm); err != nil {
			return err
		}
	}
	return nil
}

// addConstraintNot is m_ctx.add_constraint(false, mk_not(a), b), with the
// mk_not expr_ref temporary alive for the call.
func (p *ArithmeticPlugin) addConstraintNot(a, b ast.ASTHandle) error {
	return p.addConstraintNotVar(false, a, b)
}

// addConstraintNotVar is m_ctx.add_constraint(useVariable, mk_not(a), b).
func (p *ArithmeticPlugin) addConstraintNotVar(useVariable bool, a, b ast.ASTHandle) (err error) {
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
	return p.appendConstraint2(useVariable, negated, b)
}
