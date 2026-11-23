package rewriter

// Source: src/ast/rewriter/arith_rewriter.cpp;
// src/ast/rewriter/arith_rewriter.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

const (
	arithOpNum    int32 = 0
	arithOpLE     int32 = 2
	arithOpGE     int32 = 3
	arithOpLT     int32 = 4
	arithOpGT     int32 = 5
	arithOpAdd    int32 = 6
	arithOpSub    int32 = 7
	arithOpUMinus int32 = 8
	arithOpMul    int32 = 9
	arithOpDiv    int32 = 10
	arithOpIDiv   int32 = 11
	arithOpRem    int32 = 12
	arithOpMod    int32 = 13
	arithOpToReal int32 = 14
	arithOpToInt  int32 = 15
	arithOpIsInt  int32 = 16
	arithOpAbs    int32 = 17
	arithOpPower  int32 = 18
)

// ArithRewriter is the concrete arithmetic theory reducer. This first source
// slice owns configuration and the exact rational scalar rewrite paths; the
// expression-polynomial and transcendental paths are added independently.
type ArithRewriter struct {
	self                                      *ArithRewriter
	manager                                   *ast.ASTManager
	family                                    ast.FamilyID
	header                                    uint64
	closed                                    bool
	expandPower, mulToPower, expandTan        bool
	arithLHS, arithIneqLHS, gcdRounding       bool
	elimToReal, pushToReal, algebraicSimplify bool
	elimRem, eq2ineq, sortSums                bool
	flat                                      bool
	sumOfMonomials, hoistMul, hoistCoeff      bool
	monomialLimit                             uint32
	maxDegree                                 uint32
}

func NewArithRewriter(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, local *uparams.ParamsRef) (*ArithRewriter, error) {
	if m == nil || c == nil || g == nil || c.SymbolsField != m.Symbols || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if _, err := m.ProofsEnabled(); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(ArithRewriter{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	r := &ArithRewriter{manager: m, header: n}
	r.self = r
	var err error
	r.family, err = m.RegisterArithmeticPlugin()
	if err == nil {
		err = r.updateParams(c, g, local)
	}
	return r, err
}

func (r *ArithRewriter) open() error {
	if r == nil || r.self != r || r.closed || r.manager == nil {
		return ast.ErrASTHandle
	}
	_, err := r.manager.ProofsEnabled()
	return err
}

// SetFlat mirrors poly_rewriter::set_flat: it changes only m_flat.
func (r *ArithRewriter) SetFlat(flat bool) error {
	if err := r.open(); err != nil {
		return err
	}
	r.flat = flat
	return nil
}

func (r *ArithRewriter) updateParams(c *uparams.ParamsContext, g *uparams.GlobalParamsService, local *uparams.ParamsRef) (result error) {
	p, err := NewArithRewriterParams(c, g, local)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, p.Close()) }()
	var options uparams.ScalarParams
	if c.ReadScalarParams(&options, p.local, &p.global) {
		r.arithLHS = settingArithLHS.read(&options)
		r.arithIneqLHS = settingArithIneqLHS.read(&options)
		r.gcdRounding = settingGCDRounding.read(&options)
		r.elimToReal = settingElimToReal.read(&options)
		r.pushToReal = settingPushToReal.read(&options)
		r.algebraicSimplify = settingAlgebraicNumberEvaluator.read(&options)
		r.expandPower = settingExpandPower.read(&options)
		r.mulToPower = settingMulToPower.read(&options)
		r.elimRem = settingElimRem.read(&options)
		r.expandTan = settingExpandTan.read(&options)
		r.eq2ineq = settingEq2Ineq.read(&options)
		r.sortSums = settingSortSums.read(&options)
		r.maxDegree = settingMaxDegree.read(&options)
	} else {
		read := func(dst *bool, f func() (bool, error)) error {
			*dst, err = f()
			return err
		}
		for _, item := range []struct {
			dst *bool
			get func() (bool, error)
		}{
			{&r.arithLHS, p.ArithLHS}, {&r.arithIneqLHS, p.ArithIneqLHS},
			{&r.gcdRounding, p.GCDRounding}, {&r.elimToReal, p.ElimToReal},
			{&r.pushToReal, p.PushToReal}, {&r.algebraicSimplify, p.AlgebraicNumberEvaluator},
			{&r.expandPower, p.ExpandPower}, {&r.mulToPower, p.MulToPower},
			{&r.elimRem, p.ElimRem}, {&r.expandTan, p.ExpandTan},
			{&r.eq2ineq, p.Eq2Ineq}, {&r.sortSums, p.SortSums},
		} {
			if err = read(item.dst, item.get); err != nil {
				return err
			}
		}
		r.maxDegree, err = p.MaxDegree()
		if err != nil {
			return err
		}
	}
	base, err := NewRewriterParams(c, g, local)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, base.Close()) }()
	var general uparams.ScalarParams
	if c.ReadScalarParams(&general, base.local, &base.global) {
		r.flat = settingFlat.read(&general)
		r.sumOfMonomials = settingSumOfMonomials.read(&general)
		r.monomialLimit = settingSumOfMonomialsBlowup.read(&general)
		r.hoistMul = settingHoistMultiplication.read(&general)
		r.hoistCoeff = settingHoistCoefficients.read(&general)
	} else {
		r.flat, err = base.Flat()
		if err == nil {
			r.sumOfMonomials, err = base.SumOfMonomials()
		}
		if err == nil {
			r.monomialLimit, err = base.SumOfMonomialsBlowup()
		}
		if err == nil {
			r.hoistMul, err = base.HoistMultiplication()
		}
		if err == nil {
			r.hoistCoeff, err = base.HoistCoefficients()
		}
	}
	if !r.flat {
		r.sumOfMonomials = false
	}
	if r.sumOfMonomials {
		r.hoistMul = false
	}
	return err
}

func (r *ArithRewriter) Close() error {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	r.manager.Budget().Release(r.header)
	r.header = 0
	r.closed = true
	return nil
}

// q is rational's g_mpq_manager: the AST manager's rational manager, on
// which every rational of the rewriter lives.
func (r *ArithRewriter) q() *unumeral.RationalManager { return r.manager.NumeralService().Manager }

// arithQDel is rational's destructor: an inline value owns no cell.
func arithQDel(q *unumeral.RationalManager, v *unumeral.Rational) error {
	if v.IsSmall() {
		return nil
	}
	return q.Del(v)
}

