package smtcore

// Source: src/smt/theory_seq.cpp:321-1125, reduce_length_eq through
// mk_decompose.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (t *SMTTheorySeq) reduceLengthEqAll() bool {
	start := int(t.state.random.Next())
	n := t.eqs.len()
	for i := 0; !t.state.inconsistent && i < n; i++ {
		e := t.eqs.at((i + start) % n)
		if t.reduceLengthEq(e.ls.handles(), e.rs.handles(), e.dep) {
			return true
		}
	}
	return false
}

func (t *SMTTheorySeq) branchBinaryVariableAll() bool {
	for i := 0; i < t.eqs.len(); i++ {
		if t.branchBinaryVariable(t.eqs.at(i)) {
			return true
		}
	}
	return false
}

func (t *SMTTheorySeq) branchBinaryVariable(e *seqEq) bool {
	if t.isComplex(e) {
		return false
	}
	x, xs, ys, y, isBinary := t.isBinaryEq(e.ls.handles(), e.rs.handles())
	if !isBinary {
		x, xs, ys, y, isBinary = t.isBinaryEq(e.rs.handles(), e.ls.handles())
	}
	if !isBinary {
		return false
	}
	if x == y {
		return false
	}
	// Equation is of the form x ++ xs = ys ++ y where xs, ys are units.
	if t.branchVariableEq(e) {
		return true
	}
	lenX, ok := t.getLengthValue(x)
	if !ok {
		t.enforceLengthNode(t.ensureEnode(x))
		return true
	}
	lenY, ok := t.getLengthValue(y)
	if !ok {
		t.enforceLengthNode(t.ensureEnode(y))
		return true
	}
	x0 := t.loc()
	defer x0.drop()
	sumX := new(big.Rat).Add(lenX, big.NewRat(int64(len(xs)), 1))
	sumY := new(big.Rat).Add(lenY, big.NewRat(int64(len(ys)), 1))
	if sumX.Cmp(sumY) != 0 {
		// |x| - |y| = |ys| - |xs|
		lenY := t.mkLength(y) // arguments right to left
		a := t.ref(x0, t.mkSub(t.mkLength(x), lenY), nil)
		// unsigned subtraction, as the source writes ys.size()-xs.size(),
		// converted to mk_int's int parameter
		b := t.ref(x0, t.mkInt(int64(int32(uint32(len(ys))-uint32(len(xs))))), nil)
		t.propagateLit(e.dep, nil, t.mkEq(a, b, false))
		return true
	}
	if lenX.Cmp(big.NewRat(int64(len(ys)), 1)) <= 0 {
		t.branchUnitVariableDep(e.dep, x, ys)
		return true
	}
	size := t.mkInt(int64(len(ys))) // arguments right to left
	le := t.ref(x0, t.mkLe(t.mkLength(x), size), nil)
	lit := t.mkLiteral(le)
	if t.assignment(lit) == lbool.False {
		// |x| > |ys| => x = ys ++ y1, y = y1 ++ y2, y2 = xs
		Y1 := t.ref(x0, t.mkSkolem(t.symbol("seq.left"), x, y, ast.ASTHandle{}, ast.ASTHandle{}), nil)
		Y2 := t.ref(x0, t.mkSkolem(t.symbol("seq.right"), x, y, ast.ASTHandle{}, ast.ASTHandle{}), nil)
		ys = append(ys, Y1)
		ysY1 := t.ref(x0, t.mkConcatN(ys), nil)
		xsE := t.ref(x0, t.mkConcatN(xs), nil)
		Y1Y2 := t.ref(x0, t.mkConcat2(Y1, Y2), nil)
		dep := e.dep
		t.propagateEqLit(dep, lit.Negated(), x, ysY1, true)
		t.propagateEqLit(dep, lit.Negated(), y, Y1Y2, true)
		t.propagateEqLit(dep, lit.Negated(), Y2, xsE, true)
	} else {
		t.markLitRelevant(lit)
	}
	return true
}

