package qe

// Source: src/qe/nlarith_util.cpp nlarith::util::imp: create_branches,
// get_polys, get_decomposition, the poly operations, create_branch,
// create_branch_l, inf_branch, mk_subst, mk_def, to_expr and the isubst
// family (sqrt_subst, plus_eps_subst, minus_inf_subst). Every node is built
// through the arith_rewriter m_rw and the bool_rewriter m_bs, as in the
// C++, and is kept in m_trail until the next create_branches.
//
// GCC evaluates call arguments right to left; nested constructor calls are
// therefore made innermost-last-argument first, so the hash-consed nodes
// are created in the C++ order.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// nlComp is nlarith's comp.
type nlComp int

const (
	nlLE nlComp = iota
	nlLT
	nlEQ
	nlNE
)

// nlPoly is nlarith's poly (an app_ref_vector of coefficients in increasing
// degree); its entries are kept alive by the trail.
type nlPoly []ast.ASTHandle

// nlSqrtForm is imp::sqrt_form: (a + b*sqrt(c))/d.
type nlSqrtForm struct {
	a ast.ASTHandle
	b int64
	c ast.ASTHandle
	d ast.ASTHandle
}

// nlABCPoly is imp::abc_poly.
type nlABCPoly struct{ a, b, c ast.ASTHandle }

// nlISubst is imp::isubst. The default mk_le and mk_ne of the C++ base
// class are nlSubstLE and nlSubstNE; they dispatch to the virtual mk_lt and
// mk_eq of the receiver.
type nlISubst interface {
	mkLt(p nlPoly) (ast.ASTHandle, error)
	mkEq(p nlPoly) (ast.ASTHandle, error)
	mkLe(p nlPoly) (ast.ASTHandle, error)
	mkNe(p nlPoly) (ast.ASTHandle, error)
}

// nlSubstLE is isubst::mk_le: r = mk_or(mk_lt(p), mk_eq(p)).
func (u *NonlinearArithmeticUtil) nlSubstLE(s nlISubst, p nlPoly) (ast.ASTHandle, error) {
	r1, err := s.mkLt(p)
	if err != nil {
		return r1, err
	}
	r2, err := s.mkEq(p)
	if err != nil {
		return r2, err
	}
	return u.mkOr(r1, r2)
}

// nlSubstNE is isubst::mk_ne: r = m.mk_not(mk_eq(p)).
func (u *NonlinearArithmeticUtil) nlSubstNE(s nlISubst, p nlPoly) (ast.ASTHandle, error) {
	r, err := s.mkEq(p)
	if err != nil {
		return r, err
	}
	n, err := u.manager.MkNot(r)
	if err != nil {
		return n, err
	}
	return u.track(n)
}

// ---------------------------------------------------------------------
// Trail and constructors (imp::track, mk_lt, ..., mk_or).

func (u *NonlinearArithmeticUtil) track(h ast.ASTHandle) (ast.ASTHandle, error) {
	r, err := u.manager.NewRef(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	u.trail = append(u.trail, r)
	return h, nil
}

// trackRef moves an owned result reference into the trail.
func (u *NonlinearArithmeticUtil) trackRef(r ast.ASTRef, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	h, err := u.manager.RefGet(r)
	if err != nil {
		return h, errors.Join(err, u.manager.DelRef(&r))
	}
	u.trail = append(u.trail, r)
	return h, nil
}

func (u *NonlinearArithmeticUtil) resetTrail() (result error) {
	for i := len(u.trail) - 1; i >= 0; i-- {
		result = errors.Join(result, u.manager.DelRef(&u.trail[i]))
	}
	u.trail = nil
	return result
}

func (u *NonlinearArithmeticUtil) z() (ast.ASTHandle, error)   { return u.manager.RefGet(u.zero) }
func (u *NonlinearArithmeticUtil) one() (ast.ASTHandle, error) { return u.manager.RefGet(u.unit) }

// num is A.mk_numeral(v, false): a real numeral of a rational of the
// manager's NumeralService().Manager.
func (u *NonlinearArithmeticUtil) num(v *unumeral.Rational) (ast.ASTHandle, error) {
	h, err := u.manager.ArithmeticNumeralFromRational(v, false)
	if err != nil {
		return h, err
	}
	return u.track(h)
}

// numFraction is num(rational(n, d)).
func (u *NonlinearArithmeticUtil) numFraction(n int64, d uint64) (_ ast.ASTHandle, resultErr error) {
	q := u.term.q()
	v := unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&v)) }()
	if err := q.SetFraction(&v, n, d); err != nil {
		return ast.ASTHandle{}, err
	}
	return u.num(&v)
}

// numInt is imp::num(i).
func (u *NonlinearArithmeticUtil) numInt(i int64) (ast.ASTHandle, error) {
	return u.numFraction(i, 1)
}

func (u *NonlinearArithmeticUtil) mkLt(p ast.ASTHandle) (ast.ASTHandle, error) {
	zero, err := u.z()
	if err != nil {
		return zero, err
	}
	return u.trackRef(u.arith.MkLt2(p, zero))
}

func (u *NonlinearArithmeticUtil) mkLe(p ast.ASTHandle) (ast.ASTHandle, error) {
	zero, err := u.z()
	if err != nil {
		return zero, err
	}
	return u.trackRef(u.arith.MkLe2(p, zero))
}

func (u *NonlinearArithmeticUtil) mkGt(p ast.ASTHandle) (ast.ASTHandle, error) {
	n, err := u.mkUminus(p)
	if err != nil {
		return n, err
	}
	return u.mkLt(n)
}

func (u *NonlinearArithmeticUtil) mkGe(p ast.ASTHandle) (ast.ASTHandle, error) {
	n, err := u.mkUminus(p)
	if err != nil {
		return n, err
	}
	return u.mkLe(n)
}

// boolOut runs a bool_rewriter constructor into a fresh reference and
// tracks it.
func (u *NonlinearArithmeticUtil) boolOut(f func(out ast.ASTRef) error) (ast.ASTHandle, error) {
	out, err := u.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = f(out); err != nil {
		return ast.ASTHandle{}, errors.Join(err, u.manager.DelRef(&out))
	}
	return u.trackRef(out, nil)
}