// arithQDel2 frees two rational locals, the second declared first.
func arithQDel2(q *unumeral.RationalManager, a, b *unumeral.Rational) error {
	return errors.Join(arithQDel(q, a), arithQDel(q, b))
}

// numeral is m_util.is_numeral(h, v, is_int): a matched numeral's value is
// copied into v (rational's copy assignment).
func (r *ArithRewriter) numeral(h ast.ASTHandle, v *unumeral.Rational) (integer, matched bool, err error) {
	kind, err := r.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, false, err
	}
	return r.manager.ArithmeticNumeralRational(h, v)
}

// isNumeral is m_util.is_numeral(h): the value is not read.
func (r *ArithRewriter) isNumeral(h ast.ASTHandle) (bool, error) {
	kind, err := r.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	return r.manager.IsArithmeticNumeral(h)
}

// makeNumeral is m_util.mk_numeral(value, is_int).
func (r *ArithRewriter) makeNumeral(value *unumeral.Rational, integer bool) (ast.ASTHandle, error) {
	return r.manager.ArithmeticNumeralFromRational(value, integer)
}

// makeNumeralInt is m_util.mk_numeral(rational(v), is_int).
func (r *ArithRewriter) makeNumeralInt(v int32, integer bool) (ast.ASTHandle, error) {
	value := unumeral.Rational{Num: unumeral.NewInt(v), Den: unumeral.NewInt(1)}
	return r.makeNumeral(&value, integer)
}

func (r *ArithRewriter) app(kind int32, args ...ast.ASTHandle) (ast.ASTHandle, error) {
	return r.manager.MkFamilyApp(r.family, kind, nil, args)
}

func (r *ArithRewriter) operation(h ast.ASTHandle, kinds ...int32) (int32, []ast.ASTHandle, bool, error) {
	nodeKind, err := r.manager.GetKind(h)
	if err != nil {
		return 0, nil, false, err
	}
	if nodeKind != ast.ASTApp {
		return 0, nil, false, nil
	}
	family, err := r.manager.GetFamilyID(h)
	if err != nil || family != r.family {
		return 0, nil, false, err
	}
	kind, err := r.manager.GetDeclKind(h)
	if err != nil {
		return 0, nil, false, err
	}
	matched := len(kinds) == 0
	for _, candidate := range kinds {
		matched = matched || kind == candidate
	}
	if !matched {
		return kind, nil, false, nil
	}
	n, err := r.manager.GetNumArgs(h)
	if err != nil {
		return 0, nil, false, err
	}
	args := make([]ast.ASTHandle, n)
	for i := range args {
		args[i], err = r.manager.GetArg(h, uint32(i))
		if err != nil {
			return 0, nil, false, err
		}
	}
	return kind, args, true, nil
}

func (r *ArithRewriter) arithmeticTerm(h ast.ASTHandle) (bool, error) {
	kind, err := r.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := r.manager.GetFamilyID(h)
	return err == nil && family == r.family, err
}

func (r *ArithRewriter) isITE(h ast.ASTHandle) (bool, error) {
	nodeKind, err := r.manager.GetKind(h)
	if err != nil || nodeKind != ast.ASTApp {
		return false, err
	}
	family, err := r.manager.GetFamilyID(h)
	if err != nil || family != r.manager.BasicFamilyID() {
		return false, err
	}
	kind, err := r.manager.GetDeclKind(h)
	return err == nil && kind == ast.ASTOpIte, err
}

func (r *ArithRewriter) setBool(out ast.ASTRef, value bool) error {
	h, err := r.manager.MkBoolVal(value)
	if err != nil {
		return err
	}
	return r.manager.SetRef(out, h)
}

// arithComparisonLocals are mk_le_ge_eq_core's expr_ref locals new_arg1,
// new_arg2, new_new_arg1 and new_new_arg2: assignment releases the old
// value, and they are released in reverse declaration order on return.
type arithComparisonLocals struct {
	manager *ast.ASTManager
	refs    [4]ast.ASTRef
}

func (l *arithComparisonLocals) set(slot int, h ast.ASTHandle) error {
	if l.refs[slot] != (ast.ASTRef{}) {
		return l.manager.SetRef(l.refs[slot], h)
	}
	ref, err := l.manager.NewRef(h)
	l.refs[slot] = ref
	return err
}

func (l *arithComparisonLocals) release() (err error) {
	for i := len(l.refs) - 1; i >= 0; i-- {
		if l.refs[i] != (ast.ASTRef{}) {
			err = errors.Join(err, l.manager.DelRef(&l.refs[i]))
		}
	}
	return err
}

func (r *ArithRewriter) rewriteNegativePolynomialComparison(kind int32, left, right ast.ASTHandle, out ast.ASTRef, locals *arithComparisonLocals) (_ bool, err error) {
	if !r.arithLHS && !r.arithIneqLHS {
		return false, nil
	}
	// numeral a2: is_numeral(arg2, a2) && is_neg_poly(arg1, new_arg1).
	rightValue := unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel(r.q(), &rightValue)) }()
	_, numeral, err := r.numeral(right, &rightValue)
	if err != nil || !numeral {
		return false, err
	}
	positive, negative, err := r.negativePolynomial(left)
	if err != nil || !negative {
		return false, err
	}
	// is_neg_poly(arg1, new_arg1); new_arg2 = mk_numeral(...).
	if err = locals.set(0, positive); err != nil {
		return false, err
	}
	if err = r.q().Neg(&rightValue); err != nil {
		return false, err
	}
	negatedRight, err := r.numeralForTerm(&rightValue, positive)
	if err != nil {
		return false, err
	}
	if err = locals.set(1, negatedRight); err != nil {
		return false, err
	}
	var result ast.ASTHandle
	if kind == arithRelationEq {
		result, err = r.manager.MkEq(positive, negatedRight)
	} else {
		result, err = r.app(invertArithmeticRelation(kind), positive, negatedRight)
	}
	if err != nil {
		return false, err
	}
	return true, r.manager.SetRef(out, result)
}

