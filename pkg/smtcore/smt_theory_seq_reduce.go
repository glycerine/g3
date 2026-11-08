package smtcore

// Source: src/smt/theory_seq.cpp:1554-1840, is_binary_eq through
// get_length(e, len, lits).

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (t *SMTTheorySeq) isBinaryEq(ls, rs []ast.ASTHandle) (x ast.ASTHandle, xs, ys []ast.ASTHandle, y ast.ASTHandle, ok bool) {
	if len(ls) > 1 && t.isVar(ls[0]) && len(rs) > 1 && t.isVar(rs[len(rs)-1]) {
		x = ls[0]
		y = rs[len(rs)-1]
		for i := 1; i < len(ls); i++ {
			if !t.isUnit(ls[i]) {
				return x, nil, nil, y, false
			}
		}
		for i := 0; i < len(rs)-1; i++ {
			if !t.isUnit(rs[i]) {
				return x, nil, nil, y, false
			}
		}
		xs = append([]ast.ASTHandle(nil), ls[1:]...)
		ys = append([]ast.ASTHandle(nil), rs[:len(rs)-1]...)
		return x, xs, ys, y, true
	}
	return ast.ASTHandle{}, nil, nil, ast.ASTHandle{}, false
}

// pushEq is m_eqs.push_back(eq(m_eq_id++, lhs, rhs, deps)).
func (t *SMTTheorySeq) pushEq(lhs, rhs []ast.ASTHandle, deps *seqDep) {
	e := seqEq{id: t.eqID, ls: newSeqExprVector(t.m), rs: newSeqExprVector(t.m), dep: deps}
	t.eqID++
	t.chk(e.ls.append(lhs))
	t.chk(e.rs.append(rhs))
	t.eqs.pushBack(e)
}

func (t *SMTTheorySeq) reduceLengthEq(ls, rs []ast.ASTHandle, deps *seqDep) bool {
	if len(ls) == 0 || len(rs) == 0 {
		return false
	}
	if len(ls) <= 1 && len(rs) <= 1 {
		return false
	}
	x := t.loc()
	defer x.drop()
	var lits []SMTLiteral
	l := t.ref(x, ls[0], nil)
	r := t.ref(x, rs[0], nil)
	if t.reduceLength(l, r, &lits) {
		deps = t.mkJoinLits(deps, lits)
		t.pushEq(ls[1:], rs[1:], deps)
		t.propagateEqLits(deps, lits, l, r, true)
		return true
	}
	l = t.ref(x, ls[len(ls)-1], nil)
	r = t.ref(x, rs[len(rs)-1], nil)
	if t.reduceLength(l, r, &lits) {
		deps = t.mkJoinLits(deps, lits)
		t.pushEq(ls[:len(ls)-1], rs[:len(rs)-1], deps)
		t.propagateEqLits(deps, lits, l, r, true)
		return true
	}

	getLen := func(e ast.ASTHandle, out **big.Rat) bool {
		v, ok := t.getLengthValue(e)
		if ok {
			*out = v
		}
		return ok
	}
	var len1, len2, ln *big.Rat
	if len(ls) > 1 && getLen(ls[0], &len1) && getLen(rs[0], &len2) && len1.Cmp(len2) >= 0 {
		len2 = new(big.Rat).Set(len2)
		j := 1
		for ; j < len(rs) && len1.Cmp(len2) > 0 && getLen(rs[j], &ln); j++ {
			len2.Add(len2, ln)
		}
		if len1.Cmp(len2) == 0 && 0 < j && j < len(rs) && t.reduceLengthSplit(1, j, true, ls, rs, deps) {
			return true
		}
	}
	if len(rs) > 1 && getLen(rs[0], &len1) && getLen(ls[0], &len2) && len1.Cmp(len2) > 0 {
		len2 = new(big.Rat).Set(len2)
		j := 1
		for ; j < len(ls) && len1.Cmp(len2) > 0 && getLen(ls[j], &ln); j++ {
			len2.Add(len2, ln)
		}
		if len1.Cmp(len2) == 0 && 0 < j && j < len(ls) && t.reduceLengthSplit(j, 1, true, ls, rs, deps) {
			return true
		}
	}
	if len(ls) > 1 && getLen(ls[len(ls)-1], &len1) && getLen(rs[len(rs)-1], &len2) && len1.Cmp(len2) >= 0 {
		len2 = new(big.Rat).Set(len2)
		j := len(rs) - 1
		for ; j > 0 && len1.Cmp(len2) > 0 && getLen(rs[j-1], &ln); j-- {
			len2.Add(len2, ln)
		}
		if len1.Cmp(len2) == 0 && 0 < j && j < len(rs) && t.reduceLengthSplit(len(ls)-1, len(rs)-j, false, ls, rs, deps) {
			return true
		}
	}
	if len(rs) > 1 && getLen(rs[len(rs)-1], &len1) && getLen(ls[len(ls)-1], &len2) && len1.Cmp(len2) > 0 {
		len2 = new(big.Rat).Set(len2)
		j := len(ls) - 1
		for ; j > 0 && len1.Cmp(len2) > 0 && getLen(ls[j-1], &ln); j-- {
			len2.Add(len2, ln)
		}
		if len1.Cmp(len2) == 0 && 0 < j && j < len(ls) && t.reduceLengthSplit(len(ls)-j, len(rs)-1, false, ls, rs, deps) {
			return true
		}
	}
	return false
}

