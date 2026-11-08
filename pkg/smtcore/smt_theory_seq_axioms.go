package smtcore

// Source: src/smt/theory_seq.cpp:3137-3933, tightest_prefix through
// propagate_eq(deps, lits, e1, e2, add_to_eqs), except propagate_in_re,
// which is with the automata (smt_theory_seq_automata.go).

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// tightestPrefix encodes that s is not contained in x s1, where s1 is all
// of s except the last element:
//
//	s = "" or s = s1*(unit c)
//	s = "" or !contains(x*s1, s)
func (t *SMTTheorySeq) tightestPrefix(s, x ast.ASTHandle) {
	y := t.loc()
	defer y.drop()
	s1 := t.ref(y, t.mkFirst(s), nil)
	c := t.ref(y, t.mkLast(s), nil)
	s1c := t.ref(y, t.mkConcat2(s1, t.mkUnit(c)), nil)
	sEqEmp := t.mkEqEmpty(s, true)
	t.addAxiom(sEqEmp, t.mkSeqEq(s, s1c))
	t.addAxiom(sEqEmp, t.mkLiteral(t.must(t.m.MkSequenceContains(t.mkConcat2(x, s1), s))).Negated())
}

// addIndexofAxiom: see the specification comment at theory_seq.cpp:3146.
func (t *SMTTheorySeq) addIndexofAxiom(i ast.ASTHandle) {
	tt, s := t.arg(i, 0), t.arg(i, 1)
	var offset ast.ASTHandle
	if t.numArgs(i) == 3 {
		offset = t.arg(i, 2)
	}
	x0 := t.loc()
	defer x0.drop()
	minusOne := t.ref(x0, t.mkInt(-1), nil)
	zero := t.ref(x0, t.mkInt(0), nil)

	cnt := t.mkLiteral(t.must(t.m.MkSequenceContains(tt, s)))
	iEqM1 := t.mkEq(i, minusOne, false)
	iEq0 := t.mkEq(i, zero, false)
	sEqEmpty := t.mkEqEmpty(s, true)
	tEqEmpty := t.mkEqEmpty(tt, true)

	// |t| = 0 => |s| = 0 or indexof(t,s,offset) = -1
	// ~contains(t,s) <=> indexof(t,s,offset) = -1
	t.addAxiom(cnt, iEqM1)
	t.addAxiom(tEqEmpty.Negated(), sEqEmpty, iEqM1)

	r, isNum := ast.ASTHandle{}, false
	if !offset.IsNull() {
		v, ok := t.isNumeral(offset)
		isNum = ok && v.Sign() == 0
		_ = r
	}
	if offset.IsNull() || isNum {
		x := t.ref(x0, t.mkSkolem(t.indexofLeft, tt, s, ast.ASTHandle{}, ast.ASTHandle{}), nil)
		y := t.ref(x0, t.mkSkolem(t.indexofRight, tt, s, ast.ASTHandle{}, ast.ASTHandle{}), nil)
		xsy := t.ref(x0, t.mkConcat3(x, s, y), nil)
		lenx := t.ref(x0, t.mkLength(x), nil)
		// |s| = 0 => indexof(t,s,0) = 0
		// contains(t,s) & |s| != 0 => t = xsy & indexof(t,s,0) = |x|
		t.addAxiom(sEqEmpty.Negated(), iEq0)
		t.addAxiom(cnt.Negated(), sEqEmpty, t.mkSeqEq(tt, xsy))
		t.addAxiom(cnt.Negated(), sEqEmpty, t.mkEq(i, lenx, false))
		t.addAxiom(cnt.Negated(), t.mkLiteral(t.mkGe(i, zero)))
		t.tightestPrefix(s, x)
		return
	}
	// offset >= len(t) => |s| = 0 or indexof(t, s, offset) = -1
	// offset > len(t) => indexof(t, s, offset) = -1
	// offset = len(t) & |s| = 0 => indexof(t, s, offset) = offset
	lenT := t.ref(x0, t.mkLength(tt), nil)
	offsetGeLen := t.mkSimplifiedLiteral(t.mkGe(t.mkSubExpr(offset, lenT), zero))
	offsetLeLen := t.mkSimplifiedLiteral(t.mkLe(t.mkSubExpr(offset, lenT), zero))
	iEqOffset := t.mkEq(i, offset, false)
	t.addAxiom(offsetGeLen.Negated(), sEqEmpty, iEqM1)
	t.addAxiom(offsetLeLen, iEqM1)
	t.addAxiom(offsetGeLen.Negated(), offsetLeLen.Negated(), sEqEmpty.Negated(), iEqOffset)

	x := t.ref(x0, t.mkSkolem(t.indexofLeft, tt, s, offset, ast.ASTHandle{}), nil)
	y := t.ref(x0, t.mkSkolem(t.indexofRight, tt, s, offset, ast.ASTHandle{}), nil)
	indexof0 := t.ref(x0, t.must(t.m.MkSequenceIndex(y, s, zero)), nil)
	offsetPIndexof0 := t.ref(x0, t.mkAdd(offset, indexof0), nil)
	offsetGe0 := t.mkSimplifiedLiteral(t.mkGe(offset, zero))

	t.addAxiom(offsetGe0.Negated(), offsetGeLen, t.mkSeqEq(tt, t.mkConcat2(x, y)))
	t.addAxiom(offsetGe0.Negated(), offsetGeLen, t.mkEq(t.mkLength(x), offset, false))
	t.addAxiom(offsetGe0.Negated(), offsetGeLen, t.mkEq(indexof0, minusOne, false).Negated(), iEqM1)
	t.addAxiom(offsetGe0.Negated(), offsetGeLen,
		t.mkSimplifiedLiteral(t.mkGe(indexof0, zero)).Negated(),
		t.mkEq(offsetPIndexof0, i, false))
	// offset < 0 => -1 = i
	t.addAxiom(offsetGe0, iEqM1)
}