func (r *ArithRewriter) rewriteComparison(kind int32, left, right ast.ASTHandle, out ast.ASTRef) (status RewriteStatus, err error) {
	if kind == arithOpLT || kind == arithOpGT {
		a, b := right, left
		if kind == arithOpGT {
			a, b = left, right
		}
		le, err := r.app(arithOpLE, a, b)
		if err != nil {
			return RewriteFailed, err
		}
		negated, err := r.manager.MkNot(le)
		if err != nil {
			return RewriteFailed, err
		}
		return Rewrite2, r.manager.SetRef(out, negated)
	}
	newLeft, newRight, powerChanged, err := r.reduceComparisonPowers(left, right, false)
	if err != nil {
		return RewriteFailed, err
	}
	if powerChanged {
		result, e := r.app(kind, newLeft, newRight)
		if e != nil {
			return RewriteFailed, e
		}
		return Rewrite1, r.manager.SetRef(out, result)
	}
	locals := &arithComparisonLocals{manager: r.manager}
	defer func() { err = errors.Join(err, locals.release()) }()
	hold := locals.set
	newLeft, newRight, changed, err := r.cancelMonomials(left, right, r.arithIneqLHS || r.arithLHS)
	if err != nil {
		return RewriteFailed, err
	}
	if changed {
		if err = hold(0, newLeft); err == nil {
			err = hold(1, newRight)
		}
		if err != nil {
			return RewriteFailed, err
		}
		left, right = newLeft, newRight
	}
	if r.elimToReal {
		newLeft, newRight, eliminated, e := r.eliminateToRealComparison(left, right)
		if e != nil {
			return RewriteFailed, e
		}
		if eliminated {
			if e = hold(2, newLeft); e == nil {
				e = hold(3, newRight)
			}
			if e != nil {
				return RewriteFailed, e
			}
			left, right, changed = newLeft, newRight, true
		} else if newLeft != left {
			// expr_ref new_new_arg1 after a failed elim_to_real
			// (AUDIT.md FUZZ_A-F8).
			if e = hold(2, newLeft); e != nil {
				return RewriteFailed, e
			}
		}
	}
	// numeral a1, a2: they live to the end of the function.
	lv, rv := unumeral.NewRational(), unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel2(r.q(), &rv, &lv)) }()
	_, ln, err := r.numeral(left, &lv)
	if err != nil {
		return RewriteFailed, err
	}
	rn := false
	if ln {
		_, rn, err = r.numeral(right, &rv)
	}
	if err != nil || !ln || !rn {
		if err == nil {
			if decided, e := r.rewriteAlgebraicComparison(left, right, kind, out); e != nil || decided {
				if e != nil {
					return RewriteFailed, e
				}
				return RewriteDone, nil
			}
			if decided, e := r.rewriteBound(left, right, kind, out); e != nil || decided {
				if e != nil {
					return RewriteFailed, e
				}
				return RewriteDone, nil
			}
			if decided, e := r.rewriteBound(right, left, invertArithmeticRelation(kind), out); e != nil || decided {
				if e != nil {
					return RewriteFailed, e
				}
				return RewriteDone, nil
			}
		}
		if r.gcdRounding {
			newLeft, newRight, reduced, impossible, e := r.reducePolynomialGCD(left, right, kind, func(h ast.ASTHandle) error { return locals.set(0, h) })
			if e != nil {
				return RewriteFailed, e
			}
			if impossible {
				return RewriteDone, r.setBool(out, false)
			}
			if reduced {
				// div_polynomial wrote new_arg1; now new_arg2.
				if e = locals.set(1, newRight); e != nil {
					return RewriteFailed, e
				}
				left, right, changed = newLeft, newRight, true
			}
		}
		if rewritten, e := r.rewriteNegativePolynomialComparison(kind, left, right, out, locals); e != nil || rewritten {
			return RewriteDone, e
		}
		if changed && err == nil {
			result, e := r.app(kind, left, right)
			if e != nil {
				return RewriteFailed, e
			}
			return RewriteDone, r.manager.SetRef(out, result)
		}
		return RewriteFailed, err
	}
	cmp, err := r.q().Compare(&lv, &rv)
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, r.setBool(out, kind == arithOpLE && cmp <= 0 || kind == arithOpGE && cmp >= 0)
}

func (r *ArithRewriter) rewriteUnaryMinus(arg ast.ASTHandle, out ast.ASTRef) (_ RewriteStatus, err error) {
	// numeral a
	v := unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel(r.q(), &v)) }()
	integer, matched, err := r.numeral(arg, &v)
	if err != nil {
		return RewriteFailed, err
	}
	if matched {
		if err = r.q().Neg(&v); err != nil {
			return RewriteFailed, err
		}
		h, e := r.makeNumeral(&v, integer)
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteDone, r.manager.SetRef(out, h)
	}
	minusOne, err := r.numeralForTermInt(-1, arg)
	if err != nil {
		return RewriteFailed, err
	}
	h, err := r.app(arithOpMul, minusOne, arg)
	if err != nil {
		return RewriteFailed, err
	}
	return Rewrite1, r.manager.SetRef(out, h)
}