func (u *NonlinearArithmeticUtil) mkEq(p ast.ASTHandle) (ast.ASTHandle, error) {
	zero, err := u.z()
	if err != nil {
		return zero, err
	}
	return u.boolOut(func(out ast.ASTRef) error { return u.bs.MkEq(p, zero, out) })
}

func (u *NonlinearArithmeticUtil) mkNe(p ast.ASTHandle) (ast.ASTHandle, error) {
	zero, err := u.z()
	if err != nil {
		return zero, err
	}
	return u.boolOut(func(out ast.ASTRef) error {
		if e := u.bs.MkEq(p, zero, out); e != nil {
			return e
		}
		eq, e := u.manager.RefGet(out)
		if e != nil {
			return e
		}
		// m_bs.mk_not(r, r): r stays alive in out while mk_not reads it.
		held, e := u.manager.NewRef(eq)
		if e != nil {
			return e
		}
		e = u.bs.MkNot(eq, out)
		return errors.Join(e, u.manager.DelRef(&held))
	})
}

func (u *NonlinearArithmeticUtil) mkUminus(e ast.ASTHandle) (ast.ASTHandle, error) {
	return u.trackRef(u.arith.MkUMinus1(e))
}

func (u *NonlinearArithmeticUtil) mkAddN(args []ast.ASTHandle) (ast.ASTHandle, error) {
	return u.trackRef(u.arith.MkAddN(args))
}

func (u *NonlinearArithmeticUtil) mkAdd(t, s ast.ASTHandle) (ast.ASTHandle, error) {
	return u.mkAddN([]ast.ASTHandle{t, s})
}

// mkAdd3 is mk_add(t, s, u) = mk_add(t, mk_add(s, u)).
func (u *NonlinearArithmeticUtil) mkAdd3(t, s, v ast.ASTHandle) (ast.ASTHandle, error) {
	inner, err := u.mkAdd(s, v)
	if err != nil {
		return inner, err
	}
	return u.mkAdd(t, inner)
}

func (u *NonlinearArithmeticUtil) mkMul(t, s ast.ASTHandle) (ast.ASTHandle, error) {
	return u.trackRef(u.arith.MkMul2(t, s))
}

// mkMul3 is mk_mul(t, s, u) = mk_mul(t, mk_mul(s, u)).
func (u *NonlinearArithmeticUtil) mkMul3(t, s, v ast.ASTHandle) (ast.ASTHandle, error) {
	inner, err := u.mkMul(s, v)
	if err != nil {
		return inner, err
	}
	return u.mkMul(t, inner)
}

func (u *NonlinearArithmeticUtil) mkSub(t, s ast.ASTHandle) (ast.ASTHandle, error) {
	return u.trackRef(u.arith.MkSub2(t, s))
}

func (u *NonlinearArithmeticUtil) mkAndN(args []ast.ASTHandle) (ast.ASTHandle, error) {
	return u.boolOut(func(out ast.ASTRef) error { return u.bs.MkAnd(args, out) })
}

func (u *NonlinearArithmeticUtil) mkAnd(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return u.mkAndN([]ast.ASTHandle{a, b})
}

func (u *NonlinearArithmeticUtil) mkOrN(args []ast.ASTHandle) (ast.ASTHandle, error) {
	return u.boolOut(func(out ast.ASTRef) error { return u.bs.MkOr(args, out) })
}

func (u *NonlinearArithmeticUtil) mkOr(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return u.mkOrN([]ast.ASTHandle{a, b})
}

// rawApp is a plain arith_util constructor (mk_div, mk_power).
func (u *NonlinearArithmeticUtil) rawApp(kind int32, args ...ast.ASTHandle) (ast.ASTHandle, error) {
	h, err := u.manager.MkFamilyApp(u.family, kind, nil, args)
	if err != nil {
		return h, err
	}
	return u.track(h)
}

func (u *NonlinearArithmeticUtil) rawImplies(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	h, err := u.manager.MkImplies(a, b)
	if err != nil {
		return h, err
	}
	return u.track(h)
}

// ---------------------------------------------------------------------
// create_branches

// CreateBranches is imp::create_branches(x, num_lits, lits, branch_conds).
func (u *NonlinearArithmeticUtil) CreateBranches(variable *ast.ContainsApp, literals []ast.ASTHandle, branches *NLBranchConditions) (bool, error) {
	if err := u.open(); err != nil {
		return false, err
	}
	if branches == nil || variable == nil || u.arith == nil || u.bs == nil {
		return false, ast.ErrASTHandle
	}
	if err := branches.Reset(); err != nil {
		return false, err
	}
	// m_trail.reset(); // use scope?
	if err := u.resetTrail(); err != nil {
		return false, err
	}
	target, err := variable.Target()
	if err != nil {
		return false, err
	}
	// if (!a().is_real(x)) return false;
	sort, err := u.manager.GetSort(target)
	if err != nil {
		return false, err
	}
	family, err := u.manager.GetFamilyID(sort)
	if err != nil {
		return false, err
	}
	if family != u.family {
		return false, nil
	}
	kind, err := u.manager.GetDeclKind(sort)
	if err != nil || kind != 0 {
		return false, err
	}
	var polys []nlPoly
	var comps []nlComp
	ok, err := u.getPolys(variable, literals, &polys, &comps, branches)
	if err != nil || !ok {
		return false, err
	}
	if nlIsDegreeTwoPlus(polys) {
		return false, nil
	}
	if !u.enableLinear {
		linear, err := u.isLinearPolys(polys)
		if err != nil || linear {
			return false, err
		}
	}
	if idx, single := nlHasSingleDegree2(polys, comps); single {
		for i := range polys {
			if err = u.createBranchL(idx, i, polys, comps, branches); err != nil {
				return false, err
			}
		}
	} else {
		for i := range polys {
			if err = u.createBranch(i, polys, comps, branches); err != nil {
				return false, err
			}
		}
	}
	if err = u.infBranch(polys, comps, branches); err != nil {
		return false, err
	}
	return true, nil
}

func nlIsDegreeTwoPlus(ps []nlPoly) bool {
	for _, p := range ps {
		if len(p) > 3 {
			return true
		}
	}
	return false
}

