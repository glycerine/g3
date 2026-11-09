package fpa

// Source: src/ast/fpa/fpa2bv_converter.cpp:1693-2271 (mk_sqrt,
// mk_round_to_integral, mk_float_eq/lt/gt/le/ge, mk_is_* decl forms).

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// mkSqrt is mk_sqrt(f, num, args, result).
func (c *FPA2BVConverter) mkSqrt(fd ast.ASTHandle, args []ast.ASTHandle, result *eref) {
	f := c.frame()
	defer f.done()
	rm, x := f.ref(), f.ref()
	rm.set(c.rmArg(args[0]))
	x.set(args[1])
	s := c.rangeOf(fd)
	nan, nzero, pzero, ninf, pinf := f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
	c.mkNanSort(s, nan)
	c.mkNzeroSort(s, nzero)
	c.mkPzeroSort(s, pzero)
	c.mkNinfSort(s, ninf)
	c.mkPinfSort(s, pinf)
	xIsNan, xIsZero, xIsPos, xIsInf := f.ref(), f.ref(), f.ref(), f.ref()
	c.mkIsNan(x.get(), xIsNan)
	c.mkIsZero(x.get(), xIsZero)
	c.mkIsPos(x.get(), xIsPos)
	c.mkIsInf(x.get(), xIsInf)
	zero1, one1 := f.ref(), f.ref()
	zero1.set(c.bvNum(0, 1))
	one1.set(c.bvNum(1, 1))
	c1, c2, c3, c4, c5, c6 := f.ref(), f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
	_, _ = c5, c6
	v1, v2, v3, v4, v5, v6, v7 := f.ref(), f.ref(), f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
	_, _ = v6, v7
	c1.set(xIsNan.get())
	v1.set(x.get())
	c.mkIsPinf(x.get(), c2)
	v2.set(x.get())
	c.mkIsZero(x.get(), c3)
	v3.set(x.get())
	c.mkIsNeg(x.get(), c4)
	v4.set(nan.get())
	ebits, sbits := c.sortShape(s)
	aSgn, aSig, aExp, aLz := f.ref(), f.ref(), f.ref(), f.ref()
	c.unpack(x.get(), aSgn, aSig, aExp, aLz, true)
	resSgn, resSig, resExp := f.ref(), f.ref(), f.ref()
	resSgn.set(zero1.get())
	realExp := f.ref()
	zl := c.zeroExtend(1, aLz.get())
	realExp.set(c.bvSub(c.signExtend(1, aExp.get()), zl))
	resExp.set(c.signExtend(2, c.extract(ebits, 1, realExp.get())))
	eIsOdd := f.ref()
	eIsOdd.set(c.mEq(c.extract(0, 0, realExp.get()), one1.get()))
	sigPrime, aZ, zA := f.ref(), f.ref(), f.ref()
	aZ.set(c.concat(aSig.get(), zero1.get()))
	zA.set(c.concat(zero1.get(), aSig.get()))
	c.simpIte(eIsOdd.get(), aZ.get(), zA.get(), sigPrime)
	Q, R, S, T := f.ref(), f.ref(), f.ref(), f.ref()
	p2 := new(big.Int).Lsh(big.NewInt(1), uint(sbits+3))
	Q.set(c.bvNumBig(p2, sbits+5))
	R.set(c.bvSub(c.concat(sigPrime.get(), c.bvNum(0, 4)), Q.get()))
	S.set(Q.get())
	for i := uint32(0); i < sbits+3; i++ {
		g := c.frame()
		S.set(c.concat(zero1.get(), c.extract(sbits+4, 1, S.get())))
		twoQPlusS := g.ref()
		right := c.concat(zero1.get(), S.get())
		left := c.concat(Q.get(), zero1.get())
		twoQPlusS.set(c.bvAdd(left, right))
		T.set(c.bvSub(c.concat(R.get(), zero1.get()), twoQPlusS.get()))
		tLt0, tLsds5 := g.ref(), g.ref()
		tLsds5.set(c.extract(sbits+5, sbits+5, T.get()))
		c.simpEq(tLsds5.get(), one1.get(), tLt0)
		qOrS, rShftd, tLsds4 := g.ref(), g.ref(), g.ref()
		qOrS.set(c.bvOr(Q.get(), S.get()))
		c.simpIte(tLt0.get(), Q.get(), qOrS.get(), Q)
		rShftd.set(c.concat(c.extract(sbits+3, 0, R.get()), zero1.get()))
		tLsds4.set(c.extract(sbits+4, 0, T.get()))
		c.simpIte(tLt0.get(), rShftd.get(), tLsds4.get(), R)
		g.done()
	}
	isExact, zeroSbits5 := f.ref(), f.ref()
	zeroSbits5.set(c.bvNum(0, sbits+5))
	c.simpEq(R.get(), zeroSbits5.get(), isExact)
	rest, last, qIsOdd, restExt := f.ref(), f.ref(), f.ref(), f.ref()
	_ = qIsOdd
	last.set(c.extract(0, 0, Q.get()))
	rest.set(c.extract(sbits+3, 1, Q.get()))
	restExt.set(c.zeroExtend(1, rest.get()))
	sticky, lastExt, oneSbits4 := f.ref(), f.ref(), f.ref()
	lastExt.set(c.zeroExtend(sbits+3, last.get()))
	oneSbits4.set(c.bvNum(1, sbits+4))
	c.simpIte(isExact.get(), lastExt.get(), oneSbits4.get(), sticky)
	resSig.set(c.bvOr(restExt.get(), sticky.get()))
	rounded := f.ref()
	c.round(s, rm, resSgn, resSig, resExp, rounded)
	v5.set(rounded.get())
	c.mkIte(c4.get(), v4.get(), v5.get(), result)
	c.mkIte(c3.get(), v3.get(), result.get(), result)
	c.mkIte(c2.get(), v2.get(), result.get(), result)
	c.mkIte(c1.get(), v1.get(), result.get(), result)
}

