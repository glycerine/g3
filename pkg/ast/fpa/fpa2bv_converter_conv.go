package fpa

// Source: src/ast/fpa/fpa2bv_converter.cpp:2273-3340 (mk_to_fp,
// mk_to_fp_float, mk_to_fp_real, mk_to_fp_real_int, mk_to_real,
// mk_to_fp_signed, mk_to_fp_unsigned, mk_to_ieee_bv(_unspecified), mk_to_bv,
// mk_to_ubv, mk_to_sbv, nan_wrap, mk_to_bv_unspecified,
// mk_to_real_unspecified).

import (
	"math"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// ---- arith_util ----

func (c *FPA2BVConverter) arithFid() ast.FamilyID {
	fid, err := c.m.ArithmeticFamilyID()
	c.chk(err)
	return fid
}

// arithSortKind returns 1 for Int, 0 for Real, -1 otherwise.
func (c *FPA2BVConverter) arithSortKind(e ast.ASTHandle) int32 {
	s := c.sortOf(e)
	fid, err := c.m.GetFamilyID(s)
	c.chk(err)
	if fid != c.arithFid() {
		return -1
	}
	k, err := c.m.GetDeclKind(s)
	c.chk(err)
	return k
}

func (c *FPA2BVConverter) isIntExpr(e ast.ASTHandle) bool  { return c.arithSortKind(e) == 1 }
func (c *FPA2BVConverter) isRealExpr(e ast.ASTHandle) bool { return c.arithSortKind(e) == 0 }

// arithNumeral is m_arith_util.is_numeral(e, q).
func (c *FPA2BVConverter) arithNumeral(e ast.ASTHandle) (*big.Rat, bool) {
	text, _, ok, err := c.m.ArithmeticNumeralText(e)
	c.chk(err)
	if !ok {
		return nil, false
	}
	q, ok := new(big.Rat).SetString(text)
	if !ok {
		c.chk(ast.ErrASTDomain)
	}
	return q, true
}

func (c *FPA2BVConverter) arithNum(q *big.Rat, isInt bool) ast.ASTHandle {
	return c.must(c.m.ArithmeticNumeralRat(q, isInt))
}
func (c *FPA2BVConverter) arithApp(op int32, args ...ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkFamilyApp(c.arithFid(), op, nil, args))
}
func (c *FPA2BVConverter) arithMul(a, b ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkArithmeticMul(a, b))
}
func (c *FPA2BVConverter) arithAdd(a, b ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkArithmeticAdd(a, b))
}
func (c *FPA2BVConverter) arithDiv(a, b ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkArithmeticDiv(a, b))
}
func (c *FPA2BVConverter) arithPower(a, b ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkArithmeticPower(a, b))
}
func (c *FPA2BVConverter) arithUminus(a ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkArithmeticNeg(a))
}

// bvNumeral is m_bv_util.is_numeral(e, val, sz).
func (c *FPA2BVConverter) bvNumeral(e ast.ASTHandle) (*big.Int, bool) {
	v, _, ok, err := c.m.BitVectorNumeralBigInt(e)
	c.chk(err)
	return v, ok
}

func (c *FPA2BVConverter) isBV(e ast.ASTHandle) bool {
	ok, err := c.m.IsBitVectorSort(c.sortOf(e))
	c.chk(err)
	return ok
}

// ---- mk_to_fp ----

// mkToFP is mk_to_fp(f, num, args, result).
func (c *FPA2BVConverter) mkToFP(fd ast.ASTHandle, args []ast.ASTHandle, result *eref) {
	num := len(args)
	switch {
	case num == 1 && c.isBV(args[0]):
		s := c.rangeOf(fd)
		toEbits, _ := c.sortShape(s)
		bv := args[0]
		sz := c.bvSize(bv)
		sig := c.extract(sz-toEbits-2, 0, bv)
		exp := c.extract(sz-2, sz-toEbits-1, bv)
		sgn := c.extract(sz-1, sz-1, bv)
		result.set(c.mkFP(sgn, exp, sig))
	case num == 2 && c.isRM(args[0]) && c.isFloatSort(c.sortOf(args[1])):
		c.mkToFPFloatDecl(c.rangeOf(fd), args[0], args[1], result)
	case num == 2 && c.isRM(args[0]) && (c.isIntExpr(args[1]) || c.isRealExpr(args[1])):
		c.mkToFPReal(fd, c.rangeOf(fd), args[0], args[1], result)
	case num == 2 && c.isRM(args[0]) && c.isBV(args[1]):
		c.mkToFPSigned(fd, args, result)
	case num == 3 && c.isBV(args[0]) && c.isBV(args[1]) && c.isBV(args[2]):
		result.set(c.mkFP(args[0], args[1], args[2]))
	case num == 3 && c.isRM(args[0]) && c.isArithNumeral(args[1]) && c.isArithNumeral(args[2]):
		c.mkToFPRealInt(fd, args, result)
	default:
		c.chk(ast.ErrASTDomain)
	}
}