func (t *SMTTheorySeq) branchUnitVariable() bool {
	for i := 0; i < t.eqs.len(); i++ {
		e := t.eqs.at(i)
		if t.isUnitEq(e.ls.handles(), e.rs.handles()) {
			t.branchUnitVariableDep(e.dep, e.ls.get(0), e.rs.handles())
			return true
		} else if t.isUnitEq(e.rs.handles(), e.ls.handles()) {
			t.branchUnitVariableDep(e.dep, e.rs.get(0), e.ls.handles())
			return true
		}
	}
	return false
}

// isUnitEq: ls := X... == rs := abcdef
func (t *SMTTheorySeq) isUnitEq(ls, rs []ast.ASTHandle) bool {
	if len(ls) == 0 || !t.isVar(ls[0]) {
		return false
	}
	for _, r := range rs {
		if !t.isUnit(r) {
			return false
		}
	}
	return true
}

func (t *SMTTheorySeq) branchUnitVariableDep(dep *seqDep, X ast.ASTHandle, units []ast.ASTHandle) {
	lenX, ok := t.getLengthValue(X)
	if !ok {
		t.enforceLengthNode(t.ensureEnode(X))
		return
	}
	x := t.loc()
	defer x.drop()
	if lenX.Cmp(big.NewRat(int64(len(units)), 1)) > 0 {
		size := t.mkInt(int64(len(units))) // arguments right to left
		le := t.ref(x, t.mkLe(t.mkLength(X), size), nil)
		t.propagateLit(dep, nil, t.mkLiteral(le))
		return
	}
	lX := lenX.Num().Int64()
	if lX == 0 {
		t.setEmpty(X)
		return
	}
	lenXE := t.mkLength(X) // arguments right to left
	lit := t.mkEq(t.mkInt(lX), lenXE, false)
	if t.assignment(lit) == lbool.True {
		R := t.ref(x, t.mkConcatN(units[:lX]), nil)
		t.propagateEqLit(dep, lit, X, R, true)
	} else {
		t.markLitRelevant(lit)
		t.forcePhase(lit)
	}
}

func (t *SMTTheorySeq) branchVariableMb() bool {
	change := false
	for i := 0; i < t.eqs.len(); i++ {
		e := t.eqs.at(i)
		if !t.isComplex(e) {
			continue
		}
		if e.ls.empty() || e.rs.empty() || (!t.isVar(e.ls.get(0)) && !t.isVar(e.rs.get(0))) {
			continue
		}
		len1, ok1 := t.enforceLengths(e.ls.handles())
		if !ok1 {
			change = true
			continue
		}
		len2, ok2 := t.enforceLengths(e.rs.handles())
		if !ok2 {
			change = true
			continue
		}
		l1, l2 := new(big.Rat), new(big.Rat)
		for _, v := range len1 {
			l1.Add(l1, v)
		}
		for _, v := range len2 {
			l2.Add(l2, v)
		}
		if l1.Cmp(l2) != 0 {
			x := t.loc()
			l := t.ref(x, t.mkConcatN(e.ls.handles()), nil)
			r := t.ref(x, t.mkConcatN(e.rs.handles()), nil)
			lnl := t.ref(x, t.mkLength(l), nil)
			lnr := t.ref(x, t.mkLength(r), nil)
			t.propagateEqDep(e.dep, lnl, lnr, false)
			x.drop()
			change = true
			continue
		}
		if t.splitLengths(e.dep, e.ls.handles(), e.rs.handles(), len1, len2) {
			change = true
			break
		}
	}
	return change
}

func (t *SMTTheorySeq) isComplex(e *seqEq) bool {
	numVars1, numVars2 := 0, 0
	for _, a := range e.ls.handles() {
		if t.isVar(a) {
			numVars1++
		}
	}
	for _, a := range e.rs.handles() {
		if t.isVar(a) {
			numVars2++
		}
	}
	return numVars1 > 0 && numVars2 > 0 && numVars1+numVars2 > 2
}