// addReplaceAxiom, for r = replace(a, s, t):
//
//	a = "" => s = "" or r = a
//	contains(a, s) or r = a
//	s = "" => r = t+a
//	tightest_prefix(s, x)
//	(contains(a, s) -> r = xty & a = xsy) & (!contains(a, s) -> r = a)
func (t *SMTTheorySeq) addReplaceAxiom(r ast.ASTHandle) {
	a, s, tt := t.arg(r, 0), t.arg(r, 1), t.arg(r, 2)
	x0 := t.loc()
	defer x0.drop()
	x := t.ref(x0, t.mkSkolem(t.indexofLeft, a, s, ast.ASTHandle{}, ast.ASTHandle{}), nil)
	y := t.ref(x0, t.mkSkolem(t.indexofRight, a, s, ast.ASTHandle{}, ast.ASTHandle{}), nil)
	xty := t.ref(x0, t.mkConcat3(x, tt, y), nil)
	xsy := t.ref(x0, t.mkConcat3(x, s, y), nil)
	aEmp := t.mkEqEmpty(a, true)
	sEmp := t.mkEqEmpty(s, true)
	cnt := t.mkLiteral(t.must(t.m.MkSequenceContains(a, s)))
	t.addAxiom(aEmp.Negated(), sEmp, t.mkSeqEq(r, a))
	t.addAxiom(cnt, t.mkSeqEq(r, a))
	t.addAxiom(sEmp.Negated(), t.mkSeqEq(r, t.mkConcat2(tt, a)))
	t.addAxiom(cnt.Negated(), aEmp, sEmp, t.mkSeqEq(a, xsy))
	t.addAxiom(cnt.Negated(), aEmp, sEmp, t.mkSeqEq(r, xty))
	t.forcePhase(cnt)
	t.tightestPrefix(s, x)
}

func (t *SMTTheorySeq) addElimStringAxiom(n ast.ASTHandle) {
	s, _ := t.isString(n)
	if len(s) == 0 {
		return
	}
	x := t.loc()
	defer x.drop()
	result := t.ref(x, t.mkUnit(t.mkChar(s, len(s)-1)), nil)
	for i := len(s) - 1; i > 0; {
		i--
		result = t.ref(x, t.mkConcat2(t.mkUnit(t.mkChar(s, i)), result), nil)
	}
	t.addAxiom(t.mkEq(n, result, false))
	t.chk(t.rep.update(n, result, nil))
	t.newSolution = true
}