func (c *FPA2BVConverter) isArithNumeral(e ast.ASTHandle) bool {
	_, ok := c.arithNumeral(e)
	return ok
}

// mkToFPFloatDecl is mk_to_fp_float(f, s, rm, x, result).
func (c *FPA2BVConverter) mkToFPFloatDecl(s, rm, x ast.ASTHandle, result *eref) {
	c.mkToFPFloat(s, c.rmArg(rm), x, result)
}

// mkToFPFloat is mk_to_fp_float(to_srt, rm, x, result).
func (c *FPA2BVConverter) mkToFPFloat(toSrt, rm, x ast.ASTHandle, result *eref) {
	fromEbits, fromSbits := c.sortShape(c.sortOf(x))
	toEbits, toSbits := c.sortShape(toSrt)
	if fromSbits == toSbits && fromEbits == toEbits {
		result.set(x)
		return
	}
	f := c.frame()
	defer f.done()
	c1, c2, c3, c4, c5 := f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
	v1, v2, v3, v4, v5, v6 := f.ref(), f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
	one1 := f.ref()
	one1.set(c.bvNum(1, 1))
	ninf, pinf := f.ref(), f.ref()
	c.mkPinfSort(toSrt, pinf)
	c.mkNinfSort(toSrt, ninf)
	c.mkIsNan(x, c1)
	c.mkNanSort(toSrt, v1)
	c.mkIsPzero(x, c2)
	c.mkPzeroSort(toSrt, v2)
	c.mkIsNzero(x, c3)
	c.mkNzeroSort(toSrt, v3)
	c.mkIsPinf(x, c4)
	v4.set(pinf.get())
	c.mkIsNinf(x, c5)
	v5.set(ninf.get())
	sgn, sig, exp, lz := f.ref(), f.ref(), f.ref(), f.ref()
	c.unpack(x, sgn, sig, exp, lz, true)
	resSgn, resSig, resExp := f.ref(), f.ref(), f.ref()
	resSgn.set(sgn.get())
	if fromSbits < toSbits+3 {
		resSig.set(c.concat(sig.get(), c.bvNum(0, toSbits+3-fromSbits)))
	} else if fromSbits > toSbits+3 {
		g := c.frame()
		sticky, low, high := g.ref(), g.ref(), g.ref()
		high.set(c.extract(fromSbits-1, fromSbits-toSbits-2, sig.get()))
		low.set(c.extract(fromSbits-toSbits-3, 0, sig.get()))
		sticky.set(c.redor(low.get()))
		resSig.set(c.concat(high.get(), sticky.get()))
		g.done()
	} else {
		resSig.set(sig.get())
	}
	resSig.set(c.zeroExtend(1, resSig.get()))
	exponentOverflow, exponentUnderflow := f.ref(), f.ref()
	exponentOverflow.set(c.mFalse())
	exponentUnderflow.set(c.mFalse())
	if fromEbits < toEbits+2 {
		resExp.set(c.signExtend(toEbits-fromEbits+2, exp.get()))
		g := c.frame()
		lzExt := g.ref()
		lzExt.set(c.zeroExtend(toEbits-fromEbits+2, lz.get()))
		resExp.set(c.bvSub(resExp.get(), lzExt.get()))
		g.done()
	} else if fromEbits > toEbits+2 {
		ebitsDiff := fromEbits - (toEbits + 2)
		g := c.frame()
		expSubLz := g.ref()
		sl := c.signExtend(2, lz.get())
		expSubLz.set(c.bvSub(c.signExtend(2, exp.get()), sl))
		maxExp, minExp, expInRange := g.ref(), g.ref(), g.ref()
		z := new(big.Int).Lsh(big.NewInt(1), uint(toEbits+1))
		z.Neg(z)
		low := c.bvNum(0, 1)
		maxExp.set(c.concat(c.bvNumBig(powers2m1(toEbits, false), toEbits+1), low))
		minExp.set(c.bvNumBig(z.Add(z, big.NewInt(2)), toEbits+2))
		firstOvfExp, firstUdfExp := g.ref(), g.ref()
		firstOvfExp.set(c.bvNumBig(powers2m1(toEbits+1, false), fromEbits+2))
		lo := c.bvNum(1, toEbits+1)
		hi := c.bvNeg(c.bvNum(1, ebitsDiff+3))
		firstUdfExp.set(c.concat(hi, lo))
		expInRange.set(c.extract(toEbits+1, 0, expSubLz.get()))
		ovfCond, udfCond := g.ref(), g.ref()
		ovfCond.set(c.sle(firstOvfExp.get(), expSubLz.get()))
		udfCond.set(c.sle(expSubLz.get(), firstUdfExp.get()))
		resExp.set(expInRange.get())
		resExp.set(c.mIte(ovfCond.get(), maxExp.get(), resExp.get()))
		resExp.set(c.mIte(udfCond.get(), minExp.get(), resExp.get()))
		g.done()
	} else {
		resExp.set(c.bvSub(exp.get(), lz.get()))
	}
	rounded, rmE := f.ref(), f.ref()
	rmE.set(rm)
	c.round(toSrt, rmE, resSgn, resSig, resExp, rounded)
	isNeg, sigInf := f.ref(), f.ref()
	c.simpEq(sgn.get(), one1.get(), isNeg)
	c.mkIte(isNeg.get(), ninf.get(), pinf.get(), sigInf)
	c.mkIte(exponentOverflow.get(), sigInf.get(), rounded.get(), v6)
	c.mkIte(c5.get(), v5.get(), v6.get(), result)
	c.mkIte(c4.get(), v4.get(), result.get(), result)
	c.mkIte(c3.get(), v3.get(), result.get(), result)
	c.mkIte(c2.get(), v2.get(), result.get(), result)
	c.mkIte(c1.get(), v1.get(), result.get(), result)
}

