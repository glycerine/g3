package qe

// Source: src/qe/qe_arith_plugin.cpp arith_qe_util::mk_resolve, mk_big_or,
// mk_big_or_blast, mk_big_or_symbolic_blast, mk_big_or_symbolic and
// mk_bounded_var.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// ResolveUnitOrReal is arith_qe_util::mk_resolve: b*t <= a*s for opposite
// signed occurrences of x. Integer bounds with two non-unit coefficients
// add the Cooper slack disjunction; when it needs a symbolic bit-vector
// expansion the util must belong to a quant_elim_plugin context (handled
// is false otherwise).
func (u *ArithmeticQETermUtil) ResolveUnitOrReal(variable ast.ASTHandle, strict bool, a *big.Rat, t ast.ASTHandle, b *big.Rat, s ast.ASTHandle) (result ast.ASTRef, handled bool, resultErr error) {
	if resultErr = u.open(); resultErr != nil {
		return result, false, resultErr
	}
	if a == nil || b == nil || a.Sign() == 0 || b.Sign() == 0 || a.Sign() == b.Sign() {
		return result, false, ast.ErrASTDomain
	}
	m := u.manager
	one := big.NewRat(1, 1)
	absA, absB := new(big.Rat).Abs(a), new(big.Rat).Abs(b)
	integer, err := u.isInteger(variable)
	if err != nil {
		return result, false, err
	}
	// The source's integer resolution case (SASSERT: integer variables;
	// real bounds are normalized to unit coefficients).
	integerCase := integer && absA.Cmp(one) != 0 && absB.Cmp(one) != 0
	if integerCase {
		if strict || !absA.IsInt() || !absB.IsInt() {
			return result, false, ast.ErrASTDomain
		}
		minimum := absA
		if absB.Cmp(absA) < 0 {
			minimum = absB
		}
		// mk_big_or(|b|-2) with |b|-2 > 1 blasts through the context.
		if minimum.Cmp(big.NewRat(3, 1)) > 0 && (u.context == nil || u.context.owner == nil) {
			return result, false, nil
		}
	}
	defer func() {
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	bt, err := u.scale(absB, t)
	if err != nil {
		return result, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&bt)) }()
	as, err := u.scale(absA, s)
	if err != nil {
		return result, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&as)) }()
	bth, err := m.RefGet(bt)
	if err != nil {
		return result, false, err
	}
	ash, err := m.RefGet(as)
	if err != nil {
		return result, false, err
	}
	sum, err := m.MkArithmeticAdd(ash, bth)
	if err != nil {
		return result, false, err
	}
	asBt, err := m.NewRef(sum)
	if err != nil {
		return result, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&asBt)) }()
	if result, err = u.comparisonZero(sum, strict); err != nil {
		return result, false, err
	}
	if !integerCase {
		return result, true, nil
	}
	if err = u.resolveInteger(variable, a, t, b, s, ash, bth, sum, result); err != nil {
		return result, false, err
	}
	return result, true, nil
}

