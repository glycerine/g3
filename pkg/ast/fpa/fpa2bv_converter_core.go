package fpa

// Source: src/ast/fpa/fpa2bv_converter.cpp:49-389 (mk_eq .. mk_one) and
// 3342-3706 (mk_fp .. mk_rounding_mode).

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// mkEq is mk_eq(a, b, result).
func (c *FPA2BVConverter) mkEq(a, b ast.ASTHandle, result *eref) {
	if c.isFloat(a) && c.isFloat(b) {
		f := c.frame()
		defer f.done()
		eqSgn, eqExp, eqSig := f.ref(), f.ref(), f.ref()
		c.simpEq(c.arg(a, 0), c.arg(b, 0), eqSgn)
		c.simpEq(c.arg(a, 1), c.arg(b, 1), eqExp)
		c.simpEq(c.arg(a, 2), c.arg(b, 2), eqSig)
		bothTheSame := f.ref()
		c.simpAnd(bothTheSame, eqSgn.get(), eqExp.get(), eqSig.get())
		aIsNan, bIsNan, bothAreNan := f.ref(), f.ref(), f.ref()
		c.mkIsNan(a, aIsNan)
		c.mkIsNan(b, bIsNan)
		c.simpAnd(bothAreNan, aIsNan.get(), bIsNan.get())
		c.simpOr(result, bothAreNan.get(), bothTheSame.get())
	} else if c.isRM(a) && c.isRM(b) {
		c.simpEq(c.arg(a, 0), c.arg(b, 0), result)
	} else {
		c.chk(ast.ErrASTDomain)
	}
}

// mkIte is mk_ite(c, t, f, result).
func (c *FPA2BVConverter) mkIte(cond, t, e ast.ASTHandle, result *eref) {
	if c.isFP(t) && c.isFP(e) {
		tSgn, tExp, tSig := c.splitFP(t)
		fSgn, fExp, fSig := c.splitFP(e)
		f := c.frame()
		defer f.done()
		sgn, s, ex := f.ref(), f.ref(), f.ref()
		c.simpIte(cond, tSgn, fSgn, sgn)
		c.simpIte(cond, tSig, fSig, s)
		c.simpIte(cond, tExp, fExp, ex)
		result.set(c.mkFP(sgn.get(), ex.get(), s.get()))
	} else if c.isRM(t) && c.isRM(e) {
		c.simpIte(cond, c.arg(t, 0), c.arg(e, 0), result)
		result.set(c.mkBV2RM(result.get()))
	} else {
		c.chk(ast.ErrASTDomain)
	}
}

// mkDistinct is mk_distinct(f, num, args, result).
func (c *FPA2BVConverter) mkDistinct(args []ast.ASTHandle, result *eref) {
	result.set(c.mTrue())
	for i := 0; i < len(args); i++ {
		for j := i + 1; j < len(args); j++ {
			func() {
				f := c.frame()
				defer f.done()
				eq, neq := f.ref(), f.ref()
				c.mkEq(args[i], args[j], eq)
				neq.set(c.mNot(eq.get()))
				c.simpAnd(result, result.get(), neq.get())
			}()
		}
	}
}

// mkNumeral is mk_numeral(f, num, args, result) for the numeral app n.
func (c *FPA2BVConverter) mkNumeral(n ast.ASTHandle, result *eref) {
	s := c.sortOf(n)
	nan, isNan, err := c.m.ClassifyFPANumeral(n, ast.FPAIsNaN)
	c.chk(err)
	if !nan {
		c.chk(ast.ErrASTDomain)
	}
	if isNan {
		c.mkNanSort(s, result)
		return
	}
	parts, ok, err := c.m.ReadFPANumeralParts(n)
	c.chk(err)
	if !ok {
		c.chk(ast.ErrASTDomain)
	}
	sig, ok := new(big.Int).SetString(parts.Significand, 10)
	if !ok {
		c.chk(ast.ErrASTDomain)
	}
	c.mkNumeralValue(s, parts.Sign, sig, parts.Exponent, false, parts.Infinite, result)
}