// reduceLengthSplit is reduce_length(i, j, front, ls, rs, deps).
func (t *SMTTheorySeq) reduceLengthSplit(i, j int, front bool, ls, rs []ast.ASTHandle, deps *seqDep) bool {
	ls1, ls2 := ls[:i], ls[i:]
	rs1, rs2 := rs[:j], rs[j:]
	if !front {
		ls1, ls2 = ls2, ls1
		rs1, rs2 = rs2, rs1
	}
	x := t.loc()
	defer x.drop()
	l := t.ref(x, t.mkConcatN(ls1), nil)
	r := t.ref(x, t.mkConcatN(rs1), nil)
	lenl := t.ref(x, t.mkLength(l), nil)
	lenr := t.ref(x, t.mkLength(r), nil)
	lit := t.mkEq(lenl, lenr, false)
	if t.assignment(lit) == lbool.True {
		deps = t.mkJoinLit(deps, lit)
		t.pushEq(ls2, rs2, deps)
		t.propagateEqDep(deps, l, r, true)
		return true
	}
	return false
}

func (t *SMTTheorySeq) solveBinaryEq(ls, rs []ast.ASTHandle, dep *seqDep) bool {
	x, xs, ys, y, isBinary := t.isBinaryEq(ls, rs)
	if !isBinary {
		x, xs, ys, y, isBinary = t.isBinaryEq(rs, ls)
	}
	if !isBinary {
		return false
	}
	// Equation is of the form x ++ xs = ys ++ y where xs, ys are units.
	if x != y {
		return false
	}
	if len(xs) != len(ys) {
		t.setConflict(dep, nil)
		return false
	}
	if len(xs) == 0 {
		return false // UNREACHABLE: this should have been solved already
	}
	sz := len(xs)
	var conflict []SMTLiteral
	for offset := 0; offset < sz; offset++ {
		hasConflict := false
		for j := 0; !hasConflict && j < sz; j++ {
			j1 := (offset + j) % sz
			if xs[j] == ys[j1] {
				continue
			}
			eq := t.mkEq(xs[j], ys[j1], false)
			switch t.assignment(eq) {
			case lbool.False:
				conflict = append(conflict, eq.Negated())
				hasConflict = true
			case lbool.Undef:
				n1 := t.ensureEnode(xs[j])
				n2 := t.ensureEnode(ys[j1])
				if n1.Root() == n2.Root() {
					break
				}
				t.markLitRelevant(eq)
				if sz == 1 {
					t.propagateLit(dep, nil, eq)
					return true
				}
				t.newPropagation = true
			}
		}
		if !hasConflict {
			// current equalities can work when solving x ++ xs = ys ++ y
			return false
		}
	}
	t.setConflict(dep, conflict)
	return false
}