// addLengthAxiom, for n = len(x):
//
//	len(a ++ b) = len(a) + len(b) if x = a ++ b
//	len(unit(u)) = 1              if x = unit(u)
//	len(str) = str.length()       if x = str
//	len(empty) = 0                if x = empty
//	len(int.to.str(i)) >= 1       if x = int.to.str(i), more precisely below
//	len(x) >= 0                   otherwise
func (t *SMTTheorySeq) addLengthAxiom(n ast.ASTHandle) {
	x, _ := t.unaryArg(n, seqOpLength)
	if t.isConcat(x) || t.isUnit(x) || t.isEmpty(x) || t.isStringConst(x) {
		y := t.loc()
		ln := t.rewriteTh(y, n)
		t.addAxiom(t.mkEq(ln, n, false))
		y.drop()
	} else if t.isStrOp(x, strOpItos) {
		t.addItosLengthAxiom(n)
	} else {
		t.addAxiom(t.mkLiteral(t.mkGe(n, t.mkInt(0))))
	}
	if !t.atBaseLevel() {
		t.pushReplay(seqReplayAxiom, n)
	}
}

func (t *SMTTheorySeq) addItosLengthAxiom(ln ast.ASTHandle) {
	x, _ := t.unaryArg(ln, seqOpLength)
	n, _ := t.unaryArg(x, strOpItos)
	numChar1, numChar2 := int64(1), int64(1)
	ten := big.NewRat(10, 1)
	if len1, ok := t.getNumValue(n); ok {
		if len1.Sign() < 0 {
			return
		}
		// 0 <= x < 10, 10 <= x < 100, 100 <= x < 1000
		upper := big.NewRat(10, 1)
		for len1.Cmp(upper) > 0 {
			numChar1++
			upper.Mul(upper, ten)
		}
	}
	if len2, ok := t.getNumValue(ln); ok && len2.Sign() >= 0 && len2.Num().BitLen() <= 32 {
		numChar2 = len2.Num().Int64()
	}
	numChar := max(numChar1, numChar2)

	lenLe := t.mkLiteral(t.mkLe(ln, t.mkInt(numChar)))
	lenGe := t.mkLiteral(t.mkGe(ln, t.mkInt(numChar)))
	nGe0 := t.mkLiteral(t.mkGe(n, t.mkInt(0)))
	t.addAxiom(nGe0.Negated(), t.mkLiteral(t.mkGe(ln, t.mkInt(1))))

	if numChar == 1 {
		nGe10 := t.mkLiteral(t.mkGe(n, t.mkInt(10)))
		t.addAxiom(nGe0.Negated(), nGe10, lenLe)
		t.addAxiom(lenLe.Negated(), nGe10.Negated())
		return
	}
	hi := big.NewRat(1, 1)
	for i := int64(2); i < numChar; i++ {
		hi.Mul(hi, ten)
	}
	// n >= hi*10  <=>  len >= num_chars
	// n < 100*hi  <=>  len <= num_chars
	nGe10hi := t.mkLiteral(t.mkGe(n, t.mkIntRat(new(big.Rat).Mul(ten, hi))))
	nGe100hi := t.mkLiteral(t.mkGe(n, t.mkIntRat(new(big.Rat).Mul(new(big.Rat).Mul(ten, ten), hi))))
	t.addAxiom(nGe10hi.Negated(), lenGe)
	t.addAxiom(nGe100hi.Negated(), lenLe.Negated())
}

// mkSub is mk_sub(a, b): a - b rewritten. The expr_ref result is held until
// the current theory entry point returns.
func (t *SMTTheorySeq) mkSub(a, b ast.ASTHandle) ast.ASTHandle {
	result := t.ref(&t.tmps, t.mkSubExpr(a, b), nil)
	return t.rewriteTh(&t.tmps, result)
}

func (t *SMTTheorySeq) ensureEnode(e ast.ASTHandle) *SMTEnode {
	if !t.eInternalized(e) {
		t.chk(t.state.Internalize(e, false))
	}
	n := t.enodeOf(e)
	t.chk(t.state.markRelevant(n.Owner()))
	return n
}