// mkRoundToIntegralDecl is mk_round_to_integral(f, num, args, result).
func (c *FPA2BVConverter) mkRoundToIntegralDecl(fd ast.ASTHandle, args []ast.ASTHandle, result *eref) {
	g := c.frame()
	defer g.done()
	rm, x := g.ref(), g.ref()
	rm.set(c.rmArg(args[0]))
	x.set(args[1])
	c.mkRoundToIntegral(c.rangeOf(fd), rm, x, result)
}

// mkRoundToIntegral is mk_round_to_integral(s, rm, x, result).
func (c *FPA2BVConverter) mkRoundToIntegral(s ast.ASTHandle, rm, x, result *eref) {
	f := c.frame()
	defer f.done()
	rmIsRta, rmIsRte, rmIsRtp, rmIsRtn, rmIsRtz := f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
	c.mkIsRM(rm.get(), bvRMTiesToAway, rmIsRta)
	c.mkIsRM(rm.get(), bvRMTiesToEven, rmIsRte)
	c.mkIsRM(rm.get(), bvRMToPositive, rmIsRtp)
	c.mkIsRM(rm.get(), bvRMToNegative, rmIsRtn)
	c.mkIsRM(rm.get(), bvRMToZero, rmIsRtz)
	nan, nzero, pzero, ninf, pinf := f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
	_, _ = ninf, pinf
	c.mkNanSort(s, nan)
	c.mkNzeroSort(s, nzero)
	c.mkPzeroSort(s, pzero)
	xIsZero, xIsPos, xIsNeg := f.ref(), f.ref(), f.ref()
	c.mkIsZero(x.get(), xIsZero)
	c.mkIsPos(x.get(), xIsPos)
	c.mkIsNeg(x.get(), xIsNeg)
	c1, c2, c3, c4, c5 := f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
	v1, v2, v3, v4, v5, v6 := f.ref(), f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
	c.mkIsNan(x.get(), c1)
	v1.set(nan.get())
	c.mkIsInf(x.get(), c2)
	v2.set(x.get())
	c.mkIsZero(x.get(), c3)
	v3.set(x.get())
	one1, zero1 := f.ref(), f.ref()
	one1.set(c.bvNum(1, 1))
	zero1.set(c.bvNum(0, 1))
	ebits, sbits := c.sortShape(s)
	aSgn, aSig, aExp, aLz := f.ref(), f.ref(), f.ref(), f.ref()
	c.unpack(x.get(), aSgn, aSig, aExp, aLz, true)
	xzero, sgnEq1 := f.ref(), f.ref()
	sgnEq1.set(c.mEq(aSgn.get(), one1.get()))
	c.mkIte(sgnEq1.get(), nzero.get(), pzero.get(), xzero)
	expLtZero, expH := f.ref(), f.ref()
	expH.set(c.extract(ebits-1, ebits-1, aExp.get()))
	c.simpEq(expH.get(), one1.get(), expLtZero)
	c4.set(expLtZero.get())
	pone, none, xone, c421, c422, c423, t1, t2, tie, v42, expLtM1 := f.ref(), f.ref(), f.ref(), f.ref(), f.ref(), f.ref(), f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
	_ = expLtM1
	c.mkOne(s, zero1.get(), pone)
	c.mkOne(s, one1.get(), none)
	c.mkIte(sgnEq1.get(), none.get(), pone.get(), xone)
	pow2Sbitsm1, m1 := f.ref(), f.ref()
	pow2Sbitsm1.set(c.bvNumBig(new(big.Int).Lsh(big.NewInt(1), uint(sbits-1)), sbits))
	m1.set(c.bvNeg(c.bvNum(1, ebits)))
	c.simpEq(aSig.get(), pow2Sbitsm1.get(), t1)
	c.simpEq(aExp.get(), m1.get(), t2)
	c.simpAnd(tie, t1.get(), t2.get())
	c.simpAnd(c421, tie.get(), rmIsRte.get())
	c.simpAnd(c422, tie.get(), rmIsRta.get())
	c423.set(c.sle(aExp.get(), c.bvNeg(c.bvNum(2, ebits))))
	v42.set(xone.get())
	c.mkIte(c423.get(), xzero.get(), v42.get(), v42)
	c.mkIte(c422.get(), xone.get(), v42.get(), v42)
	c.mkIte(c421.get(), xzero.get(), v42.get(), v42)
	v4Rtn, v4Rtp := f.ref(), f.ref()
	c.mkIte(xIsNeg.get(), nzero.get(), pone.get(), v4Rtp)
	c.mkIte(xIsNeg.get(), none.get(), pzero.get(), v4Rtn)
	c.mkIte(rmIsRtp.get(), v4Rtp.get(), v42.get(), v4)
	c.mkIte(rmIsRtn.get(), v4Rtn.get(), v4.get(), v4)
	c.mkIte(rmIsRtz.get(), xzero.get(), v4.get(), v4)
	expIsLarge := f.ref()
	expIsLarge.set(c.sle(c.bvNum(int64(sbits-1), ebits), aExp.get()))
	c5.set(expIsLarge.get())
	v5.set(x.get())
	resSgn, resSig, resExp := f.ref(), f.ref(), f.ref()
	resSgn.set(aSgn.get())
	resExp.set(aExp.get())
	zeroS := f.ref()
	zeroS.set(c.bvNum(0, sbits))
	shift, shiftedSig, div, rem := f.ref(), f.ref(), f.ref(), f.ref()
	ze := c.zeroExtend(sbits-ebits, aExp.get())
	shift.set(c.bvSub(c.bvNum(int64(sbits-1), sbits), ze))
	rc := c.concat(zeroS.get(), shift.get())
	shiftedSig.set(c.bvLshr(c.concat(aSig.get(), zeroS.get()), rc))
	div.set(c.extract(2*sbits-1, sbits, shiftedSig.get()))
	rem.set(c.extract(sbits-1, 0, shiftedSig.get()))
	divP1 := f.ref()
	divP1.set(c.bvAdd(div.get(), c.bvNum(1, sbits)))
	tiePttrn, tie2, tie2C, divLast, v51 := f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
	tiePttrn.set(c.concat(one1.get(), c.bvNum(0, sbits-1)))
	c.simpEq(rem.get(), tiePttrn.get(), tie2)
	divLast.set(c.extract(0, 0, div.get()))
	divLastEq1, rteAndDlEq1, rteAndDlEq1OrRta, tiePttrnUleRem := f.ref(), f.ref(), f.ref(), f.ref()
	divLastEq1.set(c.mEq(divLast.get(), one1.get()))
	rteAndDlEq1.set(c.mAnd(rmIsRte.get(), divLastEq1.get()))
	rteAndDlEq1OrRta.set(c.mOr(rteAndDlEq1.get(), rmIsRta.get()))
	tiePttrnUleRem.set(c.ule(tiePttrn.get(), rem.get()))
	tie2C.set(c.mIte(tie2.get(), rteAndDlEq1OrRta.get(), tiePttrnUleRem.get()))
	c.simpIte(tie2C.get(), divP1.get(), div.get(), v51)
	c521, v52, remEq0, sgnEqZero := f.ref(), f.ref(), f.ref(), f.ref()
	remEq0.set(c.mEq(rem.get(), c.bvNum(0, sbits)))
	sgnEqZero.set(c.mEq(resSgn.get(), zero1.get()))
	c.simpNot(remEq0.get(), c521)
	c.simpAnd(c521, c521.get(), sgnEqZero.get())
	c.simpIte(c521.get(), divP1.get(), div.get(), v52)
	c531, v53, sgnEqOne := f.ref(), f.ref(), f.ref()
	sgnEqOne.set(c.mEq(resSgn.get(), one1.get()))
	c.simpNot(remEq0.get(), c531)
	c.simpAnd(c531, c531.get(), sgnEqOne.get())
	c.simpIte(c531.get(), divP1.get(), div.get(), v53)
	c51, c52, c53 := f.ref(), f.ref(), f.ref()
	c51.set(c.mOr(rmIsRte.get(), rmIsRta.get()))
	c52.set(rmIsRtp.get())
	c53.set(rmIsRtn.get())
	resSig.set(div.get())
	c.simpIte(c53.get(), v53.get(), resSig.get(), resSig)
	c.simpIte(c52.get(), v52.get(), resSig.get(), resSig)
	c.simpIte(c51.get(), v51.get(), resSig.get(), resSig)
	resSig.set(c.zeroExtend(1, c.concat(resSig.get(), c.bvNum(0, 3))))
	eShift := f.ref()
	if ebits+2 <= sbits+1 {
		eShift.set(c.extract(ebits+1, 0, shift.get()))
	} else {
		eShift.set(c.signExtend((ebits+2)-sbits, shift.get()))
	}
	resExp.set(c.bvAdd(c.zeroExtend(2, resExp.get()), eShift.get()))
	c.round(s, rm, resSgn, resSig, resExp, v6)
	c.mkIte(c5.get(), v5.get(), v6.get(), result)
	c.mkIte(c4.get(), v4.get(), result.get(), result)
	c.mkIte(c3.get(), v3.get(), result.get(), result)
	c.mkIte(c2.get(), v2.get(), result.get(), result)
	c.mkIte(c1.get(), v1.get(), result.get(), result)
}