// isLinearPolys is imp::is_linear: it rewrites a numeral-valued linear
// coefficient in place with a plain numeral.
func (u *NonlinearArithmeticUtil) isLinearPolys(ps []nlPoly) (bool, error) {
	for i := range ps {
		if len(ps[i]) > 2 {
			return false, nil
		}
		if len(ps[i]) == 2 {
			// rational n;
			q := u.term.q()
			n := unumeral.NewRational()
			ok, err := u.isNumeralFold(ps[i][1], &n)
			if err == nil && ok {
				ps[i][1], err = u.num(&n)
			}
			if err = errors.Join(err, q.Del(&n)); err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
	}
	return true, nil
}

// isNumeralFold is imp::is_numeral(t, n): constant folding over +, *, -,
// unary -, writing n (a rational of the manager's NumeralService().Manager).
func (u *NonlinearArithmeticUtil) isNumeralFold(t ast.ASTHandle, n *unumeral.Rational) (_ bool, resultErr error) {
	kindOf, err := u.manager.GetKind(t)
	if err != nil || kindOf != ast.ASTApp {
		return false, err
	}
	family, err := u.manager.GetFamilyID(t)
	if err != nil || family != u.family {
		return false, err
	}
	kind, err := u.manager.GetDeclKind(t)
	if err != nil {
		return false, err
	}
	q := u.term.q()
	// rational m;
	m := unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&m)) }()
	switch kind {
	case 6, 9, 7: // OP_ADD, OP_MUL, OP_SUB: MK_AOP
		count, err := u.manager.GetNumArgs(t)
		if err != nil || count == 0 {
			return false, err
		}
		first, err := u.manager.GetArg(t, 0)
		if err != nil {
			return false, err
		}
		ok, err := u.isNumeralFold(first, n)
		if err != nil || !ok {
			return false, err
		}
		for i := uint32(1); i < count; i++ {
			arg, err := u.manager.GetArg(t, i)
			if err != nil {
				return false, err
			}
			ok, err := u.isNumeralFold(arg, &m)
			if err != nil || !ok {
				return false, err
			}
			switch kind {
			case 6:
				err = q.Add(n, &m, n)
			case 9:
				err = q.Mul(n, &m, n)
			default:
				err = q.Sub(n, &m, n)
			}
			if err != nil {
				return false, err
			}
		}
		return true, nil
	case 8: // OP_UMINUS
		arg, err := u.manager.GetArg(t, 0)
		if err != nil {
			return false, err
		}
		ok, err := u.isNumeralFold(arg, n)
		if err != nil || !ok {
			return false, err
		}
		return true, q.Neg(n)
	case 0: // OP_NUM
		return u.numeral(t, n)
	}
	return false, nil
}

func nlHasSingleDegree2(ps []nlPoly, comps []nlComp) (uint32, bool) {
	n := 0
	idx := uint32(0)
	for i := range ps {
		if len(ps[i]) == 3 {
			n++
			idx = uint32(i)
			if comps[i] == nlEQ {
				return idx, false
			}
		}
	}
	return idx, n == 1
}

// arithArgs reports whether e is an application of the arithmetic family
// with the given kind, and its two arguments.
func (u *NonlinearArithmeticUtil) arithBinary(e ast.ASTHandle, kind int32) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	args, ok, err := u.term.operation(e, u.family, kind)
	if err != nil || !ok || len(args) != 2 {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	return args[0], args[1], true, nil
}

// isLE is imp::is_le: a().is_ge(e, t, s) || a().is_le(e, s, t).
func (u *NonlinearArithmeticUtil) isLE(e ast.ASTHandle) (s, t ast.ASTHandle, ok bool, err error) {
	if t, s, ok, err = u.arithBinary(e, 3); err != nil || ok {
		return s, t, ok, err
	}
	s, t, ok, err = u.arithBinary(e, 2)
	return s, t, ok, err
}

// isLT is imp::is_lt: a().is_gt(e, t, s) || a().is_lt(e, s, t).
func (u *NonlinearArithmeticUtil) isLT(e ast.ASTHandle) (s, t ast.ASTHandle, ok bool, err error) {
	if t, s, ok, err = u.arithBinary(e, 5); err != nil || ok {
		return s, t, ok, err
	}
	s, t, ok, err = u.arithBinary(e, 4)
	return s, t, ok, err
}