// resolveInteger is mk_resolve's integer resolution case:
//
//	a*s + b*t + (a-1)(b-1) <= 0
//	or exists z in [0 .. |b|-2] . |b| | (z + s) && a*n_sign(b)(s + z) + |b|t <= 0
func (u *ArithmeticQETermUtil) resolveInteger(x ast.ASTHandle, a *big.Rat, t ast.ASTHandle, b *big.Rat, s ast.ASTHandle, as, bt, asBt ast.ASTHandle, result ast.ASTRef) (err error) {
	m := u.manager
	absA, absB := new(big.Rat).Abs(a), new(big.Rat).Abs(b)
	// The block's expr_refs are released in reverse declaration order.
	var refs []*ast.ASTRef
	newRef := func(h ast.ASTHandle) (*ast.ASTRef, error) {
		ref, e := m.NewRef(h)
		if e != nil {
			return nil, e
		}
		p := &ref
		refs = append(refs, p)
		return p, nil
	}
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			if *refs[i] != (ast.ASTRef{}) {
				err = errors.Join(err, m.DelRef(refs[i]))
			}
		}
	}()
	one := big.NewRat(1, 1)
	slackValue := new(big.Rat).Mul(new(big.Rat).Sub(absA, one), new(big.Rat).Sub(absB, one))
	slackHandle, err := m.ArithmeticNumeralRat(slackValue, true)
	if err != nil {
		return err
	}
	slack, err := newRef(slackHandle)
	if err != nil {
		return err
	}
	result1, err := newRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	result2, err := newRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	current, err := m.RefGet(result)
	if err != nil {
		return err
	}
	asBtLe0, err := newRef(current)
	if err != nil {
		return err
	}
	tmp2, err := newRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	aszBtLe0, err := newRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	if _, err = newRef(ast.ASTHandle{}); err != nil { // tmp3
		return err
	}
	tmp4, err := newRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	bDividesSz, err := newRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	// a*s + b*t + (a-1)(b-1) <= 0
	sh, err := m.RefGet(*slack)
	if err != nil {
		return err
	}
	h, err := m.MkArithmeticAdd(asBt, sh)
	if err == nil {
		err = m.SetRef(*tmp2, h)
	}
	if err != nil {
		return err
	}
	if err = u.assignOwned(*result1, func() (ast.ASTRef, error) { return u.mkLe(h) }); err != nil {
		return err
	}
	a1, b1 := a, b
	if absA.Cmp(absB) < 0 {
		absA, absB = absB, absA
		a1, b1 = b1, a1
		s, t = t, s
		as, bt = bt, as
	}
	h, err = m.MkArithmeticAdd(s, x)
	if err != nil {
		return err
	}
	sz, err := newRef(h)
	if err != nil {
		return err
	}
	if b1.Sign() > 0 {
		if h, err = m.MkArithmeticNeg(h); err == nil {
			err = m.SetRef(*sz, h)
		}
		if err != nil {
			return err
		}
	}
	szh := h
	scaled, err := u.scale(a1, szh)
	if err != nil {
		return err
	}
	sch, err := m.RefGet(scaled)
	if err == nil {
		h, err = m.MkArithmeticAdd(sch, bt)
	}
	if err == nil {
		err = m.SetRef(*tmp4, h)
	}
	if err = errors.Join(err, m.DelRef(&scaled)); err != nil {
		return err
	}
	if err = u.assignOwned(*aszBtLe0, func() (ast.ASTRef, error) { return u.mkLe(h) }); err != nil {
		return err
	}
	asz, err := m.RefGet(*aszBtLe0)
	if err != nil {
		return err
	}
	direct := false
	if args, isLE, e := u.operation(asz, u.family, 2); e != nil {
		return e
	} else if isLE && len(args) == 2 && args[0] == x {
		if k, numeral, e := u.numeral(args[1]); e != nil {
			return e
		} else if numeral && k.Sign() == 0 {
			direct = true
		}
	}
	if direct {
		//    |b| | s
		if err = u.assignOwned(*tmp2, func() (ast.ASTRef, error) { return u.MakeDivides(absB, s) }); err != nil {
			return err
		}
	} else {
		if err = u.assignOwned(*bDividesSz, func() (ast.ASTRef, error) { return u.MakeDivides(absB, szh) }); err != nil {
			return err
		}
		bd, e := m.RefGet(*bDividesSz)
		if e != nil {
			return e
		}
		if e = u.boolRw.MkAnd([]ast.ASTHandle{bd, asz}, *tmp4); e != nil {
			return e
		}
		body, e := m.RefGet(*tmp4)
		if e != nil {
			return e
		}
		if e = u.mkBigOr(new(big.Rat).Sub(absB, big.NewRat(2, 1)), x, body, *tmp2); e != nil {
			return e
		}
	}
	left, err := m.RefGet(*asBtLe0)
	if err != nil {
		return err
	}
	right, err := m.RefGet(*tmp2)
	if err != nil {
		return err
	}
	if err = u.assignOwned(*result2, func() (ast.ASTRef, error) { return u.MakeFlatAnd(left, right) }); err != nil {
		return err
	}
	r1, err := m.RefGet(*result1)
	if err != nil {
		return err
	}
	r2, err := m.RefGet(*result2)
	if err != nil {
		return err
	}
	if err = u.boolRw.MkOr([]ast.ASTHandle{r1, r2}, result); err != nil {
		return err
	}
	value, err := m.RefGet(result)
	if err != nil {
		return err
	}
	return u.theory.Rewrite(value, result)
}