// ---- comparisons ----

// binaryDecl runs the (x, y) expr_ref prologue of the decl forms.
func (c *FPA2BVConverter) binaryDecl(fd ast.ASTHandle, args []ast.ASTHandle, result *eref, op func(s ast.ASTHandle, x, y, result *eref)) {
	g := c.frame()
	defer g.done()
	x, y := g.ref(), g.ref()
	x.set(args[0])
	y.set(args[1])
	op(c.rangeOf(fd), x, y, result)
}

func (c *FPA2BVConverter) mkFloatEqDecl(fd ast.ASTHandle, args []ast.ASTHandle, result *eref) {
	c.binaryDecl(fd, args, result, c.mkFloatEq)
}
func (c *FPA2BVConverter) mkFloatLtDecl(fd ast.ASTHandle, args []ast.ASTHandle, result *eref) {
	c.binaryDecl(fd, args, result, c.mkFloatLt)
}
func (c *FPA2BVConverter) mkFloatGtDecl(fd ast.ASTHandle, args []ast.ASTHandle, result *eref) {
	c.binaryDecl(fd, args, result, c.mkFloatGt)
}
func (c *FPA2BVConverter) mkFloatLeDecl(fd ast.ASTHandle, args []ast.ASTHandle, result *eref) {
	c.binaryDecl(fd, args, result, c.mkFloatLe)
}
func (c *FPA2BVConverter) mkFloatGeDecl(fd ast.ASTHandle, args []ast.ASTHandle, result *eref) {
	c.binaryDecl(fd, args, result, c.mkFloatGe)
}

