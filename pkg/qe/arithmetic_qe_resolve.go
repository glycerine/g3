package qe

// Source: src/qe/qe_arith_plugin.cpp arith_qe_util::mk_resolve, mk_big_or,
// mk_big_or_blast, mk_big_or_symbolic_blast, mk_big_or_symbolic and
// mk_bounded_var.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// ResolveUnitOrReal is arith_qe_util::mk_resolve: b*t <= a*s for opposite
// signed occurrences of x. Integer bounds with two non-unit coefficients
// add the Cooper slack disjunction; when it needs a symbolic bit-vector
// expansion the util must belong to a quant_elim_plugin context (handled
// is false otherwise).
func (u *ArithmeticQETermUtil) ResolveUnitOrReal(variable ast.ASTHandle, strict bool, a *unumeral.Rational, t ast.ASTHandle, b *unumeral.Rational, s ast.ASTHandle) (result ast.ASTRef, handled bool, resultErr error) {
	if resultErr = u.open(); resultErr != nil {
		return result, false, resultErr
	}
	if a == nil || b == nil {
		return result, false, ast.ErrASTDomain
	}
	q := u.q()
	aSign, err := q.Sign(a)
	if err != nil {
		return result, false, err
	}
	bSign, err := q.Sign(b)
	if err != nil {
		return result, false, err
	}
	if aSign == 0 || bSign == 0 || aSign == bSign {
		return result, false, ast.ErrASTDomain
	}
	m := u.manager
	// rational abs_a(abs(a)), abs_b(abs(b));
	absA, absB := unumeral.NewRational(), unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&absA), q.Del(&absB)) }()
	if err = errors.Join(q.Copy(&absA, a), q.Abs(&absA), q.Copy(&absB, b), q.Abs(&absB)); err != nil {
		return result, false, err
	}
	integer, err := u.isInteger(variable)
	if err != nil {
		return result, false, err
	}
	// The source's integer resolution case (SASSERT: integer variables;
	// real bounds are normalized to unit coefficients).
	aOne, err := q.IsOne(&absA)
	if err != nil {
		return result, false, err
	}
	bOne, err := q.IsOne(&absB)
	if err != nil {
		return result, false, err
	}
	integerCase := integer && !aOne && !bOne
	if integerCase {
		aInt, err := q.IsInt(&absA)
		if err != nil {
			return result, false, err
		}
		bInt, err := q.IsInt(&absB)
		if err != nil {
			return result, false, err
		}
		if strict || !aInt || !bInt {
			return result, false, ast.ErrASTDomain
		}
		minimum := &absA
		if c, err := q.Compare(&absB, &absA); err != nil {
			return result, false, err
		} else if c < 0 {
			minimum = &absB
		}
		// mk_big_or(|b|-2) with |b|-2 > 1 blasts through the context.
		small, err := q.IsInt32(minimum)
		if err != nil {
			return result, false, err
		}
		large := !small
		if small {
			v, err := q.Int32(minimum)
			if err != nil {
				return result, false, err
			}
			large = v > 3
		}
		if large && !u.context.canBlastOr() {
			return result, false, nil
		}
	}
	defer func() {
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	bt, err := u.scale(&absB, t)
	if err != nil {
		return result, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&bt)) }()
	as, err := u.scale(&absA, s)
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
	if err = u.resolveInteger(variable, &absA, &absB, a, t, b, s, ash, bth, sum, result); err != nil {
		return result, false, err
	}
	return result, true, nil
}