// mkNumeralValue is mk_numeral(sort * s, mpf const & v, result) given v's
// sign, significand, exponent and NaN/infinity class.
func (c *FPA2BVConverter) mkNumeralValue(s ast.ASTHandle, sign bool, sig *big.Int, exp int64, nan, inf bool, result *eref) {
	ebits, sbits := c.sortShape(s)
	if nan {
		c.mkNanSort(s, result)
	} else if inf {
		if sign {
			c.mkNinfSort(s, result)
		} else {
			c.mkPinfSort(s, result)
		}
	} else {
		f := c.frame()
		defer f.done()
		bvSgn, bvSig, e, biasedExp := f.ref(), f.ref(), f.ref(), f.ref()
		var sg int64
		if sign {
			sg = 1
		}
		bvSgn.set(c.bvNum(sg, 1))
		bvSig.set(c.bvNumBig(sig, sbits-1))
		e.set(c.bvNum(exp, ebits))
		c.mkBias(e.get(), biasedExp)
		result.set(c.mkFP(bvSgn.get(), biasedExp.get(), bvSig.get()))
	}
}

// mkFreshConst is mk_fresh_const(prefix, sz).
func (c *FPA2BVConverter) mkFreshConst(prefix *string, sz uint32) ast.ASTHandle {
	return c.must(c.m.MkFreshConstNullable(prefix, c.bvSort(sz)))
}

// mkConst is the virtual mk_const(f, result).
func (c *FPA2BVConverter) mkConst(f ast.ASTHandle, result *eref) {
	if c.overrides.MkConst != nil {
		c.chk(c.overrides.MkConst(c, f, result.r))
		return
	}
	c.mkConstBase(f, result)
}

// MkConstBase is fpa2bv_converter::mk_const for a derived converter.
func (c *FPA2BVConverter) MkConstBase(f ast.ASTHandle, result ast.ASTRef) (err error) {
	defer c.guard(&err)
	c.mkConstBase(f, &eref{c: c, r: result})
	return nil
}

func (c *FPA2BVConverter) mkConstBase(f ast.ASTHandle, result *eref) {
	if r, ok, err := c.const2bv.Find(f); err != nil {
		c.chk(err)
	} else if ok {
		result.set(r[0])
		return
	}
	srt := c.rangeOf(f)
	ebits, sbits := c.sortShape(srt)
	g := c.frame()
	defer g.done()
	sgn, s, e := g.ref(), g.ref(), g.ref()
	if ast.ASTSourceDebug {
		name, err := c.m.Symbols.Display(c.must2name(f))
		c.chk(err)
		ps, pe, pg := "fpa2bv_sgn_"+name, "fpa2bv_exp_"+name, "fpa2bv_sig_"+name
		sgn.set(c.mkFreshConst(&ps, 1))
		e.set(c.mkFreshConst(&pe, ebits))
		s.set(c.mkFreshConst(&pg, sbits-1))
	} else {
		bv := g.ref()
		bvSz := 1 + ebits + (sbits - 1)
		bv.set(c.mkFreshConst(nil, bvSz))
		sgn.set(c.extract(bvSz-1, bvSz-1, bv.get()))
		e.set(c.extract(bvSz-2, sbits-1, bv.get()))
		s.set(c.extract(sbits-2, 0, bv.get()))
	}
	result.set(c.mkFP(sgn.get(), e.get(), s.get()))
	c.chk(c.const2bv.Insert(f, [2]ast.ASTHandle{result.get()}))
	c.chk(c.m.IncRef(f))
	c.chk(c.m.IncRef(result.get()))
}

func (c *FPA2BVConverter) must2name(f ast.ASTHandle) usymbol.Symbol {
	n, err := c.m.GetASTName(f)
	c.chk(err)
	return n
}