// mkFloatEq is mk_float_eq(s, x, y, result).
func (c *FPA2BVConverter) mkFloatEq(s ast.ASTHandle, x, y, result *eref) {
	f := c.frame()
	defer f.done()
	c1, c2, xIsNan, yIsNan, xIsZero, yIsZero := f.ref(), f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
	c.mkIsNan(x.get(), xIsNan)
	c.mkIsNan(y.get(), yIsNan)
	c.simpOr(c1, xIsNan.get(), yIsNan.get())
	c.mkIsZero(x.get(), xIsZero)
	c.mkIsZero(y.get(), yIsZero)
	c.simpAnd(c2, xIsZero.get(), yIsZero.get())
	xSgn, xExp, xSig := c.splitFP(x.get())
	ySgn, yExp, ySig := c.splitFP(y.get())
	xEqYSgn, xEqYExp, xEqYSig := f.ref(), f.ref(), f.ref()
	c.simpEq(xSgn, ySgn, xEqYSgn)
	c.simpEq(xExp, yExp, xEqYExp)
	c.simpEq(xSig, ySig, xEqYSig)
	c3, t4 := f.ref(), f.ref()
	c.simpNot(xEqYSgn.get(), c3)
	c.simpAnd(t4, xEqYExp.get(), xEqYSig.get())
	c3t4, c2else := f.ref(), f.ref()
	c.simpIte(c3.get(), c.mFalse(), t4.get(), c3t4)
	c.simpIte(c2.get(), c.mTrue(), c3t4.get(), c2else)
	c.simpIte(c1.get(), c.mFalse(), c2else.get(), result)
}