// arith is get_th_arith: the theory_mi_arith or theory_i_arith serving
// arithmetic, if any, when e is internalized.
func (t *SMTTheorySeq) arith(e ast.ASTHandle) *SMTTheoryArith {
	fid, err := t.m.ArithmeticFamilyID()
	t.chk(err)
	a, ok := t.state.Theory(fid).(*SMTTheoryArith)
	if !ok || !t.eInternalized(e) {
		return nil
	}
	return a
}

// getArithValue is get_arith_value(ctx, afid, e, v).
func (t *SMTTheorySeq) getArithValue(e ast.ASTHandle) (*big.Rat, bool) {
	a := t.arith(e)
	if a == nil {
		return nil, false
	}
	v, ok, err := a.GetTheoryValue(t.enodeOf(e))
	t.chk(err)
	if !ok {
		return nil, false
	}
	return t.isNumeral(v)
}

func (t *SMTTheorySeq) getNumValue(e ast.ASTHandle) (*big.Rat, bool) {
	if !t.eInternalized(e) {
		return nil, false
	}
	n := t.enodeOf(e)
	next := n
	for {
		if val, ok := t.getArithValue(next.Owner()); ok && val.IsInt() {
			return val, true
		}
		next = next.Next()
		if next == n {
			return nil, false
		}
	}
}

func (t *SMTTheorySeq) lowerBound(_e ast.ASTHandle) (*big.Rat, bool) {
	x := t.loc()
	defer x.drop()
	e := t.ref(x, t.mkLength(_e), nil)
	a := t.arith(e)
	if a == nil {
		return nil, false
	}
	lo, ok, err := a.getLower(t.enodeOf(e))
	t.chk(err)
	if !ok {
		return nil, false
	}
	v, ok := t.isNumeral(lo)
	return v, ok && v.IsInt()
}

func (t *SMTTheorySeq) upperBound(_e ast.ASTHandle) (*big.Rat, bool) {
	x := t.loc()
	defer x.drop()
	e := t.ref(x, t.mkLength(_e), nil)
	a := t.arith(e)
	if a == nil {
		return nil, false
	}
	hi, ok, err := a.getUpper(t.enodeOf(e))
	t.chk(err)
	if !ok {
		return nil, false
	}
	v, ok := t.isNumeral(hi)
	return v, ok && v.IsInt()
}

// getLengthValue is get_length(e, val).
func (t *SMTTheorySeq) getLengthValue(e ast.ASTHandle) (*big.Rat, bool) {
	val := new(big.Rat)
	todo := []ast.ASTHandle{e}
	x := t.loc()
	defer x.drop()
	for len(todo) != 0 {
		c := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if e1, e2, ok := t.isConcat2(c); ok {
			todo = append(todo, e1, e2)
		} else if t.isUnit(c) {
			val.Add(val, big.NewRat(1, 1))
		} else if t.isEmpty(c) {
			continue
		} else if s, ok := t.isString(c); ok {
			val.Add(val, big.NewRat(int64(len(s)), 1))
		} else if !t.hasLength(c) {
			return nil, false
		} else {
			ln := t.ref(x, t.mkLength(c), nil)
			if !t.eInternalized(ln) {
				return nil, false
			}
			val1, ok := t.getArithValue(ln)
			if !ok {
				return nil, false
			}
			val.Add(val, val1)
		}
	}
	return val, val.IsInt()
}