// mkVar is mk_var(base_inx, srt, result).
func (c *FPA2BVConverter) mkVar(baseInx uint32, srt ast.ASTHandle, result *eref) {
	ebits, sbits := c.sortShape(srt)
	f := c.frame()
	defer f.done()
	sgn, s, e := f.ref(), f.ref(), f.ref()
	sgn.set(c.must(c.m.MkVar(baseInx, c.bvSort(1))))
	s.set(c.must(c.m.MkVar(baseInx+1, c.bvSort(sbits-1))))
	e.set(c.must(c.m.MkVar(baseInx+2, c.bvSort(ebits))))
	result.set(c.mkFP(sgn.get(), e.get(), s.get()))
}

// mkUF is the virtual mk_uf(f, num, args, result).
func (c *FPA2BVConverter) mkUF(f ast.ASTHandle, args []ast.ASTHandle, result *eref) {
	if c.overrides.MkUF != nil {
		c.chk(c.overrides.MkUF(c, f, args, result.r))
		return
	}
	c.mkUFBase(f, args, result)
}

// MkUFBase is fpa2bv_converter::mk_uf for a derived converter.
func (c *FPA2BVConverter) MkUFBase(f ast.ASTHandle, args []ast.ASTHandle, result ast.ASTRef) (err error) {
	defer c.guard(&err)
	c.mkUFBase(f, args, &eref{c: c, r: result})
	return nil
}

func (c *FPA2BVConverter) domainOf(f ast.ASTHandle) []ast.ASTHandle {
	n := c.mustU(c.m.GetArity(f))
	out := make([]ast.ASTHandle, n)
	for i := range out {
		out[i] = c.must(c.m.GetDomain(f, uint32(i)))
	}
	return out
}

func (c *FPA2BVConverter) mkUFBase(f ast.ASTHandle, args []ast.ASTHandle, result *eref) {
	g := c.frame()
	defer g.done()
	fapp, feq := g.ref(), g.ref()
	_ = feq
	rng := g.ref() // sort_ref
	bvApp, fltApp := g.ref(), g.ref()
	rng.set(c.rangeOf(f))
	fapp.set(c.must(c.m.MkApp(f, args)))
	if c.isFloatSort(rng.get()) {
		h := c.frame()
		defer h.done()
		bvRng, newEq := h.ref(), h.ref()
		ebits, sbits := c.sortShape(rng.get())
		bvSz := ebits + sbits
		bvRng.set(c.bvSort(bvSz))
		bvF := c.mkBVUF(f, c.domainOf(f), bvRng.get())
		bvApp.set(c.must(c.m.MkApp(bvF, args)))
		sig := c.extract(sbits-2, 0, bvApp.get())
		exp := c.extract(sbits+ebits-2, sbits-1, bvApp.get())
		sgn := c.extract(bvSz-1, bvSz-1, bvApp.get())
		fltApp.set(c.mkFP(sgn, exp, sig))
		newEq.set(c.mEq(fapp.get(), fltApp.get()))
		c.pushExtraAssertion(newEq.get())
		result.set(fltApp.get())
	} else if c.isRMSort(rng.get()) {
		h := c.frame()
		defer h.done()
		bvRng, newEq := h.ref(), h.ref()
		bvRng.set(c.bvSort(3))
		bvF := c.mkBVUF(f, c.domainOf(f), bvRng.get())
		bvApp.set(c.must(c.m.MkApp(bvF, args)))
		fltApp.set(c.mkBV2RM(bvApp.get()))
		newEq.set(c.mEq(fapp.get(), fltApp.get()))
		c.pushExtraAssertion(newEq.get())
		result.set(fltApp.get())
	} else {
		result.set(fapp.get())
	}
}

// mkRMConst is the virtual mk_rm_const(f, result).
func (c *FPA2BVConverter) mkRMConst(f ast.ASTHandle, result *eref) {
	if c.overrides.MkRMConst != nil {
		c.chk(c.overrides.MkRMConst(c, f, result.r))
		return
	}
	c.mkRMConstBase(f, result)
}