// mkFloatLt is mk_float_lt(s, x, y, result).
func (c *FPA2BVConverter) mkFloatLt(s ast.ASTHandle, x, y, result *eref) {
	f := c.frame()
	defer f.done()
	c1, c2, xIsNan, yIsNan, xIsZero, yIsZero := f.ref(), f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
	c.mkIsNan(x.get(), xIsNan)
	c.mkIsNan(y.get(), yIsNan)
	c.simpOr(c1, xIsNan.get(), yIsNan.get())
	c.mkIsZero(x.get(), xIsZero)
	c.mkIsZero(y.get(), yIsZero)
	c.simpAnd(c2, xIsZero.get(), yIsZero.get())
	xSgn, xExp, xSig := c.splitFP(x.get())
	ySgn, yExp, ySig := c.splitFP(y.get())
	c3, t3, t4, one1, nil1 := f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
	one1.set(c.bvNum(1, 1))
	nil1.set(c.bvNum(0, 1))
	c.simpEq(xSgn, one1.get(), c3)
	ySgnEq0, yLtXExp, yLtXSig, yEqXExp, yLeXSigExp, t3Or := f.ref(), f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
	c.simpEq(ySgn, nil1.get(), ySgnEq0)
	c.bvult(yExp, xExp, yLtXExp)
	c.bvult(ySig, xSig, yLtXSig)
	c.simpEq(yExp, xExp, yEqXExp)
	c.simpAnd(yLeXSigExp, yEqXExp.get(), yLtXSig.get())
	c.simpOr(t3Or, yLtXExp.get(), yLeXSigExp.get())
	c.simpIte(ySgnEq0.get(), c.mTrue(), t3Or.get(), t3)
	ySgnEq1, xLtYExp, xEqYExp, xLtYSig, xLeYSigExp, t4Or := f.ref(), f.ref(), f.ref(), f.ref(), f.ref(), f.ref()
	c.simpEq(ySgn, one1.get(), ySgnEq1)
	c.bvult(xExp, yExp, xLtYExp)
	c.simpEq(xExp, yExp, xEqYExp)
	c.bvult(xSig, ySig, xLtYSig)
	c.simpAnd(xLeYSigExp, xEqYExp.get(), xLtYSig.get())
	c.simpOr(t4Or, xLtYExp.get(), xLeYSigExp.get())
	c.simpIte(ySgnEq1.get(), c.mFalse(), t4Or.get(), t4)
	c3t3t4, c2else := f.ref(), f.ref()
	c.simpIte(c3.get(), t3.get(), t4.get(), c3t3t4)
	c.simpIte(c2.get(), c.mFalse(), c3t3t4.get(), c2else)
	c.simpIte(c1.get(), c.mFalse(), c2else.get(), result)
}