// splitLengths decomposes ls = rs into Xa = bYc. See theory_seq.cpp:531.
func (t *SMTTheorySeq) splitLengths(dep *seqDep, ls, rs []ast.ASTHandle, ll, rl []*big.Rat) bool {
	if len(ls) == 0 || len(rs) == 0 {
		return false
	}
	if t.isVar(ls[0]) && ll[0].Sign() == 0 {
		return t.setEmpty(ls[0])
	}
	if t.isVar(rs[0]) && rl[0].Sign() == 0 {
		return t.setEmpty(rs[0])
	}
	if t.isVar(rs[0]) && !t.isVar(ls[0]) {
		return t.splitLengths(dep, rs, ls, rl, ll)
	}
	if !t.isVar(ls[0]) {
		return false
	}
	x := t.loc()
	defer x.drop()
	X := t.ref(x, ls[0], nil)
	lenX := ll[0]
	bs := newSeqExprVector(t.m)
	defer func() { t.chk(bs.reset()) }()
	lenB, lenY := new(big.Rat), new(big.Rat)
	for i := 0; lenX.Cmp(lenB) > 0 && i < len(rs); i++ {
		t.chk(bs.pushBack(rs[i]))
		lenY = rl[i]
		lenB.Add(lenB, lenY)
	}
	Y := t.ref(x, bs.back(), nil)
	t.chk(bs.shrink(bs.size() - 1))
	if !t.isVar(Y) && !t.isUnit(Y) {
		return false
	}
	if X == Y {
		return false
	}
	if lenY.Sign() == 0 {
		return t.setEmpty(Y)
	}
	b := t.ref(x, t.mkConcatSorted(bs.handles(), t.sortOf(X)), nil)

	// |b| < |X| <= |b| + |Y| => x = bY1, Y = Y1Y2
	lenXE := t.ref(x, t.mkLength(X), nil)
	lenYE := t.ref(x, t.mkLength(Y), nil)
	lenb := t.ref(x, t.mkLength(b), nil)
	// GCC evaluates call arguments right to left.
	zero := t.mkInt(0)
	le1 := t.ref(x, t.mkLe(t.mkSub(lenXE, lenb), zero), nil)
	zero = t.mkInt(0)
	le2 := t.ref(x, t.mkLe(t.mkSub(t.mkSub(lenXE, lenb), lenYE), zero), nil)
	lit1 := t.mkLiteral(le1).Negated()
	lit2 := t.mkLiteral(le2)
	lits := []SMTLiteral{lit1, lit2}

	if t.assignment(lit1) != lbool.True || t.assignment(lit2) != lbool.True {
		t.markLitRelevant(lit1)
		t.markLitRelevant(lit2)
	} else if t.isUnit(Y) {
		t.chk(bs.pushBack(Y))
		bY := t.ref(x, t.mkConcatN(bs.handles()), nil)
		t.propagateEqLits(dep, lits, X, bY, true)
	} else {
		Y1 := t.ref(x, t.mkSkolem(t.symbol("seq.left"), X, b, Y, ast.ASTHandle{}), nil)
		Y2 := t.ref(x, t.mkSkolem(t.symbol("seq.right"), X, b, Y, ast.ASTHandle{}), nil)
		bY1 := t.ref(x, t.mkConcat2(b, Y1), nil)
		Y1Y2 := t.ref(x, t.mkConcat2(Y1, Y2), nil)
		t.propagateEqLits(dep, lits, X, bY1, true)
		t.propagateEqLits(dep, lits, Y, Y1Y2, true)
	}
	return true
}