// MkRMConstBase is fpa2bv_converter::mk_rm_const for a derived converter.
func (c *FPA2BVConverter) MkRMConstBase(f ast.ASTHandle, result ast.ASTRef) (err error) {
	defer c.guard(&err)
	c.mkRMConstBase(f, &eref{c: c, r: result})
	return nil
}

func (c *FPA2BVConverter) mkRMConstBase(f ast.ASTHandle, result *eref) {
	if r, ok, err := c.rmConst2bv.Find(f); err != nil {
		c.chk(err)
	} else if ok {
		result.set(r[0])
		return
	}
	g := c.frame()
	defer g.done()
	bv3 := g.ref()
	var prefix *string
	if ast.ASTSourceDebug {
		p := "fpa2bv_rm"
		prefix = &p
	}
	bv3.set(c.must(c.m.MkFreshConstNullable(prefix, c.bvSort(3))))
	result.set(c.mkBV2RM(bv3.get()))
	c.chk(c.rmConst2bv.Insert(f, [2]ast.ASTHandle{result.get()}))
	c.chk(c.m.IncRef(f))
	c.chk(c.m.IncRef(result.get()))
	rcc := g.ref()
	rcc.set(c.ule(bv3.get(), c.bvNum(4, 3)))
	c.pushExtraAssertion(rcc.get())
}

// ---- special values ----

func (c *FPA2BVConverter) mkPinfSort(s ast.ASTHandle, result *eref) {
	ebits, sbits := c.sortShape(s)
	f := c.frame()
	defer f.done()
	topExp := f.ref()
	c.mkTopExp(ebits, topExp)
	sig := c.bvNum(0, sbits-1)
	sgn := c.bvNum(0, 1)
	result.set(c.mkFP(sgn, topExp.get(), sig))
}

func (c *FPA2BVConverter) mkNinfSort(s ast.ASTHandle, result *eref) {
	ebits, sbits := c.sortShape(s)
	f := c.frame()
	defer f.done()
	topExp := f.ref()
	c.mkTopExp(ebits, topExp)
	sig := c.bvNum(0, sbits-1)
	sgn := c.bvNum(1, 1)
	result.set(c.mkFP(sgn, topExp.get(), sig))
}

func (c *FPA2BVConverter) mkNanSort(s ast.ASTHandle, result *eref) {
	ebits, sbits := c.sortShape(s)
	f := c.frame()
	defer f.done()
	topExp := f.ref()
	c.mkTopExp(ebits, topExp)
	sig := c.bvNum(1, sbits-1)
	sgn := c.bvNum(0, 1)
	result.set(c.mkFP(sgn, topExp.get(), sig))
}

func (c *FPA2BVConverter) mkNzeroSort(s ast.ASTHandle, result *eref) {
	ebits, sbits := c.sortShape(s)
	f := c.frame()
	defer f.done()
	botExp := f.ref()
	c.mkBotExp(ebits, botExp)
	sig := c.bvNum(0, sbits-1)
	sgn := c.bvNum(1, 1)
	result.set(c.mkFP(sgn, botExp.get(), sig))
}

func (c *FPA2BVConverter) mkPzeroSort(s ast.ASTHandle, result *eref) {
	ebits, sbits := c.sortShape(s)
	f := c.frame()
	defer f.done()
	botExp := f.ref()
	c.mkBotExp(ebits, botExp)
	sig := c.bvNum(0, sbits-1)
	sgn := c.bvNum(0, 1)
	result.set(c.mkFP(sgn, botExp.get(), sig))
}

// mkZero is mk_zero(s, sgn, result).
func (c *FPA2BVConverter) mkZero(s ast.ASTHandle, sgn ast.ASTHandle, result *eref) {
	f := c.frame()
	defer f.done()
	isPos, pzero, nzero := f.ref(), f.ref(), f.ref()
	isPos.set(c.mEq(sgn, c.bvNum(0, 1)))
	c.mkPzeroSort(s, pzero)
	c.mkNzeroSort(s, nzero)
	c.mkIte(isPos.get(), pzero.get(), nzero.get(), result)
}