func (t *SMTTheorySeq) addExtractAxiom(e ast.ASTHandle) {
	s, i, l, _ := t.isExtract(e)
	if t.isTailSIL(s, i, l) {
		t.addTailAxiom(e, s)
		return
	}
	if t.isDropLast(s, i, l) {
		t.addDropLastAxiom(e, s)
		return
	}
	if t.isExtractPrefix0(i) {
		t.addExtractPrefixAxiom(e, s, l)
		return
	}
	if t.isExtractSuffix(s, i, l) {
		t.addExtractSuffixAxiom(e, s, i)
		return
	}
	x0 := t.loc()
	defer x0.drop()
	x := t.ref(x0, t.mkSkolem(t.pre, s, i, ast.ASTHandle{}, ast.ASTHandle{}), nil)
	ls := t.ref(x0, t.mkLength(s), nil)
	lx := t.ref(x0, t.mkLength(x), nil)
	le := t.ref(x0, t.mkLength(e), nil)
	lsMinusIL := t.ref(x0, t.mkSub(t.mkSub(ls, i), l), nil)
	y := t.ref(x0, t.mkSkolem(t.post, s, lsMinusIL, ast.ASTHandle{}, ast.ASTHandle{}), nil)
	_ = t.ref(x0, t.mkConcat2(x, e), nil) // xe
	xey := t.ref(x0, t.mkConcat3(x, e, y), nil)
	zero := t.ref(x0, t.mkInt(0), nil)

	iGe0 := t.mkSimplifiedLiteral(t.mkGe(i, zero))
	lsLeI := t.mkSimplifiedLiteral(t.mkLe(t.mkSub(i, ls), zero))
	liGeLs := t.mkSimplifiedLiteral(t.mkGe(lsMinusIL, zero))
	lGeZero := t.mkSimplifiedLiteral(t.mkGe(l, zero))
	lsLe0 := t.mkSimplifiedLiteral(t.mkLe(ls, zero))

	t.addAxiom(iGe0.Negated(), lsLeI.Negated(), t.mkSeqEq(xey, s))
	t.addAxiom(iGe0.Negated(), lsLeI.Negated(), t.mkEq(lx, i, false))
	t.addAxiom(iGe0.Negated(), lsLeI.Negated(), lGeZero.Negated(), liGeLs.Negated(), t.mkEq(le, l, false))
	t.addAxiom(iGe0.Negated(), lsLeI.Negated(), liGeLs, t.mkEq(le, t.mkSub(ls, i), false))
	t.addAxiom(iGe0.Negated(), lsLeI.Negated(), lGeZero, t.mkEq(le, zero, false))
	t.addAxiom(iGe0, t.mkEq(le, zero, false))
	t.addAxiom(lsLeI, t.mkEq(le, zero, false))
	t.addAxiom(lsLe0.Negated(), t.mkEq(le, zero, false))
}

func (t *SMTTheorySeq) addTailAxiom(e, s ast.ASTHandle) {
	x := t.loc()
	defer x.drop()
	head, _ := t.mkDecompose(x, s)
	t.addAxiom(t.mkEqEmpty(s, true), t.mkSeqEq(s, t.mkConcat2(head, e)))
}

func (t *SMTTheorySeq) addDropLastAxiom(e, s ast.ASTHandle) {
	t.addAxiom(t.mkEqEmpty(s, true), t.mkSeqEq(s, t.mkConcat2(e, t.mkUnit(t.mkLast(s)))))
}

func (t *SMTTheorySeq) lengthMinusOneEq(s, l ast.ASTHandle) bool {
	x := t.loc()
	defer x.drop()
	l1 := t.ref(x, l, nil)
	l2 := t.ref(x, t.mkSubExpr(t.mkLength(s), t.mkInt(1)), nil)
	l1 = t.rewriteTh(x, l1)
	l2 = t.rewriteTh(x, l2)
	return l1 == l2
}

func (t *SMTTheorySeq) isDropLast(s, i, l ast.ASTHandle) bool {
	i1, ok := t.isNumeral(i)
	if !ok || i1.Sign() != 0 {
		return false
	}
	return t.lengthMinusOneEq(s, l)
}

// isTailSIL is is_tail(s, i, l).
func (t *SMTTheorySeq) isTailSIL(s, i, l ast.ASTHandle) bool {
	i1, ok := t.isNumeral(i)
	if !ok || i1.Cmp(big.NewRat(1, 1)) != 0 {
		return false
	}
	return t.lengthMinusOneEq(s, l)
}

func (t *SMTTheorySeq) isExtractPrefix0(i ast.ASTHandle) bool {
	i1, ok := t.isNumeral(i)
	return ok && i1.Sign() == 0
}