func (r *ArithRewriter) rewriteDivision(kind int32, left, right ast.ASTHandle, out ast.ASTRef) (_ RewriteStatus, err error) {
	if kind == arithOpDiv {
		status, dispatched, err := r.rewriteAlgebraicDivision(left, right, out)
		if err != nil || dispatched {
			return status, err
		}
	}
	q := r.q()
	// numeral v1, v2
	lv, rv := unumeral.NewRational(), unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel2(q, &rv, &lv)) }()
	li, ln, err := r.numeral(left, &lv)
	if err != nil {
		return RewriteFailed, err
	}
	ri, rn, err := r.numeral(right, &rv)
	if err != nil {
		return RewriteFailed, err
	}
	if kind == arithOpDiv {
		if rn {
			if rv.IsZero() {
				return RewriteFailed, nil
			}
			if ln {
				// v1/v2
				if err = q.Div(&lv, &rv, &lv); err != nil {
					return RewriteFailed, err
				}
				h, e := r.makeNumeral(&lv, false)
				if e != nil {
					return RewriteFailed, e
				}
				return RewriteDone, r.manager.SetRef(out, h)
			}
			// rational k(1); k /= v2
			if err = q.Inv(&rv, &rv); err != nil {
				return RewriteFailed, err
			}
			c, e := r.makeNumeral(&rv, false)
			if e != nil {
				return RewriteFailed, e
			}
			h, e := r.app(arithOpMul, c, left)
			if e != nil {
				return RewriteFailed, e
			}
			return Rewrite1, r.manager.SetRef(out, h)
		}
		integer, e := r.integerTerm(left)
		if e != nil || integer {
			return RewriteFailed, e
		}
		// (/ (* v1 b) (* v2 d)) --> (* v1/v2 (/ b d)): v1 and v2 are
		// reused, set to one when there is no coefficient.
		extract := func(h ast.ASTHandle, coefficient *unumeral.Rational) (ast.ASTHandle, error) {
			_, factors, product, e := r.operation(h, arithOpMul)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if product && len(factors) == 2 {
				_, numeral, e := r.numeral(factors[0], coefficient)
				if e != nil {
					return ast.ASTHandle{}, e
				}
				if numeral {
					return factors[1], nil
				}
			}
			return h, q.SetInt32(coefficient, 1)
		}
		numerator, e := extract(left, &lv)
		if e != nil {
			return RewriteFailed, e
		}
		denominator, e := extract(right, &rv)
		if e != nil {
			return RewriteFailed, e
		}
		if !lv.IsOne() || !rv.IsOne() {
			if e = q.Div(&lv, &rv, &lv); e != nil {
				return RewriteFailed, e
			}
			coefficient, e := r.makeNumeral(&lv, false)
			if e != nil {
				return RewriteFailed, e
			}
			division, e := r.app(arithOpDiv, numerator, denominator)
			if e != nil {
				return RewriteFailed, e
			}
			result, e := r.app(arithOpMul, coefficient, division)
			if e != nil {
				return RewriteFailed, e
			}
			return Rewrite2, r.manager.SetRef(out, result)
		}
		return RewriteFailed, nil
	}
	if !rn || rv.IsZero() || !ln || !li || !ri {
		return RewriteFailed, nil
	}
	switch kind {
	case arithOpIDiv:
		// div(v1, v2)
		err = q.IDiv(&lv, &rv, &lv)
	case arithOpMod, arithOpRem:
		// mod(v1, v2)
		err = q.Mod(&lv, &rv, &lv)
		if err == nil && kind == arithOpRem && rv.IsNeg() {
			err = q.Neg(&lv)
		}
	default:
		return RewriteFailed, nil
	}
	if err != nil {
		return RewriteFailed, err
	}
	h, err := r.makeNumeral(&lv, true)
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, h)
}

func (r *ArithRewriter) rewriteMod(left, right ast.ASTHandle, out ast.ASTRef) (_ RewriteStatus, err error) {
	q := r.q()
	// numeral v1, v2
	lv, rv := unumeral.NewRational(), unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel2(q, &rv, &lv)) }()
	li, ln, err := r.numeral(left, &lv)
	if err != nil {
		return RewriteFailed, err
	}
	ri, rn, err := r.numeral(right, &rv)
	if err != nil {
		return RewriteFailed, err
	}
	if ln && rn && !rv.IsZero() && li && ri {
		// mod(v1, v2)
		if err = q.Mod(&lv, &rv, &lv); err != nil {
			return RewriteFailed, err
		}
		h, e := r.makeNumeral(&lv, true)
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteDone, r.manager.SetRef(out, h)
	}
	if rn && ri && rv.IsOne() {
		h, e := r.makeNumeralInt(0, true)
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteDone, r.manager.SetRef(out, h)
	}
	if left == right && !rn {
		zero, e := r.makeNumeralInt(0, true)
		if e != nil {
			return RewriteFailed, e
		}
		absRef, e := r.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			return RewriteFailed, e
		}
		defer func() { _ = r.manager.DelRef(&absRef) }()
		if _, e = r.rewriteAbs(right, absRef); e != nil {
			return RewriteFailed, e
		}
		absolute, e := r.manager.RefGet(absRef)
		if e != nil {
			return RewriteFailed, e
		}
		undefined, e := r.app(arithOpMod, zero, zero)
		if e != nil {
			return RewriteFailed, e
		}
		equal, e := r.manager.MkEq(right, zero)
		if e != nil {
			return RewriteFailed, e
		}
		result, e := r.manager.MkIte(equal, undefined, absolute)
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteDone, r.manager.SetRef(out, result)
	}
	if _, inner, yes, e := r.operation(left, arithOpMod); e != nil {
		return RewriteFailed, e
	} else if yes && len(inner) == 2 && inner[1] == right && rn && ri && !rv.IsZero() {
		return RewriteDone, r.manager.SetRef(out, left)
	}
	if !rn || !ri || !rv.IsPos() {
		return RewriteFailed, nil
	}
	kind, children, composite, err := r.operation(left, arithOpAdd, arithOpMul)
	if err != nil || !composite {
		return RewriteFailed, err
	}
	target := false
	// rational arg_v (v1 is free here: the numeral case returned above)
	for _, child := range children {
		if _, _, isMod, e := r.operation(child, arithOpMod); e != nil {
			return RewriteFailed, e
		} else if isMod {
			continue
		}
		if _, numeral, e := r.numeral(child, &lv); e != nil {
			return RewriteFailed, e
		} else if numeral {
			// mod(arg_v, v2) == arg_v
			m := unumeral.NewRational()
			e = q.Mod(&lv, &rv, &m)
			same := false
			if e == nil {
				same, e = q.Equal(&m, &lv)
			}
			if e = errors.Join(e, arithQDel(q, &m)); e != nil {
				return RewriteFailed, e
			}
			if same {
				continue
			}
		}
		if ite, e := r.isITE(child); e != nil {
			return RewriteFailed, e
		} else if ite {
			continue
		}
		target = true
		break
	}
	if !target {
		return RewriteFailed, nil
	}
	wrapped := make([]ast.ASTHandle, len(children))
	for i, child := range children {
		wrapped[i], err = r.app(arithOpMod, child, right)
		if err != nil {
			return RewriteFailed, err
		}
	}
	inside, err := r.app(kind, wrapped...)
	if err != nil {
		return RewriteFailed, err
	}
	result, err := r.app(arithOpMod, inside, right)
	if err != nil {
		return RewriteFailed, err
	}
	return Rewrite3, r.manager.SetRef(out, result)
}