// mkOne is mk_one(s, sign, result).
func (c *FPA2BVConverter) mkOne(s ast.ASTHandle, sign ast.ASTHandle, result *eref) {
	ebits, sbits := c.sortShape(s)
	sig := c.bvNum(0, sbits-1)
	exp := c.bvNumBig(powers2m1(ebits-1, false), ebits)
	result.set(c.mkFP(sign, exp, sig))
}

// ---- mk_fp, split_fp, join_fp ----

// mkFPDecl is mk_fp(f, num, args, result).
func (c *FPA2BVConverter) mkFPDecl(args []ast.ASTHandle, result *eref) {
	result.set(c.mkFP(args[0], args[1], args[2]))
}

// splitFP is split_fp(e, sgn, exp, sig) with raw pointers.
func (c *FPA2BVConverter) splitFP(e ast.ASTHandle) (sgn, exp, sig ast.ASTHandle) {
	if !c.isFP(e) {
		c.chk(ast.ErrASTDomain)
	}
	return c.arg(e, 0), c.arg(e, 1), c.arg(e, 2)
}

// splitFPRefs is split_fp(e, expr_ref sgn, exp, sig).
func (c *FPA2BVConverter) splitFPRefs(e ast.ASTHandle, sgn, exp, sig *eref) {
	s, x, g := c.splitFP(e)
	sgn.set(s)
	exp.set(x)
	sig.set(g)
}

// joinFP is join_fp(e, res).
func (c *FPA2BVConverter) joinFP(e ast.ASTHandle, res *eref) {
	sgn, exp, sig := c.splitFP(e)
	res.set(c.concat(c.concat(sgn, exp), sig))
}

// ---- predicates ----

func (c *FPA2BVConverter) mkIsNan(e ast.ASTHandle, result *eref) {
	_, exp, sig := c.splitFP(e)
	f := c.frame()
	defer f.done()
	sigIsZero, sigIsNotZero, expIsTop, topExp, zero := f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
	c.mkTopExp(c.bvSize(exp), topExp)
	zero.set(c.bvNum(0, c.bvSize(sig)))
	c.simpEq(sig, zero.get(), sigIsZero)
	c.simpNot(sigIsZero.get(), sigIsNotZero)
	c.simpEq(exp, topExp.get(), expIsTop)
	c.simpAnd(result, expIsTop.get(), sigIsNotZero.get())
}

func (c *FPA2BVConverter) mkIsInf(e ast.ASTHandle, result *eref) {
	_, exp, sig := c.splitFP(e)
	f := c.frame()
	defer f.done()
	eq1, eq2, topExp, zero := f.ref(), f.ref(), f.ref(), f.ref()
	c.mkTopExp(c.bvSize(exp), topExp)
	zero.set(c.bvNum(0, c.bvSize(sig)))
	c.simpEq(sig, zero.get(), eq1)
	c.simpEq(exp, topExp.get(), eq2)
	c.simpAnd(result, eq1.get(), eq2.get())
}

func (c *FPA2BVConverter) mkIsPinf(e ast.ASTHandle, result *eref) {
	f := c.frame()
	defer f.done()
	eIsPos, eIsInf := f.ref(), f.ref()
	c.mkIsPos(e, eIsPos)
	c.mkIsInf(e, eIsInf)
	c.simpAnd(result, eIsPos.get(), eIsInf.get())
}

func (c *FPA2BVConverter) mkIsNinf(e ast.ASTHandle, result *eref) {
	f := c.frame()
	defer f.done()
	eIsNeg, eIsInf := f.ref(), f.ref()
	c.mkIsNeg(e, eIsNeg)
	c.mkIsInf(e, eIsInf)
	c.simpAnd(result, eIsNeg.get(), eIsInf.get())
}