// getLengthLits is get_length(e, len, lits). len is an expr_ref held in the
// caller's locals x.
func (t *SMTTheorySeq) getLengthLits(x *seqRefs, e ast.ASTHandle, lits *[]SMTLiteral) (ast.ASTHandle, bool) {
	isZero := func(i ast.ASTHandle) bool {
		r, ok := t.isNumeral(i)
		return ok && r.Sign() == 0
	}
	allTrue := func(ls []SMTLiteral) bool {
		for _, l := range ls {
			if t.assignment(l) != lbool.True {
				return false
			}
		}
		return true
	}
	if s, i, l, ok := t.isExtract(e); ok {
		// 0 <= i <= len(s), 0 <= l, i + l <= len(s)
		y := t.loc()
		zero := t.ref(y, t.mkInt(0), nil)
		ls := t.ref(y, t.mkLength(s), nil)
		lsMinusIL := t.ref(y, t.mkSub(t.mkSub(ls, i), l), nil)
		iGe0 := SMTTrueLiteral
		if !isZero(i) {
			iGe0 = t.mkSimplifiedLiteral(t.mkGe(i, zero))
		}
		iLtLenS := t.mkSimplifiedLiteral(t.mkGe(t.mkSub(i, ls), zero)).Negated()
		liGeLs := t.mkSimplifiedLiteral(t.mkGe(lsMinusIL, zero))
		lGeZero := t.mkSimplifiedLiteral(t.mkGe(l, zero))
		_lits := []SMTLiteral{iGe0, iLtLenS, liGeLs, lGeZero}
		y.drop()
		if allTrue(_lits) {
			*lits = append(*lits, _lits...)
			return t.ref(x, l, nil), true
		}
	} else if s, i, ok := t.binaryArgs(e, seqOpAt); ok {
		// has length 1 if 0 <= i < len(s)
		y := t.loc()
		zero := t.ref(y, t.mkInt(0), nil)
		iGe0 := SMTTrueLiteral
		if !isZero(i) {
			iGe0 = t.mkSimplifiedLiteral(t.mkGe(i, zero))
		}
		iLtLenS := t.mkSimplifiedLiteral(t.mkGe(t.mkSub(i, t.mkLength(s)), zero)).Negated()
		_lits := []SMTLiteral{iGe0, iLtLenS}
		y.drop()
		if allTrue(_lits) {
			*lits = append(*lits, _lits...)
			return t.ref(x, t.mkInt(1), nil), true
		}
	} else if s, i, ok := t.isPre(e); ok {
		y := t.loc()
		zero := t.ref(y, t.mkInt(0), nil)
		iGe0 := SMTTrueLiteral
		if !isZero(i) {
			iGe0 = t.mkSimplifiedLiteral(t.mkGe(i, zero))
		}
		iLtLenS := t.mkSimplifiedLiteral(t.mkGe(t.mkSub(i, t.mkLength(s)), zero)).Negated()
		_lits := []SMTLiteral{iGe0, iLtLenS}
		y.drop()
		if allTrue(_lits) {
			*lits = append(*lits, _lits...)
			return t.ref(x, i, nil), true
		}
	} else if s, l, ok := t.isPost(e); ok {
		y := t.loc()
		zero := t.ref(y, t.mkInt(0), nil)
		lGe0 := t.mkSimplifiedLiteral(t.mkGe(l, zero))
		lLeLenS := t.mkSimplifiedLiteral(t.mkGe(t.mkSub(t.mkLength(s), l), zero))
		_lits := []SMTLiteral{lGe0, lLeLenS}
		y.drop()
		if allTrue(_lits) {
			*lits = append(*lits, _lits...)
			return t.ref(x, l, nil), true
		}
	} else if t.isUnit(e) {
		return t.ref(x, t.mkInt(1), nil), true
	}
	return ast.ASTHandle{}, false
}

// isExtract is str.is_extract(e, s, i, l).
func (t *SMTTheorySeq) isExtract(e ast.ASTHandle) (s, i, l ast.ASTHandle, ok bool) {
	s, i, l, ok, err := t.m.SequenceExtractArguments(e)
	t.chk(err)
	return s, i, l, ok
}