// fieldsFP builds mk_fp(sgn, bias(exp), sig) from an mpf's fields, through
// the sgn, sig, unbiased_exp and exp expr_refs of the caller.
func (c *FPA2BVConverter) fieldsFP(v ast.FPAMpfFields, ebits, sbits uint32, sgn, sig, exp, unbiasedExp *eref) {
	var sg int64
	if v.Sign {
		sg = 1
	}
	sgn.set(c.bvNum(sg, 1))
	sig.set(c.bvNumBig(v.Significand, sbits-1))
	unbiasedExp.set(c.bvNum(v.Exponent, ebits))
	c.mkBias(unbiasedExp.get(), exp)
}

// mkToFPReal is mk_to_fp_real(f, s, rm, x, result).
func (c *FPA2BVConverter) mkToFPReal(fd, s, rm, x ast.ASTHandle, result *eref) {
	bvRM := c.rmArg(rm)
	ebits, sbits := c.sortShape(s)
	rmv, rmNum := c.bvNumeral(bvRM)
	q, xNum := c.arithNumeral(x)
	if rmNum && xNum {
		mode, err := ast.FPARoundingMode(rmv.Int64())
		c.chk(err)
		if q.Sign() == 0 {
			c.mkPzeroSort(c.rangeOf(fd), result)
			return
		}
		v, err := c.m.FPAMpfSetRational(ebits, sbits, mode, q)
		c.chk(err)
		f := c.frame()
		defer f.done()
		sgn, sig, exp, unbiasedExp := f.ref(), f.ref(), f.ref(), f.ref()
		c.fieldsFP(v, ebits, sbits, sgn, sig, exp, unbiasedExp)
		result.set(c.mkFP(sgn.get(), exp.get(), sig.get()))
	} else if xNum {
		if q.Sign() == 0 {
			c.mkPzeroSort(c.rangeOf(fd), result)
			return
		}
		f := c.frame()
		defer f.done()
		rmNta, rmNte, rmTp, rmTn, rmTz := f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
		c.mkIsRM(bvRM, bvRMTiesToAway, rmNta)
		c.mkIsRM(bvRM, bvRMTiesToEven, rmNte)
		c.mkIsRM(bvRM, bvRMToPositive, rmTp)
		c.mkIsRM(bvRM, bvRMToNegative, rmTn)
		c.mkIsRM(bvRM, bvRMToZero, rmTz)
		var vs [5]ast.FPAMpfFields
		for i, r := range []int64{bvRMTiesToAway, bvRMTiesToEven, bvRMToPositive, bvRMToNegative, bvRMToZero} {
			mode, err := ast.FPARoundingMode(r)
			c.chk(err)
			vs[i], err = c.m.FPAMpfSetRational(ebits, sbits, mode, q)
			c.chk(err)
		}
		vNta, vNte, vTp, vTn := vs[0], vs[1], vs[2], vs[3]
		v1, v2, v3, v4 := f.ref(), f.ref(), f.ref(), f.ref()
		sgn, sig, exp, unbiasedExp := f.ref(), f.ref(), f.ref(), f.ref()
		c.fieldsFP(vNta, ebits, sbits, sgn, sig, exp, unbiasedExp)
		v1.set(c.mkFP(sgn.get(), exp.get(), sig.get()))
		c.fieldsFP(vNte, ebits, sbits, sgn, sig, exp, unbiasedExp)
		v2.set(c.mkFP(sgn.get(), exp.get(), sig.get()))
		c.fieldsFP(vTp, ebits, sbits, sgn, sig, exp, unbiasedExp)
		v3.set(c.mkFP(sgn.get(), exp.get(), sig.get()))
		c.fieldsFP(vTn, ebits, sbits, sgn, sig, exp, unbiasedExp)
		v4.set(c.mkFP(sgn.get(), exp.get(), sig.get()))
		c.fieldsFP(vTp, ebits, sbits, sgn, sig, exp, unbiasedExp)
		result.set(c.mkFP(sgn.get(), exp.get(), sig.get()))
		c.mkIte(rmTn.get(), v4.get(), result.get(), result)
		c.mkIte(rmTp.get(), v3.get(), result.get(), result)
		c.mkIte(rmNte.get(), v2.get(), result.get(), result)
		c.mkIte(rmNta.get(), v1.get(), result.get(), result)
	} else {
		f := c.frame()
		defer f.done()
		bv0, bv1, zero, two := f.ref(), f.ref(), f.ref(), f.ref()
		bv0.set(c.bvNum(0, 1))
		bv1.set(c.bvNum(1, 1))
		zero.set(c.arithNum(new(big.Rat), false))
		two.set(c.arithNum(big.NewRat(2, 1), false))
		sgn, sig, exp := f.ref(), f.ref(), f.ref()
		p1, p2, p3 := "fpa2bv_to_fp_real_sgn", "fpa2bv_to_fp_real_sig", "fpa2bv_to_fp_real_exp"
		sgn.set(c.mkFreshConst(&p1, 1))
		sig.set(c.mkFreshConst(&p2, sbits+4))
		exp.set(c.mkFreshConst(&p3, ebits+2))
		rme := f.ref()
		rme.set(bvRM)
		c.round(s, rme, sgn, sig, exp, result)
		toReal := c.must(c.m.MkFamilyApp(c.fid, ast.FPAOpToReal, nil, []ast.ASTHandle{result.get()}))
		c.pushExtraAssertion(c.mEq(toReal, x))
	}
}