func (c *FPA2BVConverter) mkIsPos(e ast.ASTHandle, result *eref) {
	if !c.isFP(e) {
		c.chk(ast.ErrASTDomain)
	}
	a0 := c.arg(e, 0)
	f := c.frame()
	defer f.done()
	zero := f.ref()
	zero.set(c.bvNum(0, c.bvSize(a0)))
	c.simpEq(a0, zero.get(), result)
}

func (c *FPA2BVConverter) mkIsNeg(e ast.ASTHandle, result *eref) {
	if !c.isFP(e) {
		c.chk(ast.ErrASTDomain)
	}
	a0 := c.arg(e, 0)
	f := c.frame()
	defer f.done()
	one := f.ref()
	one.set(c.bvNum(1, c.bvSize(a0)))
	c.simpEq(a0, one.get(), result)
}

func (c *FPA2BVConverter) mkIsZero(e ast.ASTHandle, result *eref) {
	_, exp, sig := c.splitFP(e)
	f := c.frame()
	defer f.done()
	eq1, eq2, botExp, zero := f.ref(), f.ref(), f.ref(), f.ref()
	c.mkBotExp(c.bvSize(exp), botExp)
	zero.set(c.bvNum(0, c.bvSize(sig)))
	c.simpEq(sig, zero.get(), eq1)
	c.simpEq(exp, botExp.get(), eq2)
	c.simpAnd(result, eq1.get(), eq2.get())
}

func (c *FPA2BVConverter) mkIsNzero(e ast.ASTHandle, result *eref) {
	sgn, _, _ := c.splitFP(e)
	f := c.frame()
	defer f.done()
	eIsZero, eq, one1 := f.ref(), f.ref(), f.ref()
	c.mkIsZero(e, eIsZero)
	one1.set(c.bvNum(1, 1))
	c.simpEq(sgn, one1.get(), eq)
	c.simpAnd(result, eq.get(), eIsZero.get())
}

func (c *FPA2BVConverter) mkIsPzero(e ast.ASTHandle, result *eref) {
	sgn, _, _ := c.splitFP(e)
	f := c.frame()
	defer f.done()
	eIsZero, eq, nil1 := f.ref(), f.ref(), f.ref()
	c.mkIsZero(e, eIsZero)
	nil1.set(c.bvNum(0, 1))
	c.simpEq(sgn, nil1.get(), eq)
	c.simpAnd(result, eq.get(), eIsZero.get())
}

func (c *FPA2BVConverter) mkIsDenormal(e ast.ASTHandle, result *eref) {
	_, exp, _ := c.splitFP(e)
	f := c.frame()
	defer f.done()
	zero, zexp, isZero, nIsZero := f.ref(), f.ref(), f.ref(), f.ref()
	zero.set(c.bvNum(0, c.bvSize(exp)))
	c.simpEq(exp, zero.get(), result)
	c.simpEq(exp, zero.get(), zexp)
	c.mkIsZero(e, isZero)
	c.simpNot(isZero.get(), nIsZero)
	c.simpAnd(result, nIsZero.get(), zexp.get())
}

func (c *FPA2BVConverter) mkIsNormal(e ast.ASTHandle, result *eref) {
	_, exp, _ := c.splitFP(e)
	f := c.frame()
	defer f.done()
	isSpecial, isDenormal, p, isZero := f.ref(), f.ref(), f.ref(), f.ref()
	c.mkIsDenormal(e, isDenormal)
	c.mkIsZero(e, isZero)
	ebits := c.bvSize(exp)
	p.set(c.bvNumBig(powers2m1(ebits, false), ebits))
	c.simpEq(exp, p.get(), isSpecial)
	orEx := f.ref()
	c.simpOr(orEx, isSpecial.get(), isDenormal.get())
	c.simpOr(orEx, isZero.get(), orEx.get())
	c.simpNot(orEx.get(), result)
}