// assignOwned stores an owned result into dst and releases the result's
// own reference.
func (u *ArithmeticQETermUtil) assignOwned(dst ast.ASTRef, make func() (ast.ASTRef, error)) error {
	owned, err := make()
	if err != nil {
		return err
	}
	h, err := u.manager.RefGet(owned)
	if err == nil {
		err = u.manager.SetRef(dst, h)
	}
	return errors.Join(err, u.manager.DelRef(&owned))
}

// mkBigOr is arith_qe_util::mk_big_or: result receives the disjunction of
// body[x := 0 .. up].
func (u *ArithmeticQETermUtil) mkBigOr(up *big.Rat, x, body ast.ASTHandle, result ast.ASTRef) (err error) {
	m := u.manager
	replacer, err := rewriter.NewExprSafeReplace(m)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, replacer.Close()) }()
	if big.NewRat(1, 1).Cmp(up) >= 0 {
		// mk_big_or_blast
		var ors []ast.ASTRef
		defer func() {
			for i := range ors {
				err = errors.Join(err, m.DelRef(&ors[i]))
			}
		}()
		var alternatives []ast.ASTHandle
		for index := new(big.Rat); index.Cmp(up) <= 0; index.Add(index, big.NewRat(1, 1)) {
			n, e := m.ArithmeticNumeralRat(index, true)
			if e != nil {
				return e
			}
			if e = m.SetRef(result, body); e != nil {
				return e
			}
			if e = replacer.ApplySubstitution(x, n, result); e != nil {
				return e
			}
			h, e := m.RefGet(result)
			if e != nil {
				return e
			}
			ref, e := m.NewRef(h)
			if e != nil {
				return e
			}
			ors = append(ors, ref)
			alternatives = append(alternatives, h)
		}
		return u.boolRw.MkOr(alternatives, result)
	}
	// mk_big_or_symbolic_blast
	if u.context == nil || u.context.owner == nil {
		return ast.ErrASTHandle
	}
	zbv, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&zbv)) }()
	if err = u.mkBigOrSymbolic(up, x, body, replacer, zbv, result); err != nil {
		return err
	}
	bits, err := m.RefGet(zbv)
	if err != nil {
		return err
	}
	return u.context.blastOr(bits, result)
}

// mkBigOrSymbolic is mk_big_or_symbolic(up, x, body, z_bv, result) with
// mk_bounded_var.
func (u *ArithmeticQETermUtil) mkBigOrSymbolic(up *big.Rat, x, body ast.ASTHandle, replacer *rewriter.ExprSafeReplace, zbv, result ast.ASTRef) (err error) {
	m := u.manager
	n, err := m.ArithmeticNumeralRat(up, true)
	if err != nil {
		return err
	}
	e1, err := m.MkArithmeticLE(x, n)
	if err != nil {
		return err
	}
	if err = u.assignOwned(result, func() (ast.ASTRef, error) { return u.MakeFlatAnd(e1, body) }); err != nil {
		return err
	}
	// mk_bounded_var(up, z_bv, z)
	width := uint32(0)
	for value := new(big.Int).Set(up.Num()); ; {
		width++
		value.Rsh(value, 1)
		if value.Sign() <= 0 {
			break
		}
	}
	sort, err := m.BitVectorSort(width)
	if err != nil {
		return err
	}
	bits, err := m.MkFreshConst("z", sort)
	if err == nil {
		err = m.SetRef(zbv, bits)
	}
	if err != nil {
		return err
	}
	residue, err := m.MkBitVectorToInt(bits)
	if err != nil {
		return err
	}
	z, err := m.NewRef(residue)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&z)) }()
	return replacer.ApplySubstitution(x, residue, result)
}