// mkToFPRealInt is mk_to_fp_real_int(f, num, args, result).
func (c *FPA2BVConverter) mkToFPRealInt(fd ast.ASTHandle, args []ast.ASTHandle, result *eref) {
	ebits, sbits := c.sortShape(c.rangeOf(fd))
	bvRM := c.rmArg(args[0])
	var q, e *big.Rat
	if c.isIntExpr(args[1]) && c.isRealExpr(args[2]) {
		e, _ = c.arithNumeral(args[1])
		q, _ = c.arithNumeral(args[2])
	} else {
		e, _ = c.arithNumeral(args[2])
		q, _ = c.arithNumeral(args[1])
	}
	if e == nil || q == nil {
		c.chk(ast.ErrASTDomain)
	}
	if q.Sign() == 0 {
		c.mkPzeroSort(c.rangeOf(fd), result)
		return
	}
	exp := e.Num()
	var vals [5]ast.ASTHandle
	for i, r := range []int64{bvRMTiesToEven, bvRMTiesToAway, bvRMToPositive, bvRMToNegative, bvRMToZero} {
		mode, err := ast.FPARoundingMode(r)
		c.chk(err)
		vals[i] = c.must(c.m.MkFPANumeralExponentRational(ebits, sbits, mode, exp, q))
	}
	f := c.frame()
	defer f.done()
	aNte, aNta, aTp, aTn, aTz := f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
	aNte.set(vals[0])
	aNta.set(vals[1])
	aTp.set(vals[2])
	aTn.set(vals[3])
	aTz.set(vals[4])
	bvNte, bvNta, bvTp, bvTn, bvTz := f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
	c.mkNumeral(aNte.get(), bvNte)
	c.mkNumeral(aNta.get(), bvNta)
	c.mkNumeral(aTp.get(), bvTp)
	c.mkNumeral(aTn.get(), bvTn)
	c.mkNumeral(aTz.get(), bvTz)
	c1, c2, c3, c4 := f.ref(), f.ref(), f.ref(), f.ref()
	c1.set(c.mEq(bvRM, c.bvNum(bvRMToPositive, 3)))
	c2.set(c.mEq(bvRM, c.bvNum(bvRMToPositive, 3)))
	c3.set(c.mEq(bvRM, c.bvNum(bvRMTiesToAway, 3)))
	c4.set(c.mEq(bvRM, c.bvNum(bvRMTiesToEven, 3)))
	c.mkIte(c1.get(), bvTn.get(), bvTz.get(), result)
	c.mkIte(c2.get(), bvTp.get(), result.get(), result)
	c.mkIte(c3.get(), bvNta.get(), result.get(), result)
	c.mkIte(c4.get(), bvNte.get(), result.get(), result)
}