func (t *SMTTheorySeq) isExtractSuffix(s, i, l ast.ASTHandle) bool {
	x := t.loc()
	defer x.drop()
	ln := t.ref(x, t.mkAdd(l, i), nil)
	ln = t.rewriteTh(x, ln)
	a, ok := t.unaryArg(ln, seqOpLength)
	return ok && a == s
}

// addExtractPrefixAxiom:
//
//	0 <= l <= len(s) => s = ey & l = len(e)
//	len(s) < l => s = e
func (t *SMTTheorySeq) addExtractPrefixAxiom(e, s, l ast.ASTHandle) {
	x := t.loc()
	defer x.drop()
	le := t.ref(x, t.mkLength(e), nil)
	ls := t.ref(x, t.mkLength(s), nil)
	lsMinusL := t.ref(x, t.mkSub(ls, l), nil)
	y := t.ref(x, t.mkSkolem(t.post, s, lsMinusL, ast.ASTHandle{}, ast.ASTHandle{}), nil)
	zero := t.ref(x, t.mkInt(0), nil)
	ey := t.ref(x, t.mkConcat2(e, y), nil)
	lGe0 := t.mkSimplifiedLiteral(t.mkGe(l, zero))
	lLeS := t.mkSimplifiedLiteral(t.mkLe(t.mkSub(l, ls), zero))
	t.addAxiom(lGe0.Negated(), lLeS.Negated(), t.mkSeqEq(s, ey))
	t.addAxiom(lGe0.Negated(), lLeS.Negated(), t.mkEq(l, le, false))
	t.addAxiom(lGe0.Negated(), lLeS.Negated(), t.mkEq(lsMinusL, t.mkLength(y), false))
	t.addAxiom(lLeS, t.mkEq(e, s, false))
}

// addExtractSuffixAxiom: 0 <= i <= len(s) => s = xe & i = len(x)
func (t *SMTTheorySeq) addExtractSuffixAxiom(e, s, i ast.ASTHandle) {
	y := t.loc()
	defer y.drop()
	x := t.ref(y, t.mkSkolem(t.pre, s, i, ast.ASTHandle{}, ast.ASTHandle{}), nil)
	lx := t.ref(y, t.mkLength(x), nil)
	ls := t.ref(y, t.mkLength(s), nil)
	zero := t.ref(y, t.mkInt(0), nil)
	xe := t.ref(y, t.mkConcat2(x, e), nil)
	iGe0 := t.mkSimplifiedLiteral(t.mkGe(i, zero))
	iLeS := t.mkSimplifiedLiteral(t.mkLe(t.mkSub(i, ls), zero))
	t.addAxiom(iGe0.Negated(), iLeS.Negated(), t.mkSeqEq(s, xe))
	t.addAxiom(iGe0.Negated(), iLeS.Negated(), t.mkEq(i, lx, false))
}

// addAtAxiom, for e = at(s, i):
//
//	0 <= i < len(s) -> s = xey & len(x) = i & len(e) = 1
//	i < 0 \/ i >= len(s) -> e = empty
func (t *SMTTheorySeq) addAtAxiom(e ast.ASTHandle) {
	s, i, _ := t.binaryArgs(e, seqOpAt)
	x0 := t.loc()
	defer x0.drop()
	lenE := t.ref(x0, t.mkLength(e), nil)
	lenS := t.ref(x0, t.mkLength(s), nil)
	zero := t.ref(x0, t.mkInt(0), nil)
	one := t.ref(x0, t.mkInt(1), nil)
	x := t.ref(x0, t.mkSkolem(t.pre, s, i, ast.ASTHandle{}, ast.ASTHandle{}), nil)
	y := t.ref(x0, t.mkSkolem(t.post, s, t.mkSub(t.mkSub(lenS, i), one), ast.ASTHandle{}, ast.ASTHandle{}), nil)
	xey := t.ref(x0, t.mkConcat3(x, e, y), nil)
	lenX := t.ref(x0, t.mkLength(x), nil)
	emp := t.ref(x0, t.mkEmpty(t.sortOf(e)), nil)

	iGe0 := t.mkSimplifiedLiteral(t.mkGe(i, zero))
	iGeLenS := t.mkSimplifiedLiteral(t.mkGe(t.mkSub(i, t.mkLength(s)), zero))

	t.addAxiom(iGe0.Negated(), iGeLenS, t.mkSeqEq(s, xey))
	t.addAxiom(iGe0.Negated(), iGeLenS, t.mkEq(one, lenE, false))
	t.addAxiom(iGe0.Negated(), iGeLenS, t.mkEq(i, lenX, false))
	t.addAxiom(iGe0, t.mkEq(e, emp, false))
	t.addAxiom(iGeLenS.Negated(), t.mkEq(e, emp, false))
}