func (t *SMTTheorySeq) setEmpty(x ast.ASTHandle) bool {
	// GCC evaluates call arguments right to left.
	empty := t.mkEqEmpty(x, true)
	lenX := t.mkLength(x)
	t.addAxiom(t.mkEq(t.mkInt(0), lenX, false).Negated(), empty)
	return true
}

// enforceLengths is enforce_length(es, len).
func (t *SMTTheorySeq) enforceLengths(es []ast.ASTHandle) ([]*big.Rat, bool) {
	allHaveLength := true
	var lens []*big.Rat
	for _, e := range es {
		if t.isUnit(e) {
			lens = append(lens, big.NewRat(1, 1))
		} else if t.isEmpty(e) {
			lens = append(lens, new(big.Rat))
		} else if s, ok := t.isString(e); ok {
			lens = append(lens, big.NewRat(int64(len(s)), 1))
		} else if val, ok := t.getLengthValue(e); ok {
			lens = append(lens, val)
		} else {
			t.enforceLengthNode(t.ensureEnode(e))
			allHaveLength = false
		}
	}
	return lens, allHaveLength
}

func (t *SMTTheorySeq) branchVariable() bool {
	sz := t.eqs.len()
	start := int(t.state.random.Next())
	for i := 0; i < sz; i++ {
		k := (i + start) % sz
		if t.branchVariableEq(t.eqs.at(k)) {
			return true
		}
	}
	return t.state.inconsistent
}

func (t *SMTTheorySeq) branchVariableEq(e *seqEq) bool {
	id := e.id
	s := t.findBranchStart(2 * id)
	found := t.findBranchCandidate(&s, e.dep, e.ls.handles(), e.rs.handles())
	t.insertBranchStart(2*id, s)
	if found {
		return true
	}
	s = t.findBranchStart(2*id + 1)
	found = t.findBranchCandidate(&s, e.dep, e.rs.handles(), e.ls.handles())
	t.insertBranchStart(2*id+1, s)
	return found
}

func (t *SMTTheorySeq) insertBranchStart(k uint32, s int) {
	t.branchStart[k] = s
	t.pushTrail(func(th *SMTTheorySeq) error {
		delete(th.branchStart, k) // pop_branch
		return nil
	})
}

func (t *SMTTheorySeq) findBranchStart(k uint32) int {
	if s, ok := t.branchStart[k]; ok {
		return s
	}
	return 0
}

func (t *SMTTheorySeq) findBranchCandidate(start *int, dep *seqDep, ls, rs []ast.ASTHandle) bool {
	if len(ls) == 0 {
		return false
	}
	l := ls[0]
	if !t.isVar(l) {
		return false
	}
	// expr_ref v0(m): each assignment releases the previous value.
	v0R := t.exprRef(ast.ASTHandle{})
	defer func() { t.chk(v0R.release()) }()
	t.chk(v0R.set(t.mkEmpty(t.sortOf(l))))
	if t.canBeEqual(ls[1:], rs) {
		if t.assumeEquality(l, v0R.h) != lbool.False {
			return true
		}
	}
	for ; *start < len(rs); *start++ {
		j := *start
		if l == rs[j] {
			return false
		}
		if !t.canBeEqual(ls[1:], rs[j+1:]) {
			continue
		}
		t.chk(v0R.set(t.mkConcatN(rs[:j+1])))
		if t.assumeEquality(l, v0R.h) != lbool.False {
			*start++
			return true
		}
	}

	allUnits := true
	for j := 0; allUnits && j < len(rs); j++ {
		allUnits = allUnits && t.isUnit(rs[j])
	}
	if allUnits {
		lits := []SMTLiteral{t.mkEqEmpty(l, true).Negated()}
		for i := 0; i < len(rs); i++ {
			if t.canBeEqual(ls[1:], rs[i+1:]) {
				t.chk(v0R.set(t.mkConcatN(rs[:i+1])))
				lits = append(lits, t.mkEq(l, v0R.h, false).Negated())
			}
		}
		for _, lit := range lits {
			switch t.assignment(lit) {
			case lbool.True:
			case lbool.False:
				*start = 0
				return true
			default:
				t.forcePhase(lit.Negated())
				*start = 0
				return true
			}
		}
		t.setConflict(dep, lits)
		return true
	}
	return false
}