func (r *ArithRewriter) rewriteRem(left, right ast.ASTHandle, out ast.ASTRef) (_ RewriteStatus, err error) {
	q := r.q()
	// numeral v1, v2
	lv, rv := unumeral.NewRational(), unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel2(q, &rv, &lv)) }()
	li, ln, err := r.numeral(left, &lv)
	if err != nil {
		return RewriteFailed, err
	}
	ri, rn, err := r.numeral(right, &rv)
	if err != nil {
		return RewriteFailed, err
	}
	if ln && rn && !rv.IsZero() && li && ri {
		// numeral m = mod(v1, v2); if (v2.is_neg()) m.neg();
		err = q.Mod(&lv, &rv, &lv)
		if err == nil && rv.IsNeg() {
			err = q.Neg(&lv)
		}
		if err != nil {
			return RewriteFailed, err
		}
		h, e := r.makeNumeral(&lv, true)
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteDone, r.manager.SetRef(out, h)
	}
	if rn && ri && rv.IsOne() {
		h, e := r.makeNumeralInt(0, true)
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteDone, r.manager.SetRef(out, h)
	}
	if rn && ri && !rv.IsZero() {
		if _, _, composite, e := r.operation(left, arithOpAdd, arithOpMul); e != nil {
			return RewriteFailed, e
		} else if composite {
			return RewriteFailed, nil
		}
		mod, e := r.app(arithOpMod, left, right)
		if e != nil {
			return RewriteFailed, e
		}
		if rv.IsPos() {
			return Rewrite1, r.manager.SetRef(out, mod)
		}
		negative, e := r.app(arithOpUMinus, mod)
		if e != nil {
			return RewriteFailed, e
		}
		return Rewrite2, r.manager.SetRef(out, negative)
	}
	if !r.elimRem {
		return RewriteFailed, nil
	}
	mod, err := r.app(arithOpMod, left, right)
	if err != nil {
		return RewriteFailed, err
	}
	negative, err := r.app(arithOpUMinus, mod)
	if err != nil {
		return RewriteFailed, err
	}
	zero, err := r.makeNumeralInt(0, true)
	if err != nil {
		return RewriteFailed, err
	}
	nonnegative, err := r.app(arithOpGE, right, zero)
	if err != nil {
		return RewriteFailed, err
	}
	result, err := r.manager.MkIte(nonnegative, mod, negative)
	if err != nil {
		return RewriteFailed, err
	}
	return Rewrite3, r.manager.SetRef(out, result)
}

// arithQUnsigned is rational::is_unsigned() && get_unsigned().
func arithQUnsigned(q *unumeral.RationalManager, v *unumeral.Rational) (uint32, bool, error) {
	ok, err := q.IsUnsigned(v)
	if err != nil || !ok {
		return 0, false, err
	}
	u, err := q.Unsigned(v)
	return u, err == nil, err
}

