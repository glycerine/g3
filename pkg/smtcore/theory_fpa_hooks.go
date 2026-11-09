package smtcore

// Source: src/smt/theory_fpa.cpp wrap, unwrap, convert_atom, convert_term,
// convert_conversion_term, convert, mk_side_conditions, internalize_atom,
// internalize_term, new_eq_eh, new_diseq_eh, assign_eh and relevant_eh.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// smtFPAFrame holds a source function's expr_ref/app_ref locals; release
// drops them in reverse order of construction.
type smtFPAFrame struct {
	m    *ast.ASTManager
	refs []ast.ASTRef
}

type smtFPARewriter interface {
	Rewrite(ast.ASTHandle, ast.ASTRef) error
}

func (t *SMTFloatingPointTheory) frame() (*smtFPAFrame, error) {
	state, err := t.State()
	if err != nil {
		return nil, err
	}
	return &smtFPAFrame{m: state.manager}, nil
}

func (f *smtFPAFrame) hold(h ast.ASTHandle) (ast.ASTHandle, error) {
	r, err := f.m.NewRef(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	f.refs = append(f.refs, r)
	return h, nil
}

// rewrite is rw(e, out) into a fresh local of the frame.
func (f *smtFPAFrame) rewrite(rw smtFPARewriter, e ast.ASTHandle) (ast.ASTHandle, error) {
	if rw == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	r, err := f.m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	f.refs = append(f.refs, r)
	if err = rw.Rewrite(e, r); err != nil {
		return ast.ASTHandle{}, err
	}
	return f.m.RefGet(r)
}

// local is an expr_ref local of the frame initialised to h.
func (f *smtFPAFrame) local(h ast.ASTHandle) (ast.ASTRef, error) {
	r, err := f.m.NewRef(h)
	if err != nil {
		return ast.ASTRef{}, err
	}
	f.refs = append(f.refs, r)
	return r, nil
}

// rewriteInPlace is rw(r) on an expr_ref: the value it replaces is released.
func (f *smtFPAFrame) rewriteInPlace(rw smtFPARewriter, r ast.ASTRef) (ast.ASTHandle, error) {
	if rw == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	h, err := f.m.RefGet(r)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = rw.Rewrite(h, r); err != nil {
		return ast.ASTHandle{}, err
	}
	return f.m.RefGet(r)
}

func (f *smtFPAFrame) release(result *error) {
	if f == nil {
		return
	}
	for i := len(f.refs) - 1; i >= 0; i-- {
		*result = errors.Join(*result, f.m.DelRef(&f.refs[i]))
	}
	f.refs = nil
}

// wrap is theory_fpa::wrap; the result lives in f.
func (t *SMTFloatingPointTheory) wrap(f *smtFPAFrame, expression ast.ASTHandle) (ast.ASTHandle, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := state.manager.GetSort(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	_, _, floating, err := state.manager.FloatingPointSortShape(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rounding, err := state.manager.IsRoundingModeSort(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssertBool(234, "m_fpa_util.is_float(e) || m_fpa_util.is_rm(e)", floating || rounding); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssert(235, "!m_fpa_util.is_bvwrap(e)", func() (bool, error) {
		op, own, e := t.operation(expression)
		return !own || op != ast.FPAOpBVWrap, e
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	if !floating && !rounding {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if op, own, operationErr := t.operation(expression); operationErr != nil {
		return ast.ASTHandle{}, operationErr
	} else if own && op == ast.FPAOpFP {
		arguments := make([]ast.ASTHandle, 3)
		for i := range arguments {
			arguments[i], err = state.manager.GetArg(expression, uint32(i))
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		tmp, concatErr := state.manager.MkBitVectorConcat(arguments)
		if concatErr != nil {
			return ast.ASTHandle{}, concatErr
		}
		r, localErr := f.local(tmp)
		if localErr != nil {
			return ast.ASTHandle{}, localErr
		}
		return f.rewriteInPlace(t.thRewriter, r)
	}
	if !rounding {
		if err = fpaSourceAssertBool(252, "m_converter.is_float(es)", floating); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	res, err := state.manager.MkFamilyApp(t.FamilyID(), ast.FPAOpBVWrap, nil, []ast.ASTHandle{expression})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return f.hold(res)
}

// unwrap is theory_fpa::unwrap; GCC builds nested call arguments right to
// left.
func (t *SMTFloatingPointTheory) unwrap(expression, sort ast.ASTHandle) (ast.ASTHandle, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	m := state.manager
	if err = fpaSourceAssert(267, "!m_fpa_util.is_fp(e)", func() (bool, error) {
		op, own, e := t.operation(expression)
		return !own || op != ast.FPAOpFP, e
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	width, widthErr := m.BitVectorSize(expression)
	if err = fpaSourceAssertBool(268, "m_bv_util.is_bv(e)", widthErr == nil); err != nil {
		return ast.ASTHandle{}, err
	}
	if widthErr != nil {
		return ast.ASTHandle{}, widthErr
	}
	ebits, sbits, floating, err := m.FloatingPointSortShape(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rounding, err := m.IsRoundingModeSort(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssertBool(269, "m_fpa_util.is_float(s) || m_fpa_util.is_rm(s)", floating || rounding); err != nil {
		return ast.ASTHandle{}, err
	}
	if rounding {
		if err = fpaSourceAssertBool(277, "bv_sz == 3", width == 3); err != nil {
			return ast.ASTHandle{}, err
		}
		rm := func(kind int32) (ast.ASTHandle, error) {
			return m.MkFamilyApp(t.FamilyID(), kind, nil, nil)
		}
		isCode := func(code uint64) (ast.ASTHandle, error) {
			n, e := m.BitVectorNumeralUint64(code, 3)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			return m.MkEq(expression, n)
		}
		// ite(e = AWAY, RNA, ite(e = EVEN, RNE, ite(e = NEG, RTN,
		// ite(e = POS, RTP, RTZ)))), innermost else first.
		res, e := rm(ast.FPAOpRMTowardZero)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		for _, level := range []struct {
			code uint64
			kind int32
		}{
			{bvRMToPositive, ast.FPAOpRMTowardPositive},
			{bvRMToNegative, ast.FPAOpRMTowardNegative},
			{bvRMTiesToEven, ast.FPAOpRMNearestEven},
			{bvRMTiesToAway, ast.FPAOpRMNearestAway},
		} {
			then, e := rm(level.kind)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			cond, e := isCode(level.code)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if res, e = m.MkIte(cond, then, res); e != nil {
				return ast.ASTHandle{}, e
			}
		}
		return res, nil
	}
	if err = fpaSourceAssertBool(284, "m_fpa_util.is_float(s)", floating); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssertBool(286, "bv_sz == m_fpa_util.get_ebits(s) + sbits", width == ebits+sbits); err != nil {
		return ast.ASTHandle{}, err
	}
	if width != ebits+sbits {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	significand, err := m.MkBitVectorExtract(sbits-2, 0, expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	exponent, err := m.MkBitVectorExtract(width-2, sbits-1, expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sign, err := m.MkBitVectorExtract(width-1, width-1, expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.MkFamilyApp(t.FamilyID(), ast.FPAOpFP, nil, []ast.ASTHandle{sign, exponent, significand})
}

// BV_RM_VAL (src/ast/fpa_decl_plugin.h).
const (
	bvRMTiesToEven = 0
	bvRMTiesToAway = 1
	bvRMToPositive = 2
	bvRMToNegative = 3
	bvRMToZero     = 4
)

// convertAtom is theory_fpa::convert_atom.
func (t *SMTFloatingPointTheory) convertAtom(f *smtFPAFrame, expression ast.ASTHandle) (ast.ASTHandle, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = t.traceFPAConvertAtom(expression); err != nil {
		return ast.ASTHandle{}, err
	}
	r, err := f.local(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = t.rewriter.Rewrite(expression, r); err != nil {
		return ast.ASTHandle{}, err
	}
	res, err := f.rewriteInPlace(t.thRewriter, r)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssert(302, "is_app(res)", func() (bool, error) {
		kind, e := state.manager.GetKind(res)
		return kind == ast.ASTApp, e
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssert(303, "m.is_bool(res)", func() (bool, error) {
		resultSort, e := state.manager.GetSort(res)
		if e != nil {
			return false, e
		}
		booleanSort, e := state.manager.MkBoolSort()
		return resultSort == booleanSort, e
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	return res, nil
}

// convertTerm is theory_fpa::convert_term.
func (t *SMTFloatingPointTheory) convertTerm(f *smtFPAFrame, expression ast.ASTHandle, rounding bool) (ast.ASTHandle, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	m := state.manager
	if err = fpaSourceAssertBool(308, "m_fpa_util.is_rm(e) || m_fpa_util.is_float(e)", true); err != nil {
		return ast.ASTHandle{}, err
	}
	eConv, err := f.rewrite(t.rewriter, expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = t.traceFPAConvertTerm(expression, eConv); err != nil {
		return ast.ASTHandle{}, err
	}
	op, own, err := t.operation(eConv)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if rounding {
		if err = fpaSourceAssertBool(320, "m_fpa_util.is_bv2rm(e_conv)", own && op == ast.FPAOpBV2RM); err != nil {
			return ast.ASTHandle{}, err
		}
		argument, argErr := m.GetArg(eConv, 0)
		if argErr != nil {
			return ast.ASTHandle{}, argErr
		}
		bvRM, rwErr := f.rewrite(t.thRewriter, argument)
		if rwErr != nil {
			return ast.ASTHandle{}, rwErr
		}
		res, mkErr := m.MkFamilyApp(t.FamilyID(), ast.FPAOpBV2RM, nil, []ast.ASTHandle{bvRM})
		if mkErr != nil {
			return ast.ASTHandle{}, mkErr
		}
		return f.hold(res)
	}
	if err = fpaSourceAssertBool(326, "m_fpa_util.is_fp(e_conv)", own && op == ast.FPAOpFP); err != nil {
		return ast.ASTHandle{}, err
	}
	// split_fp(e_conv, sgn, exp, sig), then m_th_rw on each in turn.
	parts := make([]ast.ASTHandle, 3)
	locals := make([]ast.ASTRef, 3)
	for i := range parts {
		if parts[i], err = m.GetArg(eConv, uint32(i)); err != nil {
			return ast.ASTHandle{}, err
		}
		if locals[i], err = f.local(parts[i]); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	for i := range parts {
		if parts[i], err = f.rewriteInPlace(t.thRewriter, locals[i]); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	res, err := m.MkFamilyApp(t.FamilyID(), ast.FPAOpFP, nil, parts)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return f.hold(res)
}

// convertConversionTerm is theory_fpa::convert_conversion_term.
func (t *SMTFloatingPointTheory) convertConversionTerm(f *smtFPAFrame, expression ast.ASTHandle) (ast.ASTHandle, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssert(341, "to_app(e)->get_family_id() == get_family_id()", func() (bool, error) {
		family, e := state.manager.GetFamilyID(expression)
		return family == t.FamilyID(), e
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	r, err := f.local(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = t.rewriter.Rewrite(expression, r); err != nil {
		return ast.ASTHandle{}, err
	}
	return f.rewriteInPlace(t.thRewriter, r)
}

// convert is theory_fpa::convert; the m_conversions entry owns the result.
func (t *SMTFloatingPointTheory) convert(expression ast.ASTHandle) (out ast.ASTHandle, result error) {
	if err := t.traceFPAExpression("t_fpa", "convert", 354, "converting ", expression); err != nil {
		return ast.ASTHandle{}, err
	}
	if value, found := t.converted[expression]; found {
		if err := t.traceFPAConversion(358, "cached:", expression, value); err != nil {
			return ast.ASTHandle{}, err
		}
		return value, nil
	}
	state, err := t.State()
	if err != nil || t.rewriter == nil || t.thRewriter == nil {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	m := state.manager
	f := &smtFPAFrame{m: m}
	defer f.release(&result)
	sort, err := m.GetSort(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	booleanSort, err := m.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	_, _, floating, err := m.FloatingPointSortShape(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rounding, err := m.IsRoundingModeSort(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	op, own, err := t.operation(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var res ast.ASTHandle
	switch {
	case own && op == ast.FPAOpFP:
		res = expression
	case sort == booleanSort:
		res, err = t.convertAtom(f, expression)
	case floating || rounding:
		res, err = t.convertTerm(f, expression, rounding)
	default:
		res, err = t.convertConversionTerm(f, expression)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = t.traceFPAConversion(372, "converted; caching:", expression, res); err != nil {
		return ast.ASTHandle{}, err
	}
	key, err := m.NewRef(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	value, err := m.NewRef(res)
	if err != nil {
		return ast.ASTHandle{}, errors.Join(err, m.DelRef(&key))
	}
	t.converted[expression] = res
	t.conversion = append(t.conversion, smtFPAConversion{key: key, value: value})
	return res, nil
}

// mkSideConditions is theory_fpa::mk_side_conditions; the result lives in f.
func (t *SMTFloatingPointTheory) mkSideConditions(f *smtFPAFrame) (out ast.ASTHandle, result error) {
	if t.converter == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	m := state.manager
	if state.theoryRewriter == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	res, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	f.refs = append(f.refs, res)
	tr, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { result = errors.Join(result, m.DelRef(&tr)) }()
	trueValue, err := m.MkTrue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = m.SetRef(res, trueValue); err != nil {
		return ast.ASTHandle{}, err
	}
	assertions, err := t.converter.FlushExtraAssertions()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for _, assertion := range assertions {
		if err != nil {
			break
		}
		if err = state.theoryRewriter.Rewrite(assertion, tr); err != nil {
			break
		}
		var current, rewritten, conj ast.ASTHandle
		if current, err = m.RefGet(res); err != nil {
			break
		}
		if rewritten, err = m.RefGet(tr); err != nil {
			break
		}
		if conj, err = m.MkAnd(current, rewritten); err != nil {
			break
		}
		err = m.SetRef(res, conj)
	}
	// m_extra_assertions.reset()
	for _, assertion := range assertions {
		err = errors.Join(err, m.DecRef(assertion))
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if out, err = f.rewriteInPlace(t.thRewriter, res); err != nil {
		return ast.ASTHandle{}, err
	}
	if !m.IsTrue(out) {
		if err = t.traceFPASideCondition(out); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return out, nil
}

// assertSideConditions is assert_cnstr(mk_side_conditions()).
func (t *SMTFloatingPointTheory) assertSideConditions() (result error) {
	f, err := t.frame()
	if err != nil {
		return err
	}
	defer f.release(&result)
	c, err := t.mkSideConditions(f)
	if err != nil {
		return err
	}
	return t.assertConstraint(c)
}

// InternalizeAtom is theory_fpa::internalize_atom.
func (t *SMTFloatingPointTheory) InternalizeAtom(expression ast.ASTHandle, _ bool) (handled bool, result error) {
	if err := t.traceFPAExpression("t_fpa_internalize", "internalize_atom", 426, "internalizing atom: ", expression); err != nil {
		return false, err
	}
	_, own, err := t.operation(expression)
	if err != nil || !own {
		return false, err
	}
	state, err := t.State()
	if err != nil {
		return false, err
	}
	m := state.manager
	if err = fpaSourceAssert(427, "atom->get_family_id() == get_family_id()", func() (bool, error) {
		family, e := m.GetFamilyID(expression)
		return family == t.FamilyID(), e
	}); err != nil {
		return false, err
	}
	if _, found := state.BoolVar(expression); found {
		return true, nil
	}
	count, err := m.GetNumArgs(expression)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < count; i++ {
		argument, argumentErr := m.GetArg(expression, i)
		if argumentErr != nil {
			return false, argumentErr
		}
		if argumentErr = state.Internalize(argument, false); argumentErr != nil {
			return false, argumentErr
		}
	}
	variable, err := state.MkBoolVar(expression)
	if err != nil {
		return false, err
	}
	if err = state.SetVariableTheory(variable, t.FamilyID()); err != nil {
		return false, err
	}
	f := &smtFPAFrame{m: m}
	defer f.release(&result)
	bvAtom, err := t.convertAtom(f, expression)
	if err != nil {
		return false, err
	}
	side, err := t.mkSideConditions(f)
	if err != nil {
		return false, err
	}
	withSide, err := m.MkAnd(bvAtom, side)
	if err != nil {
		return false, err
	}
	withSideRef, err := f.local(withSide)
	if err != nil {
		return false, err
	}
	if withSide, err = f.rewriteInPlace(t.thRewriter, withSideRef); err != nil {
		return false, err
	}
	atomEq, err := m.MkEq(expression, withSide)
	if err != nil {
		return false, err
	}
	if atomEq, err = f.hold(atomEq); err != nil {
		return false, err
	}
	return true, t.assertConstraint(atomEq)
}

// InternalizeEquality: theory_fpa keeps theory::internalize_eq_eh's empty
// body; equalities reach it through new_eq_eh and new_diseq_eh.
func (t *SMTFloatingPointTheory) InternalizeEquality(ast.ASTHandle, int32) error {
	return nil
}

// newEquality is the shared body of new_eq_eh (negated=false) and
// new_diseq_eh (negated=true).
func (t *SMTFloatingPointTheory) newEquality(left, right SMTTheoryVar, negated bool) (result error) {
	state, err := t.State()
	if err != nil {
		return err
	}
	m := state.manager
	leftNode, err := t.Enode(left)
	if err != nil {
		return err
	}
	rightNode, err := t.Enode(right)
	if err != nil {
		return err
	}
	xe, ye := leftNode.Owner(), rightNode.Owner()
	for _, e := range []ast.ASTHandle{xe, ye} {
		if op, own, opErr := t.operation(e); opErr != nil {
			return opErr
		} else if own && op == ast.FPAOpBVWrap {
			return nil
		}
	}
	f := &smtFPAFrame{m: m}
	defer f.release(&result)
	xc, err := t.convert(xe)
	if err != nil {
		return err
	}
	yc, err := t.convert(ye)
	if err != nil {
		return err
	}
	if !negated {
		if err = t.traceFPAConvertedEquality(xc, yc); err != nil {
			return err
		}
	}
	class := func(e ast.ASTHandle) (bool, bool, error) {
		sort, e2 := m.GetSort(e)
		if e2 != nil {
			return false, false, e2
		}
		_, _, floating, e2 := m.FloatingPointSortShape(sort)
		if e2 != nil {
			return false, false, e2
		}
		rounding, e2 := m.IsRoundingModeSort(sort)
		return floating, rounding, e2
	}
	xf, xr, err := class(xe)
	if err != nil {
		return err
	}
	yf, yr, err := class(ye)
	if err != nil {
		return err
	}
	cRef, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	f.refs = append(f.refs, cRef)
	var c ast.ASTHandle
	if (xf && yf) || (xr && yr) {
		if err = t.converter.MkEq(xc, yc, cRef); err != nil {
			return err
		}
		if c, err = m.RefGet(cRef); err != nil {
			return err
		}
		if negated {
			if c, err = m.MkNot(c); err != nil {
				return err
			}
			if err = m.SetRef(cRef, c); err != nil {
				return err
			}
		}
	} else {
		eq, eqErr := m.MkEq(xc, yc)
		if eqErr != nil {
			return eqErr
		}
		if negated {
			if eq, eqErr = f.hold(eq); eqErr != nil {
				return eqErr
			}
			if eq, eqErr = m.MkNot(eq); eqErr != nil {
				return eqErr
			}
		}
		if err = m.SetRef(cRef, eq); err != nil {
			return err
		}
	}
	if c, err = f.rewriteInPlace(t.thRewriter, cRef); err != nil {
		return err
	}
	xeEqYe, err := m.MkEq(xe, ye)
	if err != nil {
		return err
	}
	if xeEqYe, err = f.hold(xeEqYe); err != nil {
		return err
	}
	if negated {
		if xeEqYe, err = m.MkNot(xeEqYe); err != nil {
			return err
		}
		if xeEqYe, err = f.hold(xeEqYe); err != nil {
			return err
		}
	}
	cEqIff, err := m.MkIff(xeEqYe, c)
	if err != nil {
		return err
	}
	if cEqIff, err = f.hold(cEqIff); err != nil {
		return err
	}
	if err = t.assertConstraint(cEqIff); err != nil {
		return err
	}
	return t.assertSideConditions()
}

// NewEquality is theory_fpa::new_eq_eh.
func (t *SMTFloatingPointTheory) NewEquality(left, right SMTTheoryVar) error {
	if err := t.traceFPAEqualityCallback("new_eq_eh", 528, "=", left, right); err != nil {
		return err
	}
	return t.newEquality(left, right, false)
}

// NewDisequality is theory_fpa::new_diseq_eh.
func (t *SMTFloatingPointTheory) NewDisequality(left, right SMTTheoryVar) error {
	if err := t.traceFPAEqualityCallback("new_diseq_eh", 572, "!=", left, right); err != nil {
		return err
	}
	return t.newEquality(left, right, true)
}

// Assign is theory_fpa::assign_eh. GCC evaluates mk_and's arguments right
// to left, so mk_side_conditions runs before convert(e).
func (t *SMTFloatingPointTheory) Assign(variable int32, isTrue bool) (result error) {
	if err := t.traceFPAAssignment(variable, isTrue); err != nil {
		return err
	}
	state, err := t.State()
	if err != nil {
		return err
	}
	m := state.manager
	e, err := state.BoolExpression(variable)
	if err != nil {
		return err
	}
	f := &smtFPAFrame{m: m}
	defer f.release(&result)
	side, err := t.mkSideConditions(f)
	if err != nil {
		return err
	}
	converted, err := t.convert(e)
	if err != nil {
		return err
	}
	conj, err := m.MkAnd(converted, side)
	if err != nil {
		return err
	}
	if conj, err = f.hold(conj); err != nil {
		return err
	}
	var cnstr ast.ASTHandle
	if isTrue {
		cnstr, err = m.MkImplies(e, conj)
	} else {
		cnstr, err = m.MkImplies(conj, e)
	}
	if err != nil {
		return err
	}
	cnstrRef, err := f.local(cnstr)
	if err != nil {
		return err
	}
	if cnstr, err = f.rewriteInPlace(t.thRewriter, cnstrRef); err != nil {
		return err
	}
	return t.assertConstraint(cnstr)
}

// Relevant is theory_fpa::relevant_eh.
func (t *SMTFloatingPointTheory) Relevant(expression ast.ASTHandle) (result error) {
	state, err := t.State()
	if err != nil {
		return err
	}
	m := state.manager
	if err = t.traceFPAExpression("t_fpa", "relevant_eh", 647, "relevant_eh for: ", expression); err != nil {
		return err
	}
	sort, err := m.GetSort(expression)
	if err != nil {
		return err
	}
	_, _, floating, err := m.FloatingPointSortShape(sort)
	if err != nil {
		return err
	}
	rounding, err := m.IsRoundingModeSort(sort)
	if err != nil {
		return err
	}
	op, own, err := t.operation(expression)
	if err != nil {
		return err
	}
	if !floating && !rounding {
		if own {
			return fpaSourceAssertBool(686, "!m_fpa_util.is_float(n) && !m_fpa_util.is_rm(n)", true)
		}
		return fpaSourceAssert(695, "m_bv_util.is_bv(n)", func() (bool, error) {
			_, e := m.BitVectorSize(expression)
			return e == nil, nil
		})
	}
	if own && op == ast.FPAOpFP {
		return nil
	}
	f := &smtFPAFrame{m: m}
	defer f.release(&result)
	wrapped, err := t.wrap(f, expression)
	if err != nil {
		return err
	}
	if rm, isRM := smtFPARoundingNumeral(own, op); isRM {
		rmNum, numErr := m.BitVectorNumeralUint64(rm, 3)
		if numErr != nil {
			return numErr
		}
		if rmNum, numErr = f.hold(rmNum); numErr != nil {
			return numErr
		}
		c, eqErr := m.MkEq(wrapped, rmNum)
		if eqErr != nil {
			return eqErr
		}
		if c, eqErr = f.hold(c); eqErr != nil {
			return eqErr
		}
		return t.assertConstraint(c)
	}
	if numeral, valueErr := m.IsValue(expression); valueErr != nil {
		return valueErr
	} else if numeral {
		bvValE, conversionErr := t.convert(expression)
		if conversionErr != nil {
			return conversionErr
		}
		if conversionErr = fpaSourceAssert(666, "is_app(bv_val_e)", func() (bool, error) {
			kind, e := m.GetKind(bvValE)
			return kind == ast.ASTApp, e
		}); conversionErr != nil {
			return conversionErr
		}
		if conversionErr = fpaSourceAssert(667, "to_app(bv_val_e)->get_num_args() == 3", func() (bool, error) {
			count, e := m.GetNumArgs(bvValE)
			return count == 3, e
		}); conversionErr != nil {
			return conversionErr
		}
		args := make([]ast.ASTHandle, 3)
		for i := range args {
			if args[i], err = m.GetArg(bvValE, uint32(i)); err != nil {
				return err
			}
		}
		ccArgs, concatErr := m.MkBitVectorConcat(args)
		if concatErr != nil {
			return concatErr
		}
		if ccArgs, err = f.hold(ccArgs); err != nil {
			return err
		}
		c, eqErr := m.MkEq(wrapped, ccArgs)
		if eqErr != nil {
			return eqErr
		}
		if c, err = f.hold(c); err != nil {
			return err
		}
		if err = t.assertConstraint(c); err != nil {
			return err
		}
		return t.assertSideConditions()
	}
	unwrapped, err := t.unwrap(wrapped, sort)
	if err != nil {
		return err
	}
	wu, err := m.MkEq(unwrapped, expression)
	if err != nil {
		return err
	}
	if wu, err = f.hold(wu); err != nil {
		return err
	}
	if err = t.traceFPAWrapUnwrap(wu); err != nil {
		return err
	}
	return t.assertConstraint(wu)
}

// smtFPARoundingNumeral is fpa_util::is_rm_numeral(n, rm) read as BV_RM_VAL.
func smtFPARoundingNumeral(own bool, op int32) (uint64, bool) {
	if !own {
		return 0, false
	}
	switch op {
	case ast.FPAOpRMNearestAway:
		return bvRMTiesToAway, true
	case ast.FPAOpRMNearestEven:
		return bvRMTiesToEven, true
	case ast.FPAOpRMTowardNegative:
		return bvRMToNegative, true
	case ast.FPAOpRMTowardPositive:
		return bvRMToPositive, true
	case ast.FPAOpRMTowardZero:
		return bvRMToZero, true
	}
	return 0, false
}