func (t *SMTTheorySeq) canBeEqual(ls, rs []ast.ASTHandle) bool {
	i := 0
	for ; i < len(ls) && i < len(rs); i++ {
		if t.areDistinct(ls[i], rs[i]) {
			return false
		}
		if !t.areEqual(ls[i], rs[i]) {
			break
		}
	}
	if i == len(rs) {
		ls, rs = rs, ls
	}
	if i == len(ls) && i < len(rs) {
		for ; i < len(rs); i++ {
			if t.isUnit(rs[i]) {
				return false
			}
		}
	}
	return true
}

func (t *SMTTheorySeq) assumeEquality(l, r ast.ASTHandle) lbool.LBool {
	if t.mustBool(t.exclude.contains(l, r)) {
		return lbool.False
	}
	x := t.loc()
	defer x.drop()
	eq := t.ref(x, t.mkEqExpr(l, r), nil)
	eq = t.rewriteTh(x, eq)
	if t.m.IsTrue(eq) {
		return lbool.True
	}
	if t.m.IsFalse(eq) {
		return lbool.False
	}
	n1 := t.ensureEnode(l)
	n2 := t.ensureEnode(r)
	if n1.Root() == n2.Root() {
		return lbool.True
	}
	if t.state.context.isDisequal(n1, n2) {
		return lbool.False
	}
	t.chk(t.state.markRelevant(n1.Owner()))
	t.chk(t.state.markRelevant(n2.Owner()))
	accepted, err := t.state.assumeEquality(n1, n2)
	t.chk(err)
	if !accepted {
		return lbool.False
	}
	return t.assignment(t.mkEq(l, r, false))
}

func (t *SMTTheorySeq) propagateLengthCoherence(e ast.ASTHandle) bool {
	if !t.isVar(e) || !t.rep.isRoot(e) {
		return false
	}
	lo, ok := t.lowerBound(e)
	if !ok || lo.Sign() <= 0 || lo.Cmp(big.NewRat(2048, 1)) >= 0 {
		return false
	}
	x := t.loc()
	defer x.drop()
	// expr_ref head(m), tail(m) at function scope; then seq and elems.
	head, tailR := t.exprRef(ast.ASTHandle{}), t.exprRef(ast.ASTHandle{})
	defer func() { t.chk(errors.Join(tailR.release(), head.release())) }()
	seqR := t.exprRef(e)
	elems := newSeqExprVector(t.m)
	defer func() { t.chk(errors.Join(elems.reset(), seqR.release())) }()
	_lo := lo.Num().Int64()
	for j := int64(0); j < _lo; j++ {
		t.mkDecomposeRef(seqR.h, &head, &tailR)
		t.chk(elems.pushBack(head.h))
		t.chk(seqR.set(tailR.h))
	}
	seq := seqR.h
	emp := t.ref(x, t.mkEmpty(t.sortOf(e)), nil)
	t.chk(elems.pushBack(seq))
	t.chk(tailR.set(t.mkConcatN(elems.handles())))
	tail := tailR.h
	// len(e) >= low => e = tail;
	loNum := t.mkIntRat(lo) // arguments right to left
	low := t.mkLiteral(t.mkGe(t.mkLength(e), loNum))
	t.addAxiom(low.Negated(), t.mkSeqEq(e, tail))
	if hi, ok := t.upperBound(e); ok {
		// len(e) <= hi => len(tail) <= hi - lo
		hiNum := t.mkIntRat(hi) // arguments right to left
		high1 := t.ref(x, t.mkLe(t.mkLength(e), hiNum), nil)
		if hi.Cmp(lo) == 0 {
			eqEmp := t.mkSeqEq(seq, emp) // arguments right to left
			t.addAxiom(t.mkLiteral(high1).Negated(), eqEmp)
		} else {
			diff := t.mkIntRat(new(big.Rat).Sub(hi, lo)) // arguments right to left
			high2 := t.ref(x, t.mkLe(t.mkLength(seq), diff), nil)
			lit2 := t.mkLiteral(high2)
			t.addAxiom(t.mkLiteral(high1).Negated(), lit2)
		}
	} else {
		t.assumeEquality(seq, emp)
	}
	return true
}