// mkIsRM is mk_is_rm(rme, rm, result).
func (c *FPA2BVConverter) mkIsRM(rme ast.ASTHandle, rm int64, result *eref) {
	f := c.frame()
	defer f.done()
	rmNum := f.ref()
	rmNum.set(c.bvNum(rm, 3))
	switch rm {
	case bvRMTiesToAway, bvRMTiesToEven, bvRMToNegative, bvRMToPositive, bvRMToZero:
		c.simpEq(rme, rmNum.get(), result)
	default:
		c.chk(ast.ErrASTDomain)
	}
}

// ---- exponents ----

func (c *FPA2BVConverter) mkTopExp(sz uint32, result *eref) {
	result.set(c.bvNumBig(powers2m1(sz, false), sz))
}

func (c *FPA2BVConverter) mkBotExp(sz uint32, result *eref) {
	result.set(c.bvNum(0, sz))
}

func (c *FPA2BVConverter) mkMinExp(ebits uint32, result *eref) {
	z := powers2m1(ebits-1, true)
	result.set(c.bvNumBig(z.Add(z, big.NewInt(1)), ebits))
}

func (c *FPA2BVConverter) mkMaxExp(ebits uint32, result *eref) {
	result.set(c.bvNumBig(powers2m1(ebits-1, false), ebits))
}

// mkLeadingZeros is mk_leading_zeros(e, max_bits, result).
func (c *FPA2BVConverter) mkLeadingZeros(e ast.ASTHandle, maxBits uint32, result *eref) {
	bvSz := c.bvSize(e)
	if bvSz == 0 {
		result.set(c.bvNum(0, maxBits))
	} else if bvSz == 1 {
		f := c.frame()
		defer f.done()
		eq, nil1, oneM, nilM := f.ref(), f.ref(), f.ref(), f.ref()
		nil1.set(c.bvNum(0, 1))
		oneM.set(c.bvNum(1, maxBits))
		nilM.set(c.bvNum(0, maxBits))
		c.simpEq(e, nil1.get(), eq)
		c.simpIte(eq.get(), oneM.get(), nilM.get(), result)
	} else {
		f := c.frame()
		defer f.done()
		H, L := f.ref(), f.ref()
		H.set(c.extract(bvSz-1, bvSz/2, e))
		L.set(c.extract(bvSz/2-1, 0, e))
		hSize := c.bvSize(H.get())
		lzH, lzL := f.ref(), f.ref()
		c.mkLeadingZeros(H.get(), maxBits, lzH)
		c.mkLeadingZeros(L.get(), maxBits, lzL)
		hIsZero, nilH := f.ref(), f.ref()
		nilH.set(c.bvNum(0, hSize))
		c.simpEq(H.get(), nilH.get(), hIsZero)
		sum, hM := f.ref(), f.ref()
		hM.set(c.bvNum(int64(hSize), maxBits))
		sum.set(c.bvAdd(hM.get(), lzL.get()))
		c.simpIte(hIsZero.get(), sum.get(), lzH.get(), result)
	}
}

// mkBias is mk_bias(e, result).
func (c *FPA2BVConverter) mkBias(e ast.ASTHandle, result *eref) {
	ebits := c.bvSize(e)
	f := c.frame()
	defer f.done()
	bias := f.ref()
	bias.set(c.bvNumBig(powers2m1(ebits-1, false), ebits))
	result.set(c.bvAdd(e, bias.get()))
}

// mkUnbias is mk_unbias(e, result).
func (c *FPA2BVConverter) mkUnbias(e ast.ASTHandle, result *eref) {
	ebits := c.bvSize(e)
	f := c.frame()
	defer f.done()
	ePlusOne := f.ref()
	ePlusOne.set(c.bvAdd(e, c.bvNum(1, ebits)))
	leading, nLeading, rest := f.ref(), f.ref(), f.ref()
	leading.set(c.extract(ebits-1, ebits-1, ePlusOne.get()))
	nLeading.set(c.bvNot(leading.get()))
	rest.set(c.extract(ebits-2, 0, ePlusOne.get()))
	result.set(c.concat(nLeading.get(), rest.get()))
}