// propagateStep: step(s, idx, re, i, j, t) -> nth(s, idx) == t & len(s) > idx
func (t *SMTTheorySeq) propagateStep(lit SMTLiteral, step ast.ASTHandle) {
	s, idx, _, _, _, acc, _ := t.isStep6(step)
	t.propagateLit(nil, []SMTLiteral{lit}, t.mkSimplifiedLiteral(acc))
	lo, okLo := t.lowerBound(s)
	_idx, okIdx := t.isNumeral(idx)
	if okLo && lo.Sign() >= 0 && lo.Num().BitLen() <= 32 && okIdx && lo.Cmp(_idx) >= 0 {
		// skip
	} else {
		t.propagateLit(nil, []SMTLiteral{lit}, t.mkLiteral(t.mkLe(t.mkLength(s), idx)).Negated())
	}
	t.ensureNth(lit, s, idx)
}

// ensureNth: lit => s = (nth s 0) ++ (nth s 1) ++ ... ++ (nth s idx) ++ (tail s idx)
func (t *SMTTheorySeq) ensureNth(lit SMTLiteral, s, idx ast.ASTHandle) {
	r, _ := t.isNumeral(idx)
	_idx := r.Num().Int64()
	x := t.loc()
	defer x.drop()
	elems := newSeqExprVector(t.m)
	defer func() { t.chk(elems.reset()) }()
	s2 := s
	for j := int64(0); j <= _idx; j++ {
		head, tail := t.mkDecompose(x, s2)
		t.chk(elems.pushBack(head))
		len1 := t.ref(x, t.mkLength(s2), nil)
		len2 := t.ref(x, t.mkAdd(t.mkInt(1), t.mkLength(tail)), nil)
		t.propagateEqLitOnly(lit, len1, len2, false)
		s2 = tail
	}
	t.chk(elems.pushBack(s2))
	conc := t.ref(x, t.mkConcatSorted(elems.handles(), t.sortOf(s)), nil)
	t.propagateEqLitOnly(lit, s, conc, true)
}

func (t *SMTTheorySeq) mkSimplifiedLiteral(_e ast.ASTHandle) SMTLiteral {
	x := t.loc()
	defer x.drop()
	e := t.ref(x, _e, nil)
	e = t.rewriteTh(x, e)
	return t.mkLiteral(e)
}

func (t *SMTTheorySeq) mkLiteral(_e ast.ASTHandle) SMTLiteral {
	x := t.loc()
	defer x.drop()
	e := t.ref(x, _e, nil)
	t.ensureEnode(e)
	l, err := t.state.Literal(e)
	t.chk(err)
	return l
}

func (t *SMTTheorySeq) mkSeqEq(a, b ast.ASTHandle) SMTLiteral {
	boolSort, err := t.m.MkBoolSort()
	t.chk(err)
	return t.mkLiteral(t.mkSkolem(t.eqSym, a, b, ast.ASTHandle{}, boolSort))
}

func (t *SMTTheorySeq) mkEqEmpty(_e ast.ASTHandle, phase bool) SMTLiteral {
	x := t.loc()
	defer x.drop()
	e := t.ref(x, _e, nil)
	if t.isEmpty(e) {
		return SMTTrueLiteral
	}
	for _, c := range t.getConcat(e) {
		if t.isUnit(c) {
			return SMTFalseLiteral
		}
		if s, ok := t.isString(c); ok && len(s) > 0 {
			return SMTFalseLiteral
		}
	}
	emp := t.ref(x, t.mkEmpty(t.sortOf(e)), nil)
	lit := t.mkEq(e, emp, false)
	if phase {
		t.forcePhase(lit)
	} else {
		t.forcePhase(lit.Negated())
	}
	t.markLitRelevant(lit)
	return lit
}