func (t *SMTTheorySeq) checkLengthCoherenceE(e ast.ASTHandle) bool {
	if t.isVar(e) && t.rep.isRoot(e) {
		if !t.checkLengthCoherence0(e) {
			x := t.loc()
			defer x.drop()
			emp := t.ref(x, t.mkEmpty(t.sortOf(e)), nil)
			// e = emp \/ e = unit(head.elem(e))*tail(e)
			head, tail := t.mkDecompose(x, e)
			conc := t.ref(x, t.mkConcat2(head, tail), nil)
			if t.propagateIsConc(e, conc) {
				t.assumeEquality(tail, emp)
			}
		}
		return true
	}
	return false
}

func (t *SMTTheorySeq) checkLengthCoherence0(e ast.ASTHandle) bool {
	if t.isVar(e) && t.rep.isRoot(e) {
		x := t.loc()
		defer x.drop()
		emp := t.ref(x, t.mkEmpty(t.sortOf(e)), nil)
		if t.propagateLengthCoherence(e) || t.assumeEquality(e, emp) != lbool.False {
			if !t.atBaseLevel() {
				t.pushReplay(seqReplayLengthCoherence, e)
			}
			return true
		}
	}
	return false
}

func (t *SMTTheorySeq) checkLengthCoherence() bool {
	for _, e := range t.length.elems() {
		if t.checkLengthCoherence0(e) {
			return true
		}
	}
	for _, e := range t.length.elems() {
		if t.checkLengthCoherenceE(e) {
			return true
		}
	}
	return false
}

func (t *SMTTheorySeq) fixedLengthAll() bool {
	found := false
	for _, e := range t.length.elems() {
		if t.fixedLength(e) {
			found = true
		}
	}
	return found
}

func (t *SMTTheorySeq) fixedLength(e ast.ASTHandle) bool {
	if !t.isVar(e) {
		return false
	}
	lo, ok := t.lowerBound(e)
	if !ok {
		return false
	}
	hi, ok := t.upperBound(e)
	if !ok || lo.Cmp(hi) != 0 || !lo.IsInt() || lo.Sign() < 0 || lo.Num().BitLen() > 32 {
		return false
	}
	if t.isSkolem(t.tail, e) || t.isSkolem(t.seqFirst, e) ||
		t.isSkolem(t.indexofLeft, e) || t.isSkolem(t.indexofRight, e) ||
		t.mustBool(t.fixed.contains(e)) {
		return false
	}
	t.insertObjTrail(&t.fixed, e)

	x := t.loc()
	defer x.drop()
	// expr_ref seq(e, m), head(m), tail(m);
	seqR, head, tail := t.exprRef(e), t.exprRef(ast.ASTHandle{}), t.exprRef(ast.ASTHandle{})
	defer func() { t.chk(errors.Join(tail.release(), head.release(), seqR.release())) }()
	if lo.Sign() == 0 {
		t.chk(seqR.set(t.mkEmpty(t.sortOf(e))))
	} else {
		_lo := lo.Num().Int64()
		elems := newSeqExprVector(t.m)
		for j := int64(0); j < _lo; j++ {
			t.mkDecomposeRef(seqR.h, &head, &tail)
			t.chk(elems.pushBack(head.h))
			t.chk(seqR.set(tail.h))
		}
		t.chk(seqR.set(t.mkConcatN(elems.handles())))
		t.chk(elems.reset())
	}
	seq := seqR.h
	// GCC evaluates call arguments right to left.
	eqSeq := t.mkSeqEq(seq, e)
	loNum := t.mkIntRat(lo)
	t.addAxiom(t.mkEq(t.mkLength(e), loNum, false).Negated(), eqSeq)
	if !t.atBaseLevel() {
		t.pushReplay(seqReplayFixedLength, e)
	}
	return true
}