// mkToReal is mk_to_real(f, num, args, result).
func (c *FPA2BVConverter) mkToReal(fd ast.ASTHandle, args []ast.ASTHandle, result *eref) {
	x := args[0]
	s := c.sortOf(x)
	ebits, sbits := c.sortShape(s)
	f := c.frame()
	defer f.done()
	xIsNan, xIsInf, xIsZero := f.ref(), f.ref(), f.ref()
	c.mkIsNan(x, xIsNan)
	c.mkIsInf(x, xIsInf)
	c.mkIsZero(x, xIsZero)
	sgn, sig, exp, lz := f.ref(), f.ref(), f.ref(), f.ref()
	c.unpack(x, sgn, sig, exp, lz, true)
	rsig, bit, bitEq1, rsigMul2, zero, one, two, bv0, bv1 := f.ref(), f.ref(), f.ref(), f.ref(), f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
	zero.set(c.arithNum(new(big.Rat), false))
	one.set(c.arithNum(big.NewRat(1, 1), false))
	two.set(c.arithNum(big.NewRat(2, 1), false))
	bv0.set(c.bvNum(0, 1))
	bv1.set(c.bvNum(1, 1))
	rsig.set(one.get())
	for i := int64(sbits) - 2; i >= 0; i-- {
		bit.set(c.extract(uint32(i), uint32(i), sig.get()))
		bitEq1.set(c.mEq(bit.get(), bv1.get()))
		rsigMul2.set(c.arithMul(rsig.get(), two.get()))
		ite := c.mIte(bitEq1.get(), one.get(), zero.get())
		rsig.set(c.arithAdd(rsigMul2.get(), ite))
	}
	p2 := new(big.Int).Lsh(big.NewInt(1), uint(sbits-1))
	ep2 := f.ref()
	ep2.set(c.arithNum(new(big.Rat).SetInt(p2), false))
	rsig.set(c.arithDiv(rsig.get(), ep2.get()))
	expN, expP, expIsNeg, expAbs := f.ref(), f.ref(), f.ref(), f.ref()
	expIsNeg.set(c.mEq(c.extract(ebits-1, ebits-1, exp.get()), bv1.get()))
	expP.set(c.signExtend(1, exp.get()))
	expN.set(c.bvNeg(expP.get()))
	expAbs.set(c.mIte(expIsNeg.get(), expN.get(), expP.get()))
	exp2, exp2Mul2, prevBit := f.ref(), f.ref(), f.ref()
	exp2.set(zero.get())
	for i := int64(ebits); i >= 0; i-- {
		bit.set(c.extract(uint32(i), uint32(i), expAbs.get()))
		bitEq1.set(c.mEq(bit.get(), bv1.get()))
		exp2Mul2.set(c.arithMul(exp2.get(), two.get()))
		ite := c.mIte(bitEq1.get(), one.get(), zero.get())
		exp2.set(c.arithAdd(exp2Mul2.get(), ite))
		prevBit.set(bit.get())
	}
	oneDivExp2 := f.ref()
	oneDivExp2.set(c.arithDiv(one.get(), exp2.get()))
	exp2.set(c.mIte(expIsNeg.get(), oneDivExp2.get(), exp2.get()))
	res, twoExp2, minusRes, sgnIs1 := f.ref(), f.ref(), f.ref(), f.ref()
	twoExp2.set(c.arithPower(two.get(), exp2.get()))
	res.set(c.arithMul(rsig.get(), twoExp2.get()))
	minusRes.set(c.arithUminus(res.get()))
	sgnIs1.set(c.mEq(sgn.get(), bv1.get()))
	res.set(c.mIte(sgnIs1.get(), minusRes.get(), res.get()))
	unspec := f.ref()
	c.mkToRealUnspecified(fd, args, unspec)
	result.set(c.mIte(xIsZero.get(), zero.get(), res.get()))
	result.set(c.mIte(xIsInf.get(), unspec.get(), result.get()))
	result.set(c.mIte(xIsNan.get(), unspec.get(), result.get()))
}

// mkToFPSigned is mk_to_fp_signed(f, num, args, result).
func (c *FPA2BVConverter) mkToFPSigned(fd ast.ASTHandle, args []ast.ASTHandle, result *eref) {
	c.mkToFPBV(fd, args, result, true)
}

// mkToFPUnsigned is mk_to_fp_unsigned(f, num, args, result).
func (c *FPA2BVConverter) mkToFPUnsigned(fd ast.ASTHandle, args []ast.ASTHandle, result *eref) {
	c.mkToFPBV(fd, args, result, false)
}