// rewritePower is arith_rewriter::mk_power_core.
func (r *ArithRewriter) rewritePower(base, exponent ast.ASTHandle, out ast.ASTRef) (_ RewriteStatus, err error) {
	q := r.q()
	// numeral x, y
	x, y := unumeral.NewRational(), unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel2(q, &y, &x)) }()
	_, xn, err := r.numeral(base, &x)
	if err != nil {
		return RewriteFailed, err
	}
	_, yn, err := r.numeral(exponent, &y)
	if err != nil {
		return RewriteFailed, err
	}
	integer, err := r.integerTerm(base)
	if err != nil {
		return RewriteFailed, err
	}
	if xn && x.IsOne() || yn && y.IsOne() {
		return RewriteDone, r.manager.SetRef(out, base)
	}
	if xn && yn {
		if x.IsZero() && y.IsZero() {
			return RewriteFailed, nil
		}
		if y.IsZero() {
			h, e := r.makeNumeralInt(1, integer)
			if e != nil {
				return RewriteFailed, e
			}
			return RewriteDone, r.manager.SetRef(out, h)
		}
		if x.IsZero() {
			return RewriteDone, r.manager.SetRef(out, base)
		}
		k, ok, e := arithQUnsigned(q, &y)
		if e != nil {
			return RewriteFailed, e
		}
		if ok && k <= r.maxDegree {
			// x = power(x, y.get_unsigned())
			if e = q.Power(&x, k, &x); e != nil {
				return RewriteFailed, e
			}
			h, e := r.makeNumeral(&x, integer)
			if e != nil {
				return RewriteFailed, e
			}
			return RewriteDone, r.manager.SetRef(out, h)
		}
		if !integer {
			// (-y).is_unsigned() && (-y).get_unsigned() <= m_max_degree
			negY, e := arithQNegated(q, &y)
			if e != nil {
				return RewriteFailed, e
			}
			k, ok, e = arithQUnsigned(q, &negY)
			if e = errors.Join(e, arithQDel(q, &negY)); e != nil {
				return RewriteFailed, e
			}
			if ok && k <= r.maxDegree {
				// x = power(rational(1)/x, (-y).get_unsigned())
				if e = q.Inv(&x, &x); e == nil {
					e = q.Power(&x, k, &x)
				}
				if e != nil {
					return RewriteFailed, e
				}
				h, e := r.makeNumeral(&x, integer)
				if e != nil {
					return RewriteFailed, e
				}
				return RewriteDone, r.manager.SetRef(out, h)
			}
		}
	}
	if _, inner, nested, e := r.operation(base, arithOpPower); e != nil {
		return RewriteFailed, e
	} else if nested && len(inner) == 2 && yn && y.IsInt() && !y.IsZero() {
		// (^ (^ t y2) y) --> (^ t (* y2 y))  If y2 > 0 && y != 0 && y and y2 are integers
		// rational y2
		y2 := unumeral.NewRational()
		_, numeral, e := r.numeral(inner[1], &y2)
		if e == nil && numeral && y2.IsInt() && y2.IsPos() {
			var combinedExponent ast.ASTHandle
			if e = q.Mul(&y, &y2, &y2); e == nil {
				combinedExponent, e = r.makeNumeral(&y2, integer)
			}
			var result ast.ASTHandle
			if e == nil {
				result, e = r.app(arithOpPower, inner[0], combinedExponent)
			}
			if e = errors.Join(e, arithQDel(q, &y2)); e != nil {
				return RewriteFailed, e
			}
			return Rewrite1, r.manager.SetRef(out, result)
		}
		if e = errors.Join(e, arithQDel(q, &y2)); e != nil {
			return RewriteFailed, e
		}
	}
	if !integer && yn && y.IsNeg() {
		if !xn && r.algebraicSimplify && y.IsInt() && y.Num.IsSmall() {
			k := uint32(-int64(y.Num.Val))
			if k >= 2 && k <= r.maxDegree {
				status, err := r.rewriteBinomialPower(base, k, true, out)
				if err != nil || status != RewriteFailed {
					return status, err
				}
			}
		}
		// (^ t -k) --> (^ (/ 1 t) k)
		one, e := r.makeNumeralInt(1, false)
		if e != nil {
			return RewriteFailed, e
		}
		inverse, e := r.app(arithOpDiv, one, base)
		if e != nil {
			return RewriteFailed, e
		}
		negY, e := arithQNegated(q, &y)
		if e != nil {
			return RewriteFailed, e
		}
		positive, e := r.makeNumeral(&negY, false)
		if e = errors.Join(e, arithQDel(q, &negY)); e != nil {
			return RewriteFailed, e
		}
		result, e := r.app(arithOpPower, inverse, positive)
		if e != nil {
			return RewriteFailed, e
		}
		return Rewrite2, r.manager.SetRef(out, result)
	}
	if !integer && yn && !y.IsInt() && !(y.Num.IsSmall() && y.Num.Val == 1) {
		// (^ t (/ p q)) --> (^ (^ t (/ 1 q)) p)
		part := unumeral.NewRational()
		// rational(1)/denominator(y)
		e := q.DenominatorRational(&y, &part)
		if e == nil {
			e = q.Inv(&part, &part)
		}
		var rootExponent, root, numerator ast.ASTHandle
		if e == nil {
			rootExponent, e = r.makeNumeral(&part, false)
		}
		if e == nil {
			root, e = r.app(arithOpPower, base, rootExponent)
		}
		// numerator(y)
		if e == nil {
			e = q.NumeratorRational(&y, &part)
		}
		if e == nil {
			numerator, e = r.makeNumeral(&part, false)
		}
		if e = errors.Join(e, arithQDel(q, &part)); e != nil {
			return RewriteFailed, e
		}
		result, e := r.app(arithOpPower, root, numerator)
		if e != nil {
			return RewriteFailed, e
		}
		return Rewrite2, r.manager.SetRef(out, result)
	}
	arithmeticBase, err := r.arithmeticTerm(base)
	if err != nil {
		return RewriteFailed, err
	}
	if r.expandPower || r.sumOfMonomials && arithmeticBase {
		k, ok := uint32(0), false
		if yn {
			if k, ok, err = arithQUnsigned(q, &y); err != nil {
				return RewriteFailed, err
			}
		}
		if ok && 1 < k && k <= r.maxDegree {
			factors := make([]ast.ASTHandle, k)
			for i := range factors {
				factors[i] = base
			}
			result, e := r.app(arithOpMul, factors...)
			if e != nil {
				return RewriteFailed, e
			}
			return Rewrite1, r.manager.SetRef(out, result)
		}
	}
	if !yn {
		return RewriteFailed, nil
	}
	irrational, err := r.algebraicNumeralSyntax(base)
	if err != nil || !xn && !irrational {
		return RewriteFailed, err
	}
	// rational num_y = numerator(y); rational den_y = denominator(y);
	numY, denY := unumeral.NewRational(), unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel2(q, &denY, &numY)) }()
	if err = q.NumeratorRational(&y, &numY); err == nil {
		err = q.DenominatorRational(&y, &denY)
	}
	if err != nil {
		return RewriteFailed, err
	}
	negativeY := false
	if numY.IsNeg() {
		if err = q.Neg(&numY); err != nil {
			return RewriteFailed, err
		}
		negativeY = true
	}
	if negativeY && integer {
		return RewriteFailed, nil
	}
	uNumY, numOK, err := arithQUnsigned(q, &numY)
	if err != nil {
		return RewriteFailed, err
	}
	uDenY, denOK := uint32(0), false
	if numOK {
		if uDenY, denOK, err = arithQUnsigned(q, &denY); err != nil {
			return RewriteFailed, err
		}
	}
	if !numOK || !denOK {
		return RewriteFailed, nil
	}
	if uNumY > r.maxDegree || uDenY > r.maxDegree {
		return RewriteFailed, nil
	}
	if xn {
		// rational xk, r
		xk, root := unumeral.NewRational(), unumeral.NewRational()
		defer func() { err = errors.Join(err, arithQDel2(q, &root, &xk)) }()
		if err = q.Power(&x, uNumY, &xk); err != nil {
			return RewriteFailed, err
		}
		if xk.IsNeg() && uDenY%2 == 0 {
			return RewriteFailed, nil
		}
		exact, e := q.Root(&xk, uDenY, &root)
		if e != nil {
			return RewriteFailed, e
		}
		if exact {
			if negativeY {
				if e = q.Inv(&root, &root); e != nil {
					return RewriteFailed, e
				}
			}
			h, e := r.makeNumeral(&root, integer)
			if e != nil {
				return RewriteFailed, e
			}
			return RewriteDone, r.manager.SetRef(out, h)
		}
		if r.algebraicSimplify {
			return r.rewriteAlgebraicRationalRoot(&xk, uDenY, negativeY, out)
		}
		return RewriteFailed, nil
	}
	if !r.algebraicSimplify {
		return RewriteFailed, nil
	}
	return r.rewriteAlgebraicPower(base, uNumY, uDenY, negativeY, out)
}

// arithQNegated is a fresh -v (rational's unary minus).
func arithQNegated(q *unumeral.RationalManager, v *unumeral.Rational) (unumeral.Rational, error) {
	out := unumeral.NewRational()
	err := q.Copy(&out, v)
	if err == nil {
		err = q.Neg(&out)
	}
	return out, err
}

func (r *ArithRewriter) rewriteToInt(arg ast.ASTHandle, out ast.ASTRef) (_ RewriteStatus, err error) {
	q := r.q()
	// numeral a
	v := unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel(q, &v)) }()
	_, matched, err := r.numeral(arg, &v)
	if err != nil {
		return RewriteFailed, err
	}
	if matched {
		// floor(a)
		if err = q.Floor(&v, &v); err != nil {
			return RewriteFailed, err
		}
		h, e := r.makeNumeral(&v, true)
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteDone, r.manager.SetRef(out, h)
	}
	if yes, child, e := r.unary(arg, arithOpToReal); e != nil {
		return RewriteFailed, e
	} else if yes {
		return RewriteDone, r.manager.SetRef(out, child)
	}
	kind, children, composite, err := r.operation(arg, arithOpAdd, arithOpMul, arithOpPower)
	if err != nil || !composite {
		return RewriteFailed, err
	}
	converted := make([]ast.ASTHandle, len(children))
	for i, child := range children {
		_, numeral, e := r.numeral(child, &v)
		if e != nil {
			return RewriteFailed, e
		}
		if numeral && v.IsInt() {
			converted[i], e = r.makeNumeral(&v, true)
			if e != nil {
				return RewriteFailed, e
			}
			continue
		}
		if yes, inner, e := r.unary(child, arithOpToReal); e != nil {
			return RewriteFailed, e
		} else if yes {
			converted[i] = inner
			continue
		}
		return RewriteFailed, nil
	}
	result, err := r.app(kind, converted...)
	if err != nil {
		return RewriteFailed, err
	}
	return Rewrite1, r.manager.SetRef(out, result)
}