// propagateNonEmpty: lit => s != ""
func (t *SMTTheorySeq) propagateNonEmpty(lit SMTLiteral, s ast.ASTHandle) {
	t.propagateLit(nil, []SMTLiteral{lit}, t.mkEqEmpty(s, true).Negated())
}

func (t *SMTTheorySeq) propagateIsConc(e, conc ast.ASTHandle) bool {
	lit := t.mkEqEmpty(e, true).Negated()
	if t.assignment(lit) == lbool.True {
		t.propagateLit(nil, []SMTLiteral{lit}, t.mkEq(e, conc, false))
		x := t.loc()
		defer x.drop()
		e1 := t.ref(x, e, nil)
		e2 := t.ref(x, conc, nil)
		t.newEqEhDep(t.dm.mkLeaf(seqAssumption{lit: lit}), t.enodeOf(e1), t.enodeOf(e2))
		return true
	}
	return false
}

func (t *SMTTheorySeq) isNth(e ast.ASTHandle) bool { return t.isSkolem(t.nth, e) }

func (t *SMTTheorySeq) isNth2(e ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool) {
	if t.isNth(e) {
		return t.arg(e, 0), t.arg(e, 1), true
	}
	return ast.ASTHandle{}, ast.ASTHandle{}, false
}

// isTailIdx is is_tail(e, s, idx).
func (t *SMTTheorySeq) isTailIdx(e ast.ASTHandle) (ast.ASTHandle, int64, bool) {
	if !t.isSkolem(t.tail, e) {
		return ast.ASTHandle{}, 0, false
	}
	r, ok := t.isNumeral(t.arg(e, 1))
	if !ok {
		return ast.ASTHandle{}, 0, false
	}
	return t.arg(e, 0), r.Num().Int64(), true
}

func (t *SMTTheorySeq) isEqSk(e ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool) {
	if t.isSkolem(t.eqSym, e) {
		return t.arg(e, 0), t.arg(e, 1), true
	}
	return ast.ASTHandle{}, ast.ASTHandle{}, false
}

func (t *SMTTheorySeq) isPre(e ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool) {
	if t.isSkolem(t.pre, e) {
		return t.arg(e, 0), t.arg(e, 1), true
	}
	return ast.ASTHandle{}, ast.ASTHandle{}, false
}

func (t *SMTTheorySeq) isPost(e ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool) {
	if t.isSkolem(t.post, e) {
		return t.arg(e, 0), t.arg(e, 1), true
	}
	return ast.ASTHandle{}, ast.ASTHandle{}, false
}

func (t *SMTTheorySeq) mkNth(s, idx ast.ASTHandle) ast.ASTHandle {
	charSort := t.seqElemSort(t.sortOf(s))
	return t.mkSkolem(t.nth, s, idx, ast.ASTHandle{}, charSort)
}

func (t *SMTTheorySeq) mkSkIte(c, th, el ast.ASTHandle) ast.ASTHandle {
	return t.mkSkolem(t.symbol("seq.if"), c, th, el, t.sortOf(th))
}

func (t *SMTTheorySeq) mkLast(s ast.ASTHandle) ast.ASTHandle {
	if str, ok := t.isString(s); ok && len(str) > 0 {
		return t.mkChar(str, len(str)-1)
	}
	charSort := t.seqElemSort(t.sortOf(s))
	return t.mkSkolem(t.seqLast, s, ast.ASTHandle{}, ast.ASTHandle{}, charSort)
}