// mkToFPBV holds mk_to_fp_signed and mk_to_fp_unsigned, which differ only in
// the sign handling of x and the declarations around it.
func (c *FPA2BVConverter) mkToFPBV(fd ast.ASTHandle, args []ast.ASTHandle, result *eref, signed bool) {
	f := c.frame()
	defer f.done()
	rm, x := f.ref(), f.ref()
	rm.set(c.rmArg(args[0]))
	x.set(args[1])
	s := c.rangeOf(fd)
	ebits, sbits := c.sortShape(s)
	bvSz := c.bvSize(x.get())
	bv01, bv11, bv0Sz, bv1Sz := f.ref(), f.ref(), f.ref(), f.ref()
	bv01.set(c.bvNum(0, 1))
	bv11.set(c.bvNum(1, 1))
	bv0Sz.set(c.bvNum(0, bvSz))
	bv1Sz.set(c.bvNum(1, bvSz))
	isZero, nzero, pzero, ninf, pinf := f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
	isZero.set(c.mEq(x.get(), bv0Sz.get()))
	c.mkNzeroSort(s, nzero)
	c.mkPzeroSort(s, pzero)
	c.mkNinfSort(s, ninf)
	c.mkPinfSort(s, pinf)
	c1, v1 := f.ref(), f.ref()
	c1.set(isZero.get())
	v1.set(pzero.get())
	var isNegBit, expTooLarge, sig4, exp2 *eref
	var magnitude ast.ASTHandle
	if signed {
		isNegBit, expTooLarge, sig4, exp2 = f.ref(), f.ref(), f.ref(), f.ref()
		isNeg, xAbs, negX := f.ref(), f.ref(), f.ref()
		isNegBit.set(c.extract(bvSz-1, bvSz-1, x.get()))
		isNeg.set(c.mEq(isNegBit.get(), bv11.get()))
		negX.set(c.bvNeg(x.get()))
		xAbs.set(c.mIte(isNeg.get(), negX.get(), x.get()))
		magnitude = xAbs.get()
	} else {
		expTooLarge, sig4, exp2 = f.ref(), f.ref(), f.ref()
		magnitude = x.get()
	}
	lz, eBvSz, eRestSz := f.ref(), f.ref(), f.ref()
	c.mkLeadingZeros(magnitude, bvSz, lz)
	eBvSz.set(c.bvNum(int64(bvSz), bvSz))
	eRestSz.set(c.bvSub(eBvSz.get(), lz.get()))
	shiftedSig := f.ref()
	shiftedSig.set(c.bvShl(magnitude, lz.get()))
	sticky := f.ref()
	sigSz := sbits + 4
	if sigSz <= bvSz {
		g := c.frame()
		sigRest := g.ref()
		sig4.set(c.extract(bvSz-1, bvSz-sigSz+1, shiftedSig.get()))
		sigRest.set(c.extract(bvSz-sigSz, 0, shiftedSig.get()))
		sticky.set(c.redor(sigRest.get()))
		sig4.set(c.concat(sig4.get(), sticky.get()))
		g.done()
	} else {
		extraBits := sigSz - bvSz
		g := c.frame()
		extraZeros := g.ref()
		extraZeros.set(c.bvNum(0, extraBits))
		sig4.set(c.concat(shiftedSig.get(), extraZeros.get()))
		n := c.bvNum(int64(extraBits), sigSz)
		lz.set(c.bvAdd(c.concat(extraZeros.get(), lz.get()), n))
		bvSz = bvSz + extraBits
		g.done()
	}
	sExp, expRest := f.ref(), f.ref()
	_ = expRest
	sExp.set(c.bvSub(c.bvNum(int64(bvSz-2), bvSz), lz.get()))
	expSz := ebits + 2
	exp2.set(c.extract(expSz-1, 0, sExp.get()))
	expTooLarge.set(c.mFalse())
	expWorstCaseSz := uint32((math.Log(float64(bvSz)) / math.Log(2)) + 1.0)
	if expSz < expWorstCaseSz {
		g := c.frame()
		maxExp, maxExpBvsz, zeroSigSz := g.ref(), g.ref(), g.ref()
		c.mkMaxExp(expSz, maxExp)
		maxExpBvsz.set(c.zeroExtend(bvSz-expSz, maxExp.get()))
		one := c.bvNum(1, bvSz)
		expTooLarge.set(c.sle(c.bvAdd(maxExpBvsz.get(), one), sExp.get()))
		zeroSigSz.set(c.bvNum(0, sigSz))
		sig4.set(c.mIte(expTooLarge.get(), zeroSigSz.get(), sig4.get()))
		exp2.set(c.mIte(expTooLarge.get(), maxExp.get(), exp2.get()))
		g.done()
	}
	sgn, sig, exp := f.ref(), f.ref(), f.ref()
	if signed {
		sgn.set(isNegBit.get())
	} else {
		sgn.set(bv01.get())
	}
	sig.set(sig4.get())
	exp.set(exp2.get())
	v2 := f.ref()
	c.round(s, rm, sgn, sig, exp, v2)
	c.mkIte(c1.get(), v1.get(), v2.get(), result)
}

// ---- to_ieee_bv ----

// mkToIEEEBV is mk_to_ieee_bv(f, num, args, result).
func (c *FPA2BVConverter) mkToIEEEBV(fd ast.ASTHandle, args []ast.ASTHandle, result *eref) {
	f := c.frame()
	defer f.done()
	x, xIsNan := f.ref(), f.ref()
	x.set(args[0])
	c.splitFP(x.get())
	c.mkIsNan(x.get(), xIsNan)
	unspec := f.ref()
	c.mkToIEEEBVUnspecified(fd, args, unspec)
	sgnES := f.ref()
	c.joinFP(x.get(), sgnES)
	c.simpIte(xIsNan.get(), unspec.get(), sgnES.get(), result)
}