func (r *ArithRewriter) rewriteToReal(arg ast.ASTHandle, out ast.ASTRef) (_ RewriteStatus, err error) {
	// numeral a
	v := unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel(r.q(), &v)) }()
	_, matched, err := r.numeral(arg, &v)
	if err != nil {
		return RewriteFailed, err
	}
	if matched {
		h, e := r.makeNumeral(&v, false)
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteDone, r.manager.SetRef(out, h)
	}
	if r.pushToReal {
		kind, children, composite, e := r.operation(arg, arithOpAdd, arithOpMul)
		if e != nil {
			return RewriteFailed, e
		}
		if composite {
			converted := make([]ast.ASTHandle, len(children))
			for i, child := range children {
				converted[i], e = r.app(arithOpToReal, child)
				if e != nil {
					return RewriteFailed, e
				}
			}
			result, e := r.app(kind, converted...)
			if e != nil {
				return RewriteFailed, e
			}
			return Rewrite2, r.manager.SetRef(out, result)
		}
	}
	return RewriteFailed, nil
}

func (r *ArithRewriter) unary(h ast.ASTHandle, kind int32) (bool, ast.ASTHandle, error) {
	nodeKind, err := r.manager.GetKind(h)
	if err != nil || nodeKind != ast.ASTApp {
		return false, ast.ASTHandle{}, err
	}
	family, err := r.manager.GetFamilyID(h)
	if err != nil || family != r.family {
		return false, ast.ASTHandle{}, err
	}
	k, err := r.manager.GetDeclKind(h)
	if err != nil || k != kind {
		return false, ast.ASTHandle{}, err
	}
	n, err := r.manager.GetNumArgs(h)
	if err != nil || n != 1 {
		return false, ast.ASTHandle{}, err
	}
	arg, err := r.manager.GetArg(h, 0)
	return err == nil, arg, err
}

func (r *ArithRewriter) rewriteIsInt(arg ast.ASTHandle, out ast.ASTRef) (_ RewriteStatus, err error) {
	// numeral a
	v := unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel(r.q(), &v)) }()
	_, matched, err := r.numeral(arg, &v)
	if err != nil {
		return RewriteFailed, err
	}
	if matched {
		return RewriteDone, r.setBool(out, v.IsInt())
	}
	if yes, _, e := r.unary(arg, arithOpToReal); e != nil {
		return RewriteFailed, e
	} else if yes {
		return RewriteDone, r.setBool(out, true)
	}
	toInt, err := r.app(arithOpToInt, arg)
	if err != nil {
		return RewriteFailed, err
	}
	toReal, err := r.app(arithOpToReal, toInt)
	if err != nil {
		return RewriteFailed, err
	}
	equality, err := r.manager.MkEq(toReal, arg)
	if err != nil {
		return RewriteFailed, err
	}
	return Rewrite3, r.manager.SetRef(out, equality)
}

func (r *ArithRewriter) rewriteAbs(arg ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	// m_util.is_int(arg)
	integer, err := r.integerTerm(arg)
	if err != nil {
		return RewriteFailed, err
	}
	// mk_ite(mk_ge(arg, mk_numeral(0)), arg, mk_uminus(arg)): z3 4.7.1's
	// g++ build evaluates the call arguments right to left, so the
	// negation is created before the zero and the comparison.
	negative, err := r.app(arithOpUMinus, arg)
	if err != nil {
		return RewriteFailed, err
	}
	zero, err := r.makeNumeralInt(0, integer)
	if err != nil {
		return RewriteFailed, err
	}
	ge, err := r.app(arithOpGE, arg, zero)
	if err != nil {
		return RewriteFailed, err
	}
	result, err := r.manager.MkIte(ge, arg, negative)
	if err != nil {
		return RewriteFailed, err
	}
	return Rewrite2, r.manager.SetRef(out, result)
}