// addAxiom is add_axiom(l1, ..., l5); absent literals are SMTNullLiteral.
func (t *SMTTheorySeq) addAxiom(ls ...SMTLiteral) {
	for _, l := range ls {
		if l == SMTTrueLiteral {
			return
		}
	}
	var lits []SMTLiteral
	for _, l := range ls {
		if l != SMTNullLiteral && l != SMTFalseLiteral {
			t.markLitRelevant(l)
			lits = append(lits, l)
		}
	}
	t.traceAddAxiom(lits)
	t.newPropagation = true
	t.stats.addAxiom++
	t.chk(t.state.addTheoryAxiom(SMTTheoryID(t.FamilyID()), lits))
}

// mkSkolem is mk_skolem(name, e1, e2, e3, range); null handles are absent.
func (t *SMTTheorySeq) mkSkolem(name usymbol.Symbol, e1, e2, e3, rng ast.ASTHandle) ast.ASTHandle {
	es := []ast.ASTHandle{e1}
	if !e3.IsNull() {
		es = []ast.ASTHandle{e1, e2, e3}
	} else if !e2.IsNull() {
		es = []ast.ASTHandle{e1, e2}
	}
	if rng.IsNull() {
		rng = t.sortOf(e1)
	}
	return t.must(t.m.MkSequenceSkolem(name, es, rng))
}

func (t *SMTTheorySeq) isSkolem(s usymbol.Symbol, e ast.ASTHandle) bool {
	if !t.mustBool(t.m.IsSequenceSkolem(e)) {
		return false
	}
	decl := t.must(t.m.GetDecl(e))
	p, err := t.m.GetDeclParameterCopy(decl, 0)
	t.chk(err)
	sym, err := t.m.ParameterSymbol(p)
	t.chk(errors.Join(err, t.m.DelParameter(&p)))
	return sym == s
}

func (t *SMTTheorySeq) mkJoinLit(deps *seqDep, lit SMTLiteral) *seqDep {
	return t.dm.mkJoin(deps, t.dm.mkLeaf(seqAssumption{lit: lit}))
}

func (t *SMTTheorySeq) mkJoinLits(deps *seqDep, lits []SMTLiteral) *seqDep {
	for _, l := range lits {
		deps = t.mkJoinLit(deps, l)
	}
	return deps
}

// propagateEqLitOnly is propagate_eq(lit, e1, e2, add_to_eqs).
func (t *SMTTheorySeq) propagateEqLitOnly(lit SMTLiteral, e1, e2 ast.ASTHandle, addToEqs bool) {
	t.propagateEqLits(nil, []SMTLiteral{lit}, e1, e2, addToEqs)
}

// propagateEqLits is propagate_eq(deps, lits, e1, e2, add_to_eqs).
func (t *SMTTheorySeq) propagateEqLits(deps *seqDep, _lits []SMTLiteral, e1, e2 ast.ASTHandle, addToEqs bool) {
	n1 := t.ensureEnode(e1)
	n2 := t.ensureEnode(e2)
	if n1.Root() == n2.Root() {
		return
	}
	t.chk(t.state.markRelevant(n1.Owner()))
	t.chk(t.state.markRelevant(n2.Owner()))
	lits := append([]SMTLiteral(nil), _lits...)
	var eqs []SMTEnodePair
	if !t.linearize(deps, &eqs, &lits) {
		return
	}
	if addToEqs {
		deps = t.mkJoinLits(deps, _lits)
		t.newEqEhDep(deps, n1, n2)
	}
	js, err := NewSMTExtTheoryEqualityPropagationJustification(t.m, t.FamilyID(), lits, eqs, n1, n2, nil)
	t.chk(err)
	t.newPropagation = true
	t.state.ownedJustifications = append(t.state.ownedJustifications, js)
	t.chk(t.state.assignEquality(n1, n2, NewSMTEqualityJustification(js)))
}