func (u *NonlinearArithmeticUtil) isEq(e ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	args, ok, err := u.term.operation(e, u.manager.BasicFamilyID(), ast.ASTOpEq)
	if err != nil || !ok || len(args) != 2 {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	return args[0], args[1], true, nil
}

func (u *NonlinearArithmeticUtil) isNotLit(e ast.ASTHandle) (ast.ASTHandle, bool, error) {
	ok, arg, err := u.manager.IsNot(e)
	return arg, ok, err
}

// getPolys is imp::get_polys(contains_x, num_lits, lits, polys, comps, bc,
// nullptr).
func (u *NonlinearArithmeticUtil) getPolys(contains *ast.ContainsApp, lits []ast.ASTHandle, polys *[]nlPoly, comps *[]nlComp, bc *NLBranchConditions) (bool, error) {
	for _, lit := range lits {
		has, err := contains.Check(lit)
		if err != nil {
			return false, err
		}
		if !has {
			continue
		}
		var t ast.ASTHandle
		var c nlComp
		matched := false
		if e1, e2, ok, err := u.isLE(lit); err != nil {
			return false, err
		} else if ok {
			// e1 <= e2
			if t, err = u.mkSub(e1, e2); err != nil {
				return false, err
			}
			c, matched = nlLE, true
		}
		inner, negated, err := u.isNotLit(lit)
		if err != nil {
			return false, err
		}
		if !matched && negated {
			// ! (e2 <= e3) <=> e3 < e2
			if e2, e3, ok, err := u.isLE(inner); err != nil {
				return false, err
			} else if ok {
				if t, err = u.mkSub(e3, e2); err != nil {
					return false, err
				}
				c, matched = nlLT, true
			}
		}
		if !matched {
			// e1 < e2
			if e1, e2, ok, err := u.isLT(lit); err != nil {
				return false, err
			} else if ok {
				if t, err = u.mkSub(e1, e2); err != nil {
					return false, err
				}
				c, matched = nlLT, true
			}
		}
		if !matched && negated {
			// ! (e2 < e3) <=> e3 <= e2
			if e2, e3, ok, err := u.isLT(inner); err != nil {
				return false, err
			} else if ok {
				if t, err = u.mkSub(e3, e2); err != nil {
					return false, err
				}
				c, matched = nlLE, true
			}
		}
		if !matched {
			if e1, e2, ok, err := u.isEq(lit); err != nil {
				return false, err
			} else if ok {
				if t, err = u.mkSub(e1, e2); err != nil {
					return false, err
				}
				c, matched = nlEQ, true
			}
		}
		if !matched && negated {
			if e2, e3, ok, err := u.isEq(inner); err != nil {
				return false, err
			} else if ok {
				if t, err = u.mkSub(e2, e3); err != nil {
					return false, err
				}
				c, matched = nlNE, true
			}
		}
		if !matched {
			return false, nil
		}
		p, ok, err := u.getDecomposition(t, contains)
		if err != nil || !ok {
			return false, err
		}
		*polys = append(*polys, p)
		*comps = append(*comps, c)
		if err = bc.AddPredicate(lit); err != nil {
			return false, err
		}
	}
	return true, nil
}

// getDecomposition is imp::get_decomposition: p = result[0] + x*result[1]
// + x*x*result[2] + ...
func (u *NonlinearArithmeticUtil) getDecomposition(t ast.ASTHandle, contains *ast.ContainsApp) (nlPoly, bool, error) {
	kindOf, err := u.manager.GetKind(t)
	if err != nil || kindOf != ast.ASTApp {
		return nil, false, err
	}
	has, err := contains.Check(t)
	if err != nil {
		return nil, false, err
	}
	if !has {
		return nlPoly{t}, true, nil
	}
	x, err := contains.Target()
	if err != nil {
		return nil, false, err
	}
	if x == t {
		zero, err := u.z()
		if err != nil {
			return nil, false, err
		}
		one, err := u.one()
		if err != nil {
			return nil, false, err
		}
		return nlPoly{zero, one}, true, nil
	}
	family, err := u.manager.GetFamilyID(t)
	if err != nil || family != u.family {
		return nil, false, err
	}
	kind, err := u.manager.GetDeclKind(t)
	if err != nil {
		return nil, false, err
	}
	count, err := u.manager.GetNumArgs(t)
	if err != nil {
		return nil, false, err
	}
	switch kind {
	case 6, 9, 7: // OP_ADD, OP_MUL, OP_SUB: MK_OP
		if count == 0 {
			return nil, false, ast.ErrASTDomain
		}
		first, err := u.manager.GetArg(t, 0)
		if err != nil {
			return nil, false, err
		}
		result, ok, err := u.getDecomposition(first, contains)
		if err != nil || !ok {
			return nil, false, err
		}
		for i := uint32(1); i < count; i++ {
			arg, err := u.manager.GetArg(t, i)
			if err != nil {
				return nil, false, err
			}
			r, ok, err := u.getDecomposition(arg, contains)
			if err != nil || !ok {
				return nil, false, err
			}
			switch kind {
			case 6:
				result, err = u.polyAdd(result, r)
			case 9:
				result, err = u.polyMul(result, r)
			default:
				result, err = u.polySub(result, r)
			}
			if err != nil {
				return nil, false, err
			}
		}
		return result, true, nil
	case 8: // OP_UMINUS
		first, err := u.manager.GetArg(t, 0)
		if err != nil {
			return nil, false, err
		}
		result, ok, err := u.getDecomposition(first, contains)
		if err != nil || !ok {
			return nil, false, err
		}
		return result, true, u.polyUminus(result)
	}
	return nil, false, nil
}

// polyUminus is imp::mk_uminus(poly&).
func (u *NonlinearArithmeticUtil) polyUminus(p nlPoly) (err error) {
	for i := range p {
		if p[i], err = u.mkUminus(p[i]); err != nil {
			return err
		}
	}
	return nil
}

// polySub is imp::mk_sub(poly& r, poly const& other).
func (u *NonlinearArithmeticUtil) polySub(r, other nlPoly) (nlPoly, error) {
	var err error
	for i := 0; i < len(r) && i < len(other); i++ {
		if r[i], err = u.mkSub(r[i], other[i]); err != nil {
			return r, err
		}
	}
	for i := len(r); i < len(other); i++ {
		n, err := u.mkUminus(other[i])
		if err != nil {
			return r, err
		}
		r = append(r, n)
	}
	return r, nil
}

// polyAdd is imp::mk_add(poly& r, poly const& other).
func (u *NonlinearArithmeticUtil) polyAdd(r, other nlPoly) (nlPoly, error) {
	var err error
	for i := 0; i < len(r) && i < len(other); i++ {
		if r[i], err = u.mkAdd(r[i], other[i]); err != nil {
			return r, err
		}
	}
	for i := len(r); i < len(other); i++ {
		r = append(r, other[i])
	}
	return r, nil
}

// polyMul is imp::mk_mul(poly& r, poly const& other).
func (u *NonlinearArithmeticUtil) polyMul(r, other nlPoly) (nlPoly, error) {
	var result nlPoly
	for i := 0; i+1 < len(r)+len(other); i++ {
		t, err := u.z()
		if err != nil {
			return r, err
		}
		for j := 0; j <= i && j < len(r); j++ {
			k := i - j
			if k < len(other) {
				prod, err := u.mkMul(r[j], other[k])
				if err != nil {
					return r, err
				}
				if t, err = u.mkAdd(t, prod); err != nil {
					return r, err
				}
			}
		}
		result = append(result, t)
	}
	return result, nil
}

// mkDifferentiate is imp::mk_differentiate.
func (u *NonlinearArithmeticUtil) mkDifferentiate(p nlPoly) (nlPoly, error) {
	var result nlPoly
	for i := 1; i < len(p); i++ {
		n, err := u.numInt(int64(i))
		if err != nil {
			return nil, err
		}
		t, err := u.mkMul(n, p[i])
		if err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, nil
}

// mkZero is imp::mk_zero(poly const&): the conjunction of p[i] = 0.
func (u *NonlinearArithmeticUtil) mkZero(p nlPoly) (ast.ASTHandle, error) {
	zeros := make([]ast.ASTHandle, 0, len(p))
	for i := range p {
		e, err := u.mkEq(p[i])
		if err != nil {
			return e, err
		}
		zeros = append(zeros, e)
	}
	return u.mkAndN(zeros)
}

// mkInstantiate is imp::mk_instantiate: x := (a + b*sqrt(c))/d in p gives
// (ar + br*sqrt(c))/dr.
func (u *NonlinearArithmeticUtil) mkInstantiate(p nlPoly, s *nlSqrtForm) (ar, br, dr ast.ASTHandle, err error) {
	a, c, d := s.a, s.c, s.d
	b, err := u.numInt(s.b)
	if err != nil {
		return
	}
	if br, err = u.z(); err != nil {
		return
	}
	if dr, err = u.one(); err != nil {
		return
	}
	if len(p) == 0 {
		ar, err = u.z()
		return
	}
	i := len(p) - 1
	ar = p[i]
	for i > 0 {
		i--
		// tmp1 = mk_add(mk_mul(d, dr, p[i]), mk_mul(a, ar), mk_mul(b, br, c))
		var m3, m2, m1, tmp1 ast.ASTHandle
		if m3, err = u.mkMul3(b, br, c); err != nil {
			return
		}
		if m2, err = u.mkMul(a, ar); err != nil {
			return
		}
		if m1, err = u.mkMul3(d, dr, p[i]); err != nil {
			return
		}
		if tmp1, err = u.mkAdd3(m1, m2, m3); err != nil {
			return
		}
		// br = mk_add(mk_mul(a, br), mk_mul(ar, b))
		var n2, n1 ast.ASTHandle
		if n2, err = u.mkMul(ar, b); err != nil {
			return
		}
		if n1, err = u.mkMul(a, br); err != nil {
			return
		}
		if br, err = u.mkAdd(n1, n2); err != nil {
			return
		}
		if dr, err = u.mkMul(d, dr); err != nil {
			return
		}
		ar = tmp1
	}
	return
}

func nlIsEven(n int) bool { return n&1 == 0 }

// ---------------------------------------------------------------------
// isubst family

// nlSqrtSubst is imp::sqrt_subst.
type nlSqrtSubst struct {
	u *NonlinearArithmeticUtil
	s *nlSqrtForm
}

// aabbc is I.mk_sub(I.mk_mul(a, a), I.mk_mul(b, b, c)).
func (q *nlSqrtSubst) aabbc(a, b, c ast.ASTHandle) (ast.ASTHandle, error) {
	u := q.u
	bbc, err := u.mkMul3(b, b, c)
	if err != nil {
		return bbc, err
	}
	aa, err := u.mkMul(a, a)
	if err != nil {
		return aa, err
	}
	return u.mkSub(aa, bbc)
}

func (q *nlSqrtSubst) parity(p nlPoly) (a, b, c, ad, bd ast.ASTHandle, err error) {
	u := q.u
	c = q.s.c
	var d ast.ASTHandle
	if a, b, d, err = u.mkInstantiate(p, q.s); err != nil {
		return
	}
	ad, bd = a, b
	if nlIsEven(len(p)) {
		if ad, err = u.mkMul(a, d); err != nil {
			return
		}
		if bd, err = u.mkMul(b, d); err != nil {
			return
		}
	}
	return
}

// mkLt: p[e/x] < 0: (a*parity(d) < 0 /\ 0 < a*a - b*b*c) \/
// (b*parity(d) <= 0 /\ (a*parity(d) < 0 \/ a*a - b*b*c < 0)).
func (q *nlSqrtSubst) mkLt(p nlPoly) (ast.ASTHandle, error) {
	u := q.u
	a, b, c, ad, bd, err := q.parity(p)
	if err != nil {
		return ad, err
	}
	if q.s.b == 0 {
		return u.mkLt(ad)
	}
	aabbc, err := q.aabbc(a, b, c)
	if err != nil {
		return aabbc, err
	}
	// mk_or(mk_and(mk_lt(ad), mk_gt(aabbc)),
	//       mk_and(mk_le(bd), mk_or(mk_lt(ad), mk_lt(aabbc))))
	l4, err := u.mkLt(aabbc)
	if err != nil {
		return l4, err
	}
	l3, err := u.mkLt(ad)
	if err != nil {
		return l3, err
	}
	or2, err := u.mkOr(l3, l4)
	if err != nil {
		return or2, err
	}
	le, err := u.mkLe(bd)
	if err != nil {
		return le, err
	}
	and2, err := u.mkAnd(le, or2)
	if err != nil {
		return and2, err
	}
	gt, err := u.mkGt(aabbc)
	if err != nil {
		return gt, err
	}
	lt, err := u.mkLt(ad)
	if err != nil {
		return lt, err
	}
	and1, err := u.mkAnd(lt, gt)
	if err != nil {
		return and1, err
	}
	return u.mkOr(and1, and2)
}

// mkEq: p[e/x] = 0: a*b <= 0 & a*a - b*b*c = 0.
func (q *nlSqrtSubst) mkEq(p nlPoly) (ast.ASTHandle, error) {
	u := q.u
	c := q.s.c
	a, b, _, err := u.mkInstantiate(p, q.s)
	if err != nil {
		return a, err
	}
	if q.s.b == 0 {
		return u.mkEq(a)
	}
	aabbc, err := q.aabbc(a, b, c)
	if err != nil {
		return aabbc, err
	}
	// mk_and(mk_le(mk_mul(a, b)), mk_eq(aabbc))
	eq, err := u.mkEq(aabbc)
	if err != nil {
		return eq, err
	}
	ab, err := u.mkMul(a, b)
	if err != nil {
		return ab, err
	}
	le, err := u.mkLe(ab)
	if err != nil {
		return le, err
	}
	return u.mkAnd(le, eq)
}

// mkLe: p[e/x] <= 0: a*parity(d) <= 0 /\ 0 <= a*a - b*b*c \/
// b*parity(d) <= 0 /\ a*a - b*b*c <= 0.
func (q *nlSqrtSubst) mkLe(p nlPoly) (ast.ASTHandle, error) {
	u := q.u
	a, b, c, ad, bd, err := q.parity(p)
	if err != nil {
		return ad, err
	}
	if q.s.b == 0 {
		return u.mkLe(ad)
	}
	aabbc, err := q.aabbc(a, b, c)
	if err != nil {
		return aabbc, err
	}
	// mk_or(mk_and(mk_le(ad), mk_ge(aabbc)), mk_and(mk_le(bd), mk_le(aabbc)))
	l4, err := u.mkLe(aabbc)
	if err != nil {
		return l4, err
	}
	l3, err := u.mkLe(bd)
	if err != nil {
		return l3, err
	}
	and2, err := u.mkAnd(l3, l4)
	if err != nil {
		return and2, err
	}
	ge, err := u.mkGe(aabbc)
	if err != nil {
		return ge, err
	}
	le, err := u.mkLe(ad)
	if err != nil {
		return le, err
	}
	and1, err := u.mkAnd(le, ge)
	if err != nil {
		return and1, err
	}
	return u.mkOr(and1, and2)
}

func (q *nlSqrtSubst) mkNe(p nlPoly) (ast.ASTHandle, error) { return q.u.nlSubstNE(q, p) }

// nlPlusEpsSubst is imp::plus_eps_subst.
type nlPlusEpsSubst struct {
	u *NonlinearArithmeticUtil
	s nlISubst
}

// mkNu: nu(p) = p < 0 if degree(x) = 0, else p < 0 \/ (p = 0 /\ nu(p')).
func (q *nlPlusEpsSubst) mkNu(p nlPoly) (ast.ASTHandle, error) {
	u := q.u
	r, err := q.s.mkLt(p)
	if err != nil {
		return r, err
	}
	if len(p) > 1 {
		t3, err := q.s.mkEq(p)
		if err != nil {
			return t3, err
		}
		t1, err := u.mkDifferentiate(p)
		if err != nil {
			return r, err
		}
		t4, err := q.mkNu(t1)
		if err != nil {
			return t4, err
		}
		and, err := u.mkAnd(t3, t4)
		if err != nil {
			return and, err
		}
		return u.mkOr(r, and)
	}
	return r, nil
}

func (q *nlPlusEpsSubst) mkLt(p nlPoly) (ast.ASTHandle, error) { return q.mkNu(p) }
func (q *nlPlusEpsSubst) mkEq(p nlPoly) (ast.ASTHandle, error) { return q.u.mkZero(p) }
func (q *nlPlusEpsSubst) mkLe(p nlPoly) (ast.ASTHandle, error) { return q.u.nlSubstLE(q, p) }
func (q *nlPlusEpsSubst) mkNe(p nlPoly) (ast.ASTHandle, error) { return q.u.nlSubstNE(q, p) }

// nlMinusInfSubst is imp::minus_inf_subst.
type nlMinusInfSubst struct{ u *NonlinearArithmeticUtil }

// mkLtAt is minus_inf_subst::mk_lt(p, i).
func (q *nlMinusInfSubst) mkLtAt(p nlPoly, i int) (ast.ASTHandle, error) {
	u := q.u
	if i == 0 {
		f, err := u.manager.MkFalse()
		if err != nil {
			return f, err
		}
		return u.track(f)
	}
	i--
	t := p[i]
	var e ast.ASTHandle
	var err error
	if nlIsEven(i) {
		e, err = u.mkLt(t)
	} else {
		e, err = u.mkGt(t)
	}
	if err != nil || i == 0 {
		return e, err
	}
	// mk_or(e, mk_and(mk_eq(t), mk_lt(p, i)))
	rest, err := q.mkLtAt(p, i)
	if err != nil {
		return rest, err
	}
	eq, err := u.mkEq(t)
	if err != nil {
		return eq, err
	}
	and, err := u.mkAnd(eq, rest)
	if err != nil {
		return and, err
	}
	return u.mkOr(e, and)
}

func (q *nlMinusInfSubst) mkLt(p nlPoly) (ast.ASTHandle, error) { return q.mkLtAt(p, len(p)) }
func (q *nlMinusInfSubst) mkEq(p nlPoly) (ast.ASTHandle, error) { return q.u.mkZero(p) }
func (q *nlMinusInfSubst) mkLe(p nlPoly) (ast.ASTHandle, error) { return q.u.nlSubstLE(q, p) }
func (q *nlMinusInfSubst) mkNe(p nlPoly) (ast.ASTHandle, error) { return q.u.nlSubstNE(q, p) }

// applySubst is imp::apply_subst.
func nlApplySubst(sub nlISubst, c nlComp, p nlPoly) (ast.ASTHandle, error) {
	switch c {
	case nlEQ:
		return sub.mkEq(p)
	case nlLE:
		return sub.mkLe(p)
	case nlLT:
		return sub.mkLt(p)
	default:
		return sub.mkNe(p)
	}
}

func nlIsStrict(c nlComp) bool { return c == nlLT || c == nlNE }

// mkSubst is imp::mk_subst(c1, p, c, e, r).
func (u *NonlinearArithmeticUtil) mkSubst(c1 nlComp, p nlPoly, c nlComp, e *nlSqrtForm) (ast.ASTHandle, error) {
	sub := &nlSqrtSubst{u: u, s: e}
	if nlIsStrict(c1) {
		return nlApplySubst(&nlPlusEpsSubst{u: u, s: sub}, c, p)
	}
	return nlApplySubst(sub, c, p)
}

// ---------------------------------------------------------------------
// definitions and branches

func (u *NonlinearArithmeticUtil) mkEpsilon() (ast.ASTHandle, error) {
	return u.numFraction(1, 10000)
}

func (u *NonlinearArithmeticUtil) mkInf() (ast.ASTHandle, error) {
	return u.numFraction(-10000, 1)
}

// mkAbs is imp::mk_abs: m().mk_ite(mk_lt(e), mk_uminus(e), e).
func (u *NonlinearArithmeticUtil) mkAbs(e ast.ASTHandle) (ast.ASTHandle, error) {
	neg, err := u.mkUminus(e)
	if err != nil {
		return neg, err
	}
	lt, err := u.mkLt(e)
	if err != nil {
		return lt, err
	}
	ite, err := u.manager.MkIte(lt, neg, e)
	if err != nil {
		return ite, err
	}
	return u.track(ite)
}

// toExpr is imp::to_expr: (a + b*sqrt(c))/d.
func (u *NonlinearArithmeticUtil) toExpr(s *nlSqrtForm) (ast.ASTHandle, error) {
	zero, err := u.z()
	if err != nil {
		return zero, err
	}
	if s.c == zero || s.b == 0 {
		return u.rawApp(10, s.a, s.d) // A.mk_div
	}
	half, err := u.numFraction(1, 2)
	if err != nil {
		return half, err
	}
	// A.mk_div(mk_add(s.m_a, mk_mul(num(s.m_b), A.mk_power(mk_abs(s.m_c), half))), s.m_d)
	abs, err := u.mkAbs(s.c)
	if err != nil {
		return abs, err
	}
	power, err := u.rawApp(18, abs, half)
	if err != nil {
		return power, err
	}
	b, err := u.numInt(s.b)
	if err != nil {
		return b, err
	}
	prod, err := u.mkMul(b, power)
	if err != nil {
		return prod, err
	}
	sum, err := u.mkAdd(s.a, prod)
	if err != nil {
		return sum, err
	}
	return u.rawApp(10, sum, s.d)
}

// mkDef is imp::mk_def(cmp, abc_poly, sqrt_form).
func (u *NonlinearArithmeticUtil) mkDef(cmp nlComp, p nlABCPoly, s *nlSqrtForm) (ast.ASTHandle, error) {
	result, err := u.toExpr(s)
	if err != nil || !nlIsStrict(cmp) {
		return result, err
	}
	zero, err := u.z()
	if err != nil {
		return zero, err
	}
	var extra ast.ASTHandle
	if p.a == zero {
		// mk_mul(mk_epsilon(), m().mk_ite(mk_lt(p.m_b), num(1), num(-1)))
		minusOne, err := u.numInt(-1)
		if err != nil {
			return minusOne, err
		}
		one, err := u.numInt(1)
		if err != nil {
			return one, err
		}
		lt, err := u.mkLt(p.b)
		if err != nil {
			return lt, err
		}
		ite, err := u.manager.MkIte(lt, one, minusOne)
		if err == nil {
			ite, err = u.track(ite)
		}
		if err != nil {
			return ite, err
		}
		eps, err := u.mkEpsilon()
		if err != nil {
			return eps, err
		}
		if extra, err = u.mkMul(eps, ite); err != nil {
			return extra, err
		}
	} else if s.b > 0 {
		// mk_mul(num(-1), mk_epsilon())
		eps, err := u.mkEpsilon()
		if err != nil {
			return eps, err
		}
		minusOne, err := u.numInt(-1)
		if err != nil {
			return minusOne, err
		}
		if extra, err = u.mkMul(minusOne, eps); err != nil {
			return extra, err
		}
	} else {
		if extra, err = u.mkEpsilon(); err != nil {
			return extra, err
		}
	}
	return u.mkAdd(result, extra)
}

func nlCoefficients(p nlPoly, zero ast.ASTHandle) (a, b, c ast.ASTHandle) {
	a, b, c = zero, zero, zero
	if len(p) > 0 {
		c = p[0]
	}
	if len(p) > 1 {
		b = p[1]
	}
	if len(p) > 2 {
		a = p[2]
	}
	return
}

func (u *NonlinearArithmeticUtil) pred(bc *NLBranchConditions, i int) (ast.ASTHandle, error) {
	if i >= len(bc.Predicates) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return u.manager.RefGet(bc.Predicates[i])
}

// replaceZero is the expr_replacer of create_branch(_l) with substitution
// a -> 0.
func (u *NonlinearArithmeticUtil) replaceZero(a, t1 ast.ASTHandle) (ast.ASTHandle, error) {
	zero, err := u.z()
	if err != nil {
		return zero, err
	}
	out, err := u.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = qeReplaceSubformula(u.manager, a, zero, t1, out); err != nil {
		return ast.ASTHandle{}, errors.Join(err, u.manager.DelRef(&out))
	}
	return u.trackRef(out, nil)
}

// infBranch is imp::inf_branch.
func (u *NonlinearArithmeticUtil) infBranch(polys []nlPoly, comps []nlComp, bc *NLBranchConditions) error {
	es := make([]ast.ASTHandle, 0, len(polys))
	subst := make([]ast.ASTHandle, 0, len(polys))
	for j := range polys {
		sub := &nlMinusInfSubst{u: u}
		t1, err := nlApplySubst(sub, comps[j], polys[j])
		if err != nil {
			return err
		}
		p, err := u.pred(bc, j)
		if err != nil {
			return err
		}
		imp, err := u.rawImplies(p, t1)
		if err != nil {
			return err
		}
		es = append(es, imp)
		subst = append(subst, t1)
	}
	// bc.add_branch(mk_and(es), m().mk_true(), subst, mk_inf(bc), z(), z(), z())
	zero, err := u.z()
	if err != nil {
		return err
	}
	inf, err := u.mkInf()
	if err != nil {
		return err
	}
	truth, err := u.manager.MkTrue()
	if err != nil {
		return err
	}
	branch, err := u.mkAndN(es)
	if err != nil {
		return err
	}
	return bc.AddBranch(branch, truth, subst, inf, zero, zero, zero)
}

// createBranchL is imp::create_branch_l(j, i, polys, comps, bc).
func (u *NonlinearArithmeticUtil) createBranchL(j uint32, i int, polys []nlPoly, comps []nlComp, bc *NLBranchConditions) error {
	cmp := comps[i]
	p := polys[i]
	if uint32(i) == j {
		cmp = nlLE // non-strict to avoid epsilon substitution mode.
	}
	zero, err := u.z()
	if err != nil {
		return err
	}
	a, b, c := nlCoefficients(p, zero)
	var es, subst []ast.ASTHandle
	if b != zero {
		negC, err := u.mkUminus(c)
		if err != nil {
			return err
		}
		e0 := &nlSqrtForm{a: negC, b: 0, c: zero, d: b}
		if a != zero {
			eq, err := u.mkEq(a)
			if err != nil {
				return err
			}
			es = append(es, eq)
		}
		ne, err := u.mkNe(b)
		if err != nil {
			return err
		}
		es = append(es, ne)
		cond, err := u.mkAndN(es)
		if err != nil {
			return err
		}
		pi, err := u.pred(bc, i)
		if err != nil {
			return err
		}
		es = append(es, pi)
		for k := range polys {
			t1, err := u.mkSubst(cmp, polys[k], comps[k], e0)
			if err != nil {
				return err
			}
			t2, err := u.replaceZero(a, t1)
			if err != nil {
				return err
			}
			pk, err := u.pred(bc, k)
			if err != nil {
				return err
			}
			imp, err := u.rawImplies(pk, t2)
			if err != nil {
				return err
			}
			es = append(es, imp)
			subst = append(subst, t1)
		}
		// bc.add_branch(mk_and(es), cond, subst, mk_def(cmp, abc_poly(z, b, c), e0), a, b, c)
		def, err := u.mkDef(cmp, nlABCPoly{zero, b, c}, e0)
		if err != nil {
			return err
		}
		branch, err := u.mkAndN(es)
		if err != nil {
			return err
		}
		if err = bc.AddBranch(branch, cond, subst, def, a, b, c); err != nil {
			return err
		}
	}
	if uint32(i) == j && a != zero {
		// a != 0 & phi[-b/(2a)/x]
		two, err := u.numInt(2)
		if err != nil {
			return err
		}
		a2, err := u.mkMul(two, a)
		if err != nil {
			return err
		}
		negB, err := u.mkUminus(b)
		if err != nil {
			return err
		}
		e1 := &nlSqrtForm{a: negB, b: 0, c: zero, d: a2}
		es = es[:0]
		subst = nil
		cond, err := u.mkNe(a)
		if err != nil {
			return err
		}
		es = append(es, cond)
		pi, err := u.pred(bc, i)
		if err != nil {
			return err
		}
		es = append(es, pi)
		for k := range polys {
			t1, err := u.mkSubst(cmp, polys[k], comps[k], e1)
			if err != nil {
				return err
			}
			pk, err := u.pred(bc, k)
			if err != nil {
				return err
			}
			imp, err := u.rawImplies(pk, t1)
			if err != nil {
				return err
			}
			es = append(es, imp)
			subst = append(subst, t1)
		}
		def, err := u.mkDef(cmp, nlABCPoly{a2, b, zero}, e1)
		if err != nil {
			return err
		}
		branch, err := u.mkAndN(es)
		if err != nil {
			return err
		}
		if err = bc.AddBranch(branch, cond, subst, def, a, b, c); err != nil {
			return err
		}
	}
	return nil
}

// createBranch is imp::create_branch(i, polys, comps, bc).
func (u *NonlinearArithmeticUtil) createBranch(i int, polys []nlPoly, comps []nlComp, bc *NLBranchConditions) error {
	cmp := comps[i]
	p := polys[i]
	zero, err := u.z()
	if err != nil {
		return err
	}
	a, b, c := nlCoefficients(p, zero)
	// d = mk_sub(mk_mul(b,b), mk_mul(num(4), a, c));
	ac, err := u.mkMul(a, c)
	if err != nil {
		return err
	}
	four, err := u.numInt(4)
	if err != nil {
		return err
	}
	fourAC, err := u.mkMul(four, ac)
	if err != nil {
		return err
	}
	bb, err := u.mkMul(b, b)
	if err != nil {
		return err
	}
	d, err := u.mkSub(bb, fourAC)
	if err != nil {
		return err
	}
	// a2 = mk_mul(a, num(2));
	two, err := u.numInt(2)
	if err != nil {
		return err
	}
	a2, err := u.mkMul(a, two)
	if err != nil {
		return err
	}
	var es, subst []ast.ASTHandle
	if b != zero {
		negC, err := u.mkUminus(c)
		if err != nil {
			return err
		}
		e0 := &nlSqrtForm{a: negC, b: 0, c: zero, d: b}
		if a != zero {
			eq, err := u.mkEq(a)
			if err != nil {
				return err
			}
			es = append(es, eq)
		}
		ne, err := u.mkNe(b)
		if err != nil {
			return err
		}
		es = append(es, ne)
		cond, err := u.mkAndN(es)
		if err != nil {
			return err
		}
		pi, err := u.pred(bc, i)
		if err != nil {
			return err
		}
		es = append(es, pi)
		for j := range polys {
			t1, err := u.mkSubst(cmp, polys[j], comps[j], e0)
			if err != nil {
				return err
			}
			t2, err := u.replaceZero(a, t1)
			if err != nil {
				return err
			}
			pj, err := u.pred(bc, j)
			if err != nil {
				return err
			}
			imp, err := u.rawImplies(pj, t2)
			if err != nil {
				return err
			}
			es = append(es, imp)
			subst = append(subst, t2)
		}
		branch, err := u.mkAndN(es)
		if err != nil {
			return err
		}
		def, err := u.mkDef(cmp, nlABCPoly{zero, b, c}, e0)
		if err != nil {
			return err
		}
		if err = bc.AddBranch(branch, cond, subst, def, a, b, c); err != nil {
			return err
		}
	}
	if a != zero {
		negB1, err := u.mkUminus(b)
		if err != nil {
			return err
		}
		e1 := &nlSqrtForm{a: negB1, b: 1, c: d, d: a2}
		negB2, err := u.mkUminus(b)
		if err != nil {
			return err
		}
		e2 := &nlSqrtForm{a: negB2, b: -1, c: d, d: a2}
		es = es[:0]
		subst = nil
		ne, err := u.mkNe(a)
		if err != nil {
			return err
		}
		es = append(es, ne)
		ge, err := u.mkGe(d)
		if err != nil {
			return err
		}
		es = append(es, ge)
		cond, err := u.mkAndN(es)
		if err != nil {
			return err
		}
		pi, err := u.pred(bc, i)
		if err != nil {
			return err
		}
		es = append(es, pi)
		for j := range polys {
			t1, err := u.mkSubst(cmp, polys[j], comps[j], e1)
			if err != nil {
				return err
			}
			pj, err := u.pred(bc, j)
			if err != nil {
				return err
			}
			imp, err := u.rawImplies(pj, t1)
			if err != nil {
				return err
			}
			es = append(es, imp)
			subst = append(subst, t1)
		}
		branch, err := u.mkAndN(es)
		if err != nil {
			return err
		}
		def, err := u.mkDef(cmp, nlABCPoly{a, b, c}, e1)
		if err != nil {
			return err
		}
		if err = bc.AddBranch(branch, cond, subst, def, a, b, c); err != nil {
			return err
		}
		// es.resize(3); subst.reset();
		es = es[:3]
		subst = nil
		for j := range polys {
			t1, err := u.mkSubst(cmp, polys[j], comps[j], e2)
			if err != nil {
				return err
			}
			pj, err := u.pred(bc, j)
			if err != nil {
				return err
			}
			imp, err := u.rawImplies(pj, t1)
			if err != nil {
				return err
			}
			es = append(es, imp)
			subst = append(subst, t1)
		}
		branch, err = u.mkAndN(es)
		if err != nil {
			return err
		}
		def, err = u.mkDef(cmp, nlABCPoly{a, b, c}, e2)
		if err != nil {
			return err
		}
		if err = bc.AddBranch(branch, cond, subst, def, a, b, c); err != nil {
			return err
		}
	}
	return nil
}