// MkEqCore performs the arithmetic equality dispatch used by the Boolean and
// combined theory rewriters.
func (r *ArithRewriter) MkEqCore(left, right ast.ASTHandle, out ast.ASTRef) (status RewriteStatus, err error) {
	locals := &arithComparisonLocals{manager: r.manager}
	defer func() { err = errors.Join(err, locals.release()) }()
	if err := r.open(); err != nil {
		return RewriteFailed, err
	}
	if r.eq2ineq {
		// mk_and(mk_le(a, b), mk_ge(a, b)): the reference build (GCC, x86-64)
		// evaluates the call arguments right to left, so the ge atom is
		// created, and takes its AST id, first.
		ge, err := r.app(arithOpGE, left, right)
		if err != nil {
			return RewriteFailed, err
		}
		le, err := r.app(arithOpLE, left, right)
		if err != nil {
			return RewriteFailed, err
		}
		both, err := r.manager.MkAndArgs([]ast.ASTHandle{le, ge})
		if err != nil {
			return RewriteFailed, err
		}
		return Rewrite2, r.manager.SetRef(out, both)
	}
	newLeft, newRight, powerChanged, err := r.reduceComparisonPowers(left, right, true)
	if err != nil {
		return RewriteFailed, err
	}
	if powerChanged {
		result, e := r.manager.MkEq(newLeft, newRight)
		if e != nil {
			return RewriteFailed, e
		}
		return Rewrite1, r.manager.SetRef(out, result)
	}
	leftArithmetic, err := r.arithmeticTerm(left)
	if err != nil {
		return RewriteFailed, err
	}
	rightArithmetic, err := r.arithmeticTerm(right)
	if err != nil {
		return RewriteFailed, err
	}
	changed := false
	if r.arithLHS || leftArithmetic || rightArithmetic {
		newLeft, newRight, cancelled, e := r.cancelMonomials(left, right, r.arithLHS || r.arithIneqLHS)
		err = e
		if err != nil {
			return RewriteFailed, err
		}
		changed = cancelled
		if changed {
			if err = locals.set(0, newLeft); err == nil {
				err = locals.set(1, newRight)
			}
			if err != nil {
				return RewriteFailed, err
			}
			left, right = newLeft, newRight
		}
	}
	if r.elimToReal {
		newLeft, newRight, eliminated, e := r.eliminateToRealComparison(left, right)
		if e != nil {
			return RewriteFailed, e
		}
		if eliminated {
			if e = locals.set(2, newLeft); e == nil {
				e = locals.set(3, newRight)
			}
			if e != nil {
				return RewriteFailed, e
			}
			left, right, changed = newLeft, newRight, true
		} else if newLeft != left {
			// expr_ref new_new_arg1 after a failed elim_to_real
			// (AUDIT.md FUZZ_A-F8).
			if e = locals.set(2, newLeft); e != nil {
				return RewriteFailed, e
			}
		}
	}
	// numeral a1, a2 of mk_le_ge_eq_core
	lv, rv := unumeral.NewRational(), unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel2(r.q(), &rv, &lv)) }()
	_, ln, err := r.numeral(left, &lv)
	if err != nil {
		return RewriteFailed, err
	}
	rn := false
	if ln {
		_, rn, err = r.numeral(right, &rv)
	}
	if err != nil || !ln || !rn {
		if err == nil {
			if decided, e := r.rewriteAlgebraicComparison(left, right, arithRelationEq, out); e != nil || decided {
				if e != nil {
					return RewriteFailed, e
				}
				return RewriteDone, nil
			}
			if decided, e := r.rewriteBound(left, right, arithRelationEq, out); e != nil || decided {
				if e != nil {
					return RewriteFailed, e
				}
				return RewriteDone, nil
			}
			if decided, e := r.rewriteBound(right, left, arithRelationEq, out); e != nil || decided {
				if e != nil {
					return RewriteFailed, e
				}
				return RewriteDone, nil
			}
		}
		if r.gcdRounding {
			newLeft, newRight, reduced, impossible, e := r.reducePolynomialGCD(left, right, arithRelationEq, func(h ast.ASTHandle) error { return locals.set(0, h) })
			if e != nil {
				return RewriteFailed, e
			}
			if impossible {
				return RewriteDone, r.setBool(out, false)
			}
			if reduced {
				// div_polynomial wrote new_arg1; now new_arg2.
				if e = locals.set(1, newRight); e != nil {
					return RewriteFailed, e
				}
				left, right, changed = newLeft, newRight, true
			}
		}
		if rewritten, e := r.rewriteNegativePolynomialComparison(arithRelationEq, left, right, out, locals); e != nil || rewritten {
			return RewriteDone, e
		}
		if changed && err == nil {
			result, e := r.manager.MkEq(left, right)
			if e != nil {
				return RewriteFailed, e
			}
			return RewriteDone, r.manager.SetRef(out, result)
		}
		return RewriteFailed, err
	}
	equal, err := r.q().Equal(&lv, &rv)
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, r.setBool(out, equal)
}

// MkAppCore applies one arithmetic declaration to already rewritten arguments.
func (r *ArithRewriter) MkAppCore(decl ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	if err := r.open(); err != nil {
		return RewriteFailed, err
	}
	family, err := r.manager.GetFamilyID(decl)
	if err != nil || family != r.family {
		return RewriteFailed, err
	}
	kind, err := r.manager.GetDeclKind(decl)
	if err != nil {
		return RewriteFailed, err
	}
	at := func(i uint32) (ast.ASTHandle, error) { return args.At(i) }
	if kind == arithOpNum {
		return RewriteFailed, nil
	}
	if kind == arithOpAdd {
		status, e := r.rewriteAlgebraicFold(kind, args, out)
		if e != nil || status != RewriteFailed {
			return status, e
		}
		return r.rewriteAdd(args, out)
	}
	if kind == arithOpMul {
		status, e := r.rewriteAlgebraicFold(kind, args, out)
		if e != nil || status != RewriteFailed {
			return status, e
		}
		return r.rewriteMul(args, out)
	}
	if kind == arithOpSub {
		return r.rewriteSub(args, out)
	}
	if kind == arithOpDiv || kind == arithOpIDiv {
		if args.Count() == 1 {
			h, e := at(0)
			if e != nil {
				return RewriteFailed, e
			}
			return RewriteDone, r.manager.SetRef(out, h)
		}
	}
	if kind == arithOpLE || kind == arithOpGE || kind == arithOpLT || kind == arithOpGT || kind == arithOpDiv || kind == arithOpIDiv || kind == arithOpMod || kind == arithOpRem || kind == arithOpPower {
		if args.Count() != 2 {
			return RewriteFailed, ast.ErrASTDomain
		}
		left, e := at(0)
		if e != nil {
			return RewriteFailed, e
		}
		right, e := at(1)
		if e != nil {
			return RewriteFailed, e
		}
		switch kind {
		case arithOpLE, arithOpGE, arithOpLT, arithOpGT:
			return r.rewriteComparison(kind, left, right, out)
		case arithOpPower:
			return r.rewritePower(left, right, out)
		case arithOpMod:
			return r.rewriteMod(left, right, out)
		case arithOpRem:
			return r.rewriteRem(left, right, out)
		default:
			return r.rewriteDivision(kind, left, right, out)
		}
	}
	if args.Count() != 1 {
		return RewriteFailed, nil
	}
	arg, err := at(0)
	if err != nil {
		return RewriteFailed, err
	}
	switch kind {
	case arithOpUMinus:
		return r.rewriteUnaryMinus(arg, out)
	case arithOpToInt:
		return r.rewriteToInt(arg, out)
	case arithOpToReal:
		return r.rewriteToReal(arg, out)
	case arithOpIsInt:
		return r.rewriteIsInt(arg, out)
	case arithOpAbs:
		return r.rewriteAbs(arg, out)
	case arithOpSin:
		return r.rewriteSin(arg, out)
	case arithOpCos:
		return r.rewriteCos(arg, out)
	case arithOpTan:
		return r.rewriteTan(arg, out)
	case arithOpASin, arithOpACos, arithOpATan:
		return r.rewriteInverseTrig(kind, arg, out)
	case arithOpSinh:
		return r.rewriteHyperbolic(kind, arithOpASinh, arg, out)
	case arithOpCosh:
		return r.rewriteHyperbolic(kind, arithOpACosh, arg, out)
	case arithOpTanh:
		return r.rewriteHyperbolic(kind, arithOpATanh, arg, out)
	}
	return RewriteFailed, nil
}