// mkToIEEEBVUnspecified is mk_to_ieee_bv_unspecified(f, num, args, result).
func (c *FPA2BVConverter) mkToIEEEBVUnspecified(fd ast.ASTHandle, args []ast.ASTHandle, result *eref) {
	dom := c.must(c.m.GetDomain(fd, 0))
	ebits, sbits := c.sortShape(dom)
	if c.hiFPUnspecified {
		c.mkNanSort(dom, result)
		c.joinFP(result.get(), result)
		return
	}
	f := c.frame()
	defer f.done()
	nw := f.ref()
	c.nanWrap(args[0], nw)
	fBV := c.mkBVUF(fd, []ast.ASTHandle{c.sortOf(nw.get())}, c.rangeOf(fd))
	result.set(c.must(c.m.MkApp(fBV, []ast.ASTHandle{nw.get()})))
	expBV, expAllOnes := f.ref(), f.ref()
	expBV.set(c.extract(ebits+sbits-2, sbits-1, result.get()))
	expAllOnes.set(c.mEq(expBV.get(), c.bvNeg(c.bvNum(1, ebits))))
	c.pushExtraAssertion(expAllOnes.get())
	sigBV, sigIsNonZero := f.ref(), f.ref()
	sigBV.set(c.extract(sbits-2, 0, result.get()))
	sigIsNonZero.set(c.mNot(c.mEq(sigBV.get(), c.bvNum(0, sbits-1))))
	c.pushExtraAssertion(sigIsNonZero.get())
}

// ---- to_bv ----

// mkToBV is mk_to_bv(f, num, args, is_signed, result).
func (c *FPA2BVConverter) mkToBV(fd ast.ASTHandle, args []ast.ASTHandle, isSigned bool, result *eref) {
	rm := c.rmArg(args[0])
	x := args[1]
	xs := c.sortOf(x)
	bvSrt := c.rangeOf(fd)
	ebits, sbits := c.sortShape(xs)
	bvSz := c.bvSize(bvSrt)
	f := c.frame()
	defer f.done()
	bv0, bv1 := f.ref(), f.ref()
	bv0.set(c.bvNum(0, 1))
	bv1.set(c.bvNum(1, 1))
	xIsNan, xIsInf, xIsZero, xIsNeg, xIsNzero := f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
	c.mkIsNan(x, xIsNan)
	c.mkIsInf(x, xIsInf)
	c.mkIsZero(x, xIsZero)
	c.mkIsNeg(x, xIsNeg)
	c.mkIsNzero(x, xIsNzero)
	c1, v1, unspecV := f.ref(), f.ref(), f.ref()
	c1.set(c.mOr(xIsNan.get(), xIsInf.get()))
	c.mkToBVUnspecified(fd, args, unspecV)
	v1.set(unspecV.get())
	c2, v2 := f.ref(), f.ref()
	c2.set(xIsZero.get())
	v2.set(c.bvNum(0, bvSz))
	sgn, sig, exp, lz := f.ref(), f.ref(), f.ref(), f.ref()
	c.unpack(x, sgn, sig, exp, lz, true)
	sigSz := sbits
	if sigSz < bvSz+3 {
		sig.set(c.concat(sig.get(), c.bvNum(0, bvSz-sigSz+3)))
	}
	sigSz = c.bvSize(sig.get())
	expMLz, eMLzMBvSz, shift, isNegShift, bigSig := f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
	_ = eMLzMBvSz
	zl := c.zeroExtend(2, lz.get())
	expMLz.set(c.bvSub(c.signExtend(2, exp.get()), zl))
	bigSig.set(c.concat(c.zeroExtend(bvSz+2, sig.get()), bv0.get()))
	bigSigSz := sigSz + 1 + bvSz + 2
	isNegShift.set(c.sle(expMLz.get(), c.bvNum(0, ebits+2)))
	shift.set(c.mIte(isNegShift.get(), c.bvNeg(expMLz.get()), expMLz.get()))
	if ebits+2 < bigSigSz {
		shift.set(c.zeroExtend(bigSigSz-ebits-2, shift.get()))
	} else if ebits+2 > bigSigSz {
		g := c.frame()
		upper := g.ref()
		upper.set(c.extract(bigSigSz, ebits+2, shift.get()))
		shift.set(c.extract(ebits+1, 0, shift.get()))
		cap := c.bvNum(int64(bigSigSz-1), ebits+2)
		cond := c.mEq(upper.get(), c.bvNum(0, c.bvSize(upper.get())))
		shift.set(c.mIte(cond, shift.get(), cap))
		g.done()
	}
	shiftLimit := f.ref()
	shiftLimit.set(c.bvNum(int64(bvSz+2), c.bvSize(shift.get())))
	shift.set(c.mIte(c.ule(shift.get(), shiftLimit.get()), shift.get(), shiftLimit.get()))
	bigSigShifted, intPart, last, rnd, stickies, sticky := f.ref(), f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
	shl := c.bvShl(bigSig.get(), shift.get())
	lshr := c.bvLshr(bigSig.get(), shift.get())
	bigSigShifted.set(c.mIte(isNegShift.get(), lshr, shl))
	intPart.set(c.extract(bigSigSz-1, bigSigSz-(bvSz+3), bigSigShifted.get()))
	last.set(c.extract(bigSigSz-(bvSz+3), bigSigSz-(bvSz+3), bigSigShifted.get()))
	rnd.set(c.extract(bigSigSz-(bvSz+4), bigSigSz-(bvSz+4), bigSigShifted.get()))
	stickies.set(c.extract(bigSigSz-(bvSz+5), 0, bigSigShifted.get()))
	sticky.set(c.redor(stickies.get()))
	roundingDecision := f.ref()
	c.mkRoundingDecision(rm, sgn.get(), last.get(), rnd.get(), sticky.get(), roundingDecision)
	inc, preRounded := f.ref(), f.ref()
	inc.set(c.zeroExtend(bvSz+2, roundingDecision.get()))
	preRounded.set(c.bvAdd(intPart.get(), inc.get()))
	incd, prIsZero, ovfl := f.ref(), f.ref(), f.ref()
	incd.set(c.mEq(roundingDecision.get(), bv1.get()))
	prIsZero.set(c.mEq(preRounded.get(), c.bvNum(0, bvSz+3)))
	ovfl.set(c.mAnd(incd.get(), prIsZero.get()))
	ul, inRange := f.ref(), f.ref()
	if !isSigned {
		ul.set(c.zeroExtend(3, c.bvNeg(c.bvNum(1, bvSz))))
		le := c.ule(preRounded.get(), ul.get())
		nov := c.mNot(ovfl.get())
		eq := c.mEq(preRounded.get(), c.bvNum(0, bvSz+3))
		or := c.mOr(c.mNot(xIsNeg.get()), eq)
		inRange.set(c.mAnd(or, nov, le))
	} else {
		g := c.frame()
		ll := g.ref()
		ll.set(c.signExtend(3, c.concat(bv1.get(), c.bvNum(0, bvSz-1))))
		ul.set(c.zeroExtend(4, c.bvNeg(c.bvNum(1, bvSz-1))))
		ovfl.set(c.mOr(ovfl.get(), c.sle(preRounded.get(), c.bvNeg(c.bvNum(1, bvSz+3)))))
		hi := c.sle(preRounded.get(), ul.get())
		lo := c.sle(ll.get(), preRounded.get())
		inRange.set(c.mAnd(c.mNot(ovfl.get()), lo, hi))
		preRounded.set(c.mIte(xIsNeg.get(), c.bvNeg(preRounded.get()), preRounded.get()))
		g.done()
	}
	rounded := f.ref()
	rounded.set(c.extract(bvSz-1, 0, preRounded.get()))
	result.set(c.mIte(c.mNot(inRange.get()), unspecV.get(), rounded.get()))
	result.set(c.mIte(c2.get(), v2.get(), result.get()))
	result.set(c.mIte(c1.get(), v1.get(), result.get()))
}