// unpack is unpack(e, sgn, sig, exp, lz, normalize).
func (c *FPA2BVConverter) unpack(e ast.ASTHandle, sgn, sig, exp, lz *eref, normalize bool) {
	if !c.isFP(e) {
		c.chk(ast.ErrASTDomain)
	}
	srt := c.rangeOf(c.must(c.m.GetDecl(e)))
	ebits, sbits := c.sortShape(srt)
	c.splitFPRefs(e, sgn, exp, sig)
	f := c.frame()
	defer f.done()
	isNormal := f.ref()
	c.mkIsNormal(e, isNormal)
	normalSig, normalExp := f.ref(), f.ref()
	one1 := c.bvNum(1, 1)
	normalSig.set(c.concat(one1, sig.get()))
	c.mkUnbias(exp.get(), normalExp)
	denormalSig, denormalExp := f.ref(), f.ref()
	denormalSig.set(c.zeroExtend(1, sig.get()))
	denormalExp.set(c.bvNum(1, ebits))
	c.mkUnbias(denormalExp.get(), denormalExp)
	zeroE := f.ref()
	zeroE.set(c.bvNum(0, ebits))
	if normalize {
		g := c.frame()
		isSigZero, zeroS := g.ref(), g.ref()
		zeroS.set(c.bvNum(0, sbits))
		c.simpEq(zeroS.get(), denormalSig.get(), isSigZero)
		lzD, normOrZero := g.ref(), g.ref()
		c.mkLeadingZeros(denormalSig.get(), ebits, lzD)
		normOrZero.set(c.mOr(isNormal.get(), isSigZero.get()))
		c.simpIte(normOrZero.get(), zeroE.get(), lzD.get(), lz)
		shift := g.ref()
		c.simpIte(isSigZero.get(), zeroE.get(), lz.get(), shift)
		if ebits <= sbits {
			h := c.frame()
			q := h.ref()
			q.set(c.zeroExtend(sbits-ebits, shift.get()))
			denormalSig.set(c.bvShl(denormalSig.get(), q.get()))
			h.done()
		} else {
			h := c.frame()
			zeroEms, sh, isShZero, sl, sbitsS, shortShift := h.ref(), h.ref(), h.ref(), h.ref(), h.ref(), h.ref()
			zeroEms.set(c.bvNum(0, ebits-sbits))
			sbitsS.set(c.bvNum(int64(sbits), sbits))
			sh.set(c.extract(ebits-1, sbits, shift.get()))
			c.simpEq(zeroEms.get(), sh.get(), isShZero)
			shortShift.set(c.extract(sbits-1, 0, shift.get()))
			c.simpIte(isShZero.get(), shortShift.get(), sbitsS.get(), sl)
			denormalSig.set(c.bvShl(denormalSig.get(), sl.get()))
			h.done()
		}
		g.done()
	} else {
		lz.set(zeroE.get())
	}
	c.simpIte(isNormal.get(), normalSig.get(), denormalSig.get(), sig)
	c.simpIte(isNormal.get(), normalExp.get(), denormalExp.get(), exp)
}

// mkRoundingMode is mk_rounding_mode(k, result).
func (c *FPA2BVConverter) mkRoundingMode(k int32, result *eref) {
	switch k {
	case ast.FPAOpRMNearestEven:
		result.set(c.bvNum(bvRMTiesToEven, 3))
	case ast.FPAOpRMNearestAway:
		result.set(c.bvNum(bvRMTiesToAway, 3))
	case ast.FPAOpRMTowardPositive:
		result.set(c.bvNum(bvRMToPositive, 3))
	case ast.FPAOpRMTowardNegative:
		result.set(c.bvNum(bvRMToNegative, 3))
	case ast.FPAOpRMTowardZero:
		result.set(c.bvNum(bvRMToZero, 3))
	default:
		c.chk(ast.ErrASTDomain)
	}
	result.set(c.mkBV2RM(result.get()))
}