// mkFloatGt is mk_float_gt(s, x, y, result).
func (c *FPA2BVConverter) mkFloatGt(s ast.ASTHandle, x, y, result *eref) {
	f := c.frame()
	defer f.done()
	t3 := f.ref()
	c.mkFloatLe(s, x, y, t3)
	nanOr, xyZero, notT3, rElse := f.ref(), f.ref(), f.ref(), f.ref()
	xIsNan, yIsNan, xIsZero, yIsZero := f.ref(), f.ref(), f.ref(), f.ref()
	c.mkIsNan(x.get(), xIsNan)
	c.mkIsNan(y.get(), yIsNan)
	c.simpOr(nanOr, xIsNan.get(), yIsNan.get())
	c.mkIsZero(x.get(), xIsZero)
	c.mkIsZero(y.get(), yIsZero)
	c.simpAnd(xyZero, xIsZero.get(), yIsZero.get())
	c.simpNot(t3.get(), notT3)
	c.simpIte(xyZero.get(), c.mFalse(), notT3.get(), rElse)
	c.simpIte(nanOr.get(), c.mFalse(), rElse.get(), result)
}

// mkFloatLe is mk_float_le(s, x, y, result).
func (c *FPA2BVConverter) mkFloatLe(s ast.ASTHandle, x, y, result *eref) {
	f := c.frame()
	defer f.done()
	a, b := f.ref(), f.ref()
	c.mkFloatLt(s, x, y, a)
	c.mkFloatEq(s, x, y, b)
	c.simpOr(result, a.get(), b.get())
}

// mkFloatGe is mk_float_ge(s, x, y, result).
func (c *FPA2BVConverter) mkFloatGe(s ast.ASTHandle, x, y, result *eref) {
	f := c.frame()
	defer f.done()
	a, b := f.ref(), f.ref()
	c.mkFloatGt(s, x, y, a)
	c.mkFloatEq(s, x, y, b)
	c.simpOr(result, a.get(), b.get())
}

// ---- classification decl forms ----

func (c *FPA2BVConverter) mkIsZeroDecl(args []ast.ASTHandle, result *eref) {
	c.mkIsZero(args[0], result)
}

func (c *FPA2BVConverter) mkIsNzeroDecl(args []ast.ASTHandle, result *eref) {
	f := c.frame()
	defer f.done()
	a0IsNeg, a0IsZero := f.ref(), f.ref()
	c.mkIsNeg(args[0], a0IsNeg)
	c.mkIsZero(args[0], a0IsZero)
	c.simpAnd(result, a0IsNeg.get(), a0IsZero.get())
}

func (c *FPA2BVConverter) mkIsPzeroDecl(args []ast.ASTHandle, result *eref) {
	f := c.frame()
	defer f.done()
	a0IsPos, a0IsZero := f.ref(), f.ref()
	c.mkIsPos(args[0], a0IsPos)
	c.mkIsZero(args[0], a0IsZero)
	c.simpAnd(result, a0IsPos.get(), a0IsZero.get())
}

func (c *FPA2BVConverter) mkIsNanDecl(args []ast.ASTHandle, result *eref) {
	c.mkIsNan(args[0], result)
}
func (c *FPA2BVConverter) mkIsInfDecl(args []ast.ASTHandle, result *eref) {
	c.mkIsInf(args[0], result)
}
func (c *FPA2BVConverter) mkIsNormalDecl(args []ast.ASTHandle, result *eref) {
	c.mkIsNormal(args[0], result)
}
func (c *FPA2BVConverter) mkIsSubnormalDecl(args []ast.ASTHandle, result *eref) {
	c.mkIsDenormal(args[0], result)
}

func (c *FPA2BVConverter) mkIsNegativeDecl(args []ast.ASTHandle, result *eref) {
	f := c.frame()
	defer f.done()
	t1, t2, nt1 := f.ref(), f.ref(), f.ref()
	c.mkIsNan(args[0], t1)
	c.mkIsNeg(args[0], t2)
	nt1.set(c.mNot(t1.get()))
	result.set(c.mAnd(nt1.get(), t2.get()))
}

func (c *FPA2BVConverter) mkIsPositiveDecl(args []ast.ASTHandle, result *eref) {
	f := c.frame()
	defer f.done()
	t1, t2, nt1 := f.ref(), f.ref(), f.ref()
	c.mkIsNan(args[0], t1)
	c.mkIsPos(args[0], t2)
	nt1.set(c.mNot(t1.get()))
	result.set(c.mAnd(nt1.get(), t2.get()))
}