// mkToUBV is mk_to_ubv.
func (c *FPA2BVConverter) mkToUBV(fd ast.ASTHandle, args []ast.ASTHandle, result *eref) {
	c.mkToBV(fd, args, false, result)
}

// mkToSBV is mk_to_sbv.
func (c *FPA2BVConverter) mkToSBV(fd ast.ASTHandle, args []ast.ASTHandle, result *eref) {
	c.mkToBV(fd, args, true, result)
}

// nanWrap is nan_wrap(n); res is the returned expr_ref.
func (c *FPA2BVConverter) nanWrap(n ast.ASTHandle, res *eref) {
	f := c.frame()
	defer f.done()
	nBV, argIsNan, nan, nanBV := f.ref(), f.ref(), f.ref(), f.ref()
	c.mkIsNan(n, argIsNan)
	c.mkNanSort(c.sortOf(n), nan)
	c.joinFP(nan.get(), nanBV)
	c.joinFP(n, nBV)
	res.set(c.mIte(argIsNan.get(), nanBV.get(), nBV.get()))
}

// mkToBVUnspecified is mk_to_bv_unspecified(f, num, args, result).
func (c *FPA2BVConverter) mkToBVUnspecified(fd ast.ASTHandle, args []ast.ASTHandle, result *eref) {
	if c.hiFPUnspecified {
		result.set(c.bvNum(0, c.bvSize(c.rangeOf(fd))))
		return
	}
	rmBV := c.rmArg(args[0])
	f := c.frame()
	defer f.done()
	nw := f.ref()
	c.nanWrap(args[1], nw)
	domain := []ast.ASTHandle{c.sortOf(rmBV), c.sortOf(nw.get())}
	fBV := c.mkBVUF(fd, domain, c.rangeOf(fd))
	result.set(c.must(c.m.MkApp(fBV, []ast.ASTHandle{rmBV, nw.get()})))
}

// mkToRealUnspecified is mk_to_real_unspecified(f, num, args, result).
func (c *FPA2BVConverter) mkToRealUnspecified(fd ast.ASTHandle, args []ast.ASTHandle, result *eref) {
	if c.hiFPUnspecified {
		result.set(c.arithNum(new(big.Rat), false))
		return
	}
	f := c.frame()
	defer f.done()
	nw := f.ref()
	c.nanWrap(args[0], nw)
	fBV := c.mkBVUF(fd, []ast.ASTHandle{c.sortOf(nw.get())}, c.rangeOf(fd))
	result.set(c.must(c.m.MkApp(fBV, []ast.ASTHandle{nw.get()})))
}