// resolveInteger is mk_resolve's integer resolution case:
//
//	a*s + b*t + (a-1)(b-1) <= 0
//	or exists z in [0 .. |b|-2] . |b| | (z + s) && a*n_sign(b)(s + z) + |b|t <= 0
//
// absA and absB are mk_resolve's abs_a and abs_b.
func (u *ArithmeticQETermUtil) resolveInteger(x ast.ASTHandle, absA, absB, a *unumeral.Rational, t ast.ASTHandle, b *unumeral.Rational, s ast.ASTHandle, as, bt, asBt ast.ASTHandle, result ast.ASTRef) (err error) {
	m := u.manager
	q := u.q()
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
	// mk_numeral((abs_a-numeral(1))*(abs_b-numeral(1)), true)
	a1Minus, b1Minus := unumeral.NewRational(), unumeral.NewRational()
	err = errors.Join(q.Copy(&a1Minus, absA), q.Dec(&a1Minus), q.Copy(&b1Minus, absB), q.Dec(&b1Minus))
	if err == nil {
		err = q.Mul(&a1Minus, &b1Minus, &a1Minus)
	}
	var slackHandle ast.ASTHandle
	if err == nil {
		slackHandle, err = m.ArithmeticNumeralFromRational(&a1Minus, true)
	}
	if err = errors.Join(err, q.Del(&a1Minus), q.Del(&b1Minus)); err != nil {
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
	// rational a1 = a, b1 = b;
	a1Value, b1Value := unumeral.NewRational(), unumeral.NewRational()
	defer func() { err = errors.Join(err, q.Del(&a1Value), q.Del(&b1Value)) }()
	if err = errors.Join(q.Copy(&a1Value, a), q.Copy(&b1Value, b)); err != nil {
		return err
	}
	a1, b1 := &a1Value, &b1Value
	if c, e := q.Compare(absA, absB); e != nil {
		return e
	} else if c < 0 {
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
	if positive, e := q.IsPos(b1); e != nil {
		return e
	} else if positive {
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
		if zero, e := m.ArithmeticNumeralIsZero(args[1]); e != nil {
			return e
		} else if zero {
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
		// mk_big_or(abs_b-numeral(2), ...)
		up := unumeral.NewRational()
		e = q.Copy(&up, absB)
		if e == nil {
			e = errors.Join(q.Dec(&up), q.Dec(&up))
		}
		if e == nil {
			e = u.mkBigOr(&up, x, body, *tmp2)
		}
		if e = errors.Join(e, q.Del(&up)); e != nil {
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
func (u *ArithmeticQETermUtil) mkBigOr(up *unumeral.Rational, x, body ast.ASTHandle, result ast.ASTRef) (err error) {
	m := u.manager
	q := u.q()
	replacer, err := rewriter.NewExprSafeReplace(m)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, replacer.Close()) }()
	// if (numeral(1) >= up)
	one := unumeral.NewRational()
	defer func() { err = errors.Join(err, q.Del(&one)) }()
	if err = q.SetInt32(&one, 1); err != nil {
		return err
	}
	if c, e := q.Compare(&one, up); e != nil {
		return e
	} else if c >= 0 {
		// mk_big_or_blast
		var ors []ast.ASTRef
		defer func() {
			for i := range ors {
				err = errors.Join(err, m.DelRef(&ors[i]))
			}
		}()
		var alternatives []ast.ASTHandle
		// for (numeral i(0); i <= up; ++i)
		index := unumeral.NewRational()
		defer func() { err = errors.Join(err, q.Del(&index)) }()
		for {
			if c, e := q.Compare(&index, up); e != nil {
				return e
			} else if c > 0 {
				break
			}
			n, e := m.ArithmeticNumeralFromRational(&index, true)
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
			if e = q.Inc(&index); e != nil {
				return e
			}
		}
		return u.boolRw.MkOr(alternatives, result)
	}
	// mk_big_or_symbolic_blast
	if !u.context.canBlastOr() {
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
func (u *ArithmeticQETermUtil) mkBigOrSymbolic(up *unumeral.Rational, x, body ast.ASTHandle, replacer *rewriter.ExprSafeReplace, zbv, result ast.ASTRef) (err error) {
	m := u.manager
	n, err := m.ArithmeticNumeralFromRational(up, true)
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
	residue, err := u.mkBoundedVar(up)
	if err != nil {
		return err
	}
	bits, err := m.GetArg(residue, 0)
	if err == nil {
		err = m.SetRef(zbv, bits)
	}
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