func (t *SMTTheorySeq) mkFirst(s ast.ASTHandle) ast.ASTHandle {
	if str, ok := t.isString(s); ok && len(str) > 0 {
		return t.mkString(str[:len(str)-1])
	}
	return t.mkSkolem(t.seqFirst, s, ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{})
}

// mkDecompose is mk_decompose(e, head, tail). head and tail are expr_ref
// out-parameters, held in the caller's locals x.
func (t *SMTTheorySeq) exprRef(h ast.ASTHandle) seqExprRef {
	r, err := newSeqExprRef(t.m, h)
	t.chk(err)
	return r
}

// mkDecomposeRef is mk_decompose(e, head, tail) assigning expr_ref outputs.
func (t *SMTTheorySeq) mkDecomposeRef(e ast.ASTHandle, head, tail *seqExprRef) {
	if t.isEmpty(e) {
		t.chk(head.set(t.mkUnit(t.mkNth(e, t.mkInt(0)))))
		t.chk(tail.set(e))
	} else if s, ok := t.isString(e); ok {
		t.chk(head.set(t.mkUnit(t.mkChar(s, 0))))
		t.chk(tail.set(t.mkString(s[1:])))
	} else if t.isUnit(e) {
		t.chk(head.set(e))
		t.chk(tail.set(t.mkEmpty(t.sortOf(e))))
	} else if e1, e2, ok := t.isConcat2(e); ok && t.isUnit(e1) {
		t.chk(head.set(e1))
		t.chk(tail.set(e2))
	} else if t.isSkolem(t.tail, e) {
		s := t.arg(e, 0)
		r, _ := t.isNumeral(t.arg(e, 1))
		idx := t.mkInt(r.Num().Int64() + 1)
		t.chk(head.set(t.mkUnit(t.mkNth(s, idx))))
		t.chk(tail.set(t.mkSkolem(t.tail, s, idx, ast.ASTHandle{}, ast.ASTHandle{})))
	} else {
		t.chk(head.set(t.mkUnit(t.mkNth(e, t.mkInt(0)))))
		t.chk(tail.set(t.mkSkolem(t.tail, e, t.mkInt(0), ast.ASTHandle{}, ast.ASTHandle{})))
	}
}

func (t *SMTTheorySeq) mkDecompose(x *seqRefs, e ast.ASTHandle) (head, tail ast.ASTHandle) {
	if t.isEmpty(e) {
		head = t.ref(x, t.mkUnit(t.mkNth(e, t.mkInt(0))), nil)
		tail = t.ref(x, e, nil)
	} else if s, ok := t.isString(e); ok {
		head = t.ref(x, t.mkUnit(t.mkChar(s, 0)), nil)
		tail = t.ref(x, t.mkString(s[1:]), nil)
	} else if t.isUnit(e) {
		head = t.ref(x, e, nil)
		tail = t.ref(x, t.mkEmpty(t.sortOf(e)), nil)
	} else if e1, e2, ok := t.isConcat2(e); ok && t.isUnit(e1) {
		head = t.ref(x, e1, nil)
		tail = t.ref(x, e2, nil)
	} else if t.isSkolem(t.tail, e) {
		s := t.arg(e, 0)
		r, _ := t.isNumeral(t.arg(e, 1))
		idx := t.mkInt(r.Num().Int64() + 1)
		head = t.ref(x, t.mkUnit(t.mkNth(s, idx)), nil)
		tail = t.ref(x, t.mkSkolem(t.tail, s, idx, ast.ASTHandle{}, ast.ASTHandle{}), nil)
	} else {
		head = t.ref(x, t.mkUnit(t.mkNth(e, t.mkInt(0))), nil)
		tail = t.ref(x, t.mkSkolem(t.tail, e, t.mkInt(0), ast.ASTHandle{}, ast.ASTHandle{}), nil)
	}
	return head, tail
}
