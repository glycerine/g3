package smtcore

// Source: src/smt/theory_seq.cpp:3391-3451 (propagate_in_re) and
// 4166-4794 (get_automaton through get_concat).

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (t *SMTTheorySeq) propagateInRe(n ast.ASTHandle, isTrue bool) {
	s, re, _ := t.binaryArgs(n, seqOpInRe)
	x := t.loc()
	defer x.drop()
	tmp := t.ref(x, n, nil)
	tmp = t.rewriteTh(x, tmp)
	if t.m.IsTrue(tmp) {
		if !isTrue {
			t.setConflict(nil, []SMTLiteral{t.mkLiteral(n)})
		}
		return
	} else if t.m.IsFalse(tmp) {
		if isTrue {
			t.setConflict(nil, []SMTLiteral{t.mkLiteral(n).Negated()})
		}
		return
	}
	e3 := t.ref(x, re, nil)
	lit, err := t.state.Literal(n)
	t.chk(err)
	if !isTrue {
		e3 = t.ref(x, t.must(t.m.MkRegexComplement(re)), nil)
		lit = lit.Negated()
	}
	a := t.getAutomaton(e3)
	if a == nil {
		return
	}
	g, err := a.Graph()
	t.chk(err)
	ln := t.ref(x, t.mkLength(s), nil)
	for i := uint32(0); i < g.NumStates(); i++ {
		acc := t.mkAccept(s, ln, e3, t.mkInt(int64(i)))
		rej := t.mkReject(s, ln, e3, t.mkInt(int64(i)))
		if g.IsFinal(i) {
			t.addAxiom(acc)
			t.addAxiom(rej.Negated())
		} else {
			t.addAxiom(acc.Negated())
			t.addAxiom(rej)
		}
	}
	zero := t.ref(x, t.mkInt(0), nil)
	states := g.EpsilonClosure(g.Initial())
	lits := []SMTLiteral{lit.Negated()}
	for _, st := range states {
		lits = append(lits, t.mkAccept(s, zero, e3, t.mkInt(int64(st))))
	}
	if len(lits) == 2 {
		t.propagateLit(nil, []SMTLiteral{lit}, lits[1])
	} else {
		t.chk(t.state.addTheoryAxiom(SMTTheoryID(t.FamilyID()), lits))
	}
}

// seqExprSolver returns the theory's seq_expr_solver, created by init.
func (t *SMTTheorySeq) seqExprSolver() rewriter.SequenceExpressionSolver {
	if t.exprSolver == nil {
		return nil
	}
	return t.exprSolver
}

func (t *SMTTheorySeq) getAutomaton(re ast.ASTHandle) *rewriter.StringRegexAutomaton {
	if result, ok := t.re2aut[re]; ok {
		return result
	}
	result, err := rewriter.NewSeqAutomaton(t.m, re, t.seqExprSolver())
	t.chk(err)
	if result != nil {
		t.traceAutomaton(result)
	}
	t.automata = append(t.automata, result)
	t.re2aut[re] = result
	t.chk(t.res.pushBack(re))
	return result
}

// closeAutomata is m_automata.reset().
func (t *SMTTheorySeq) closeAutomata() {
	for _, a := range t.automata {
		if a != nil {
			t.chk(a.Close())
		}
	}
	t.automata = t.automata[:0]
}

func (t *SMTTheorySeq) mkAccept(s, idx, re, state ast.ASTHandle) SMTLiteral {
	boolSort, err := t.m.MkBoolSort()
	t.chk(err)
	return t.mkLiteral(t.must(t.m.MkSequenceSkolem(t.accept, []ast.ASTHandle{s, idx, re, state}, boolSort)))
}

func (t *SMTTheorySeq) mkReject(s, idx, re, state ast.ASTHandle) SMTLiteral {
	boolSort, err := t.m.MkBoolSort()
	t.chk(err)
	return t.mkLiteral(t.must(t.m.MkSequenceSkolem(t.reject, []ast.ASTHandle{s, idx, re, state}, boolSort)))
}

type seqAccRej struct {
	s, idx, re ast.ASTHandle
	i          uint32
	aut        *rewriter.StringRegexAutomaton
}

func (t *SMTTheorySeq) isAccRej(name usymbol.Symbol, e ast.ASTHandle) (seqAccRej, bool) {
	if !t.isSkolem(name, e) {
		return seqAccRej{}, false
	}
	r, _ := t.isNumeral(t.arg(e, 3))
	re := t.arg(e, 2)
	return seqAccRej{s: t.arg(e, 0), idx: t.arg(e, 1), re: re, i: uint32(r.Num().Uint64()), aut: t.getAutomaton(re)}, true
}

func (t *SMTTheorySeq) isAccept(e ast.ASTHandle) bool { return t.isSkolem(t.accept, e) }
func (t *SMTTheorySeq) isReject(e ast.ASTHandle) bool { return t.isSkolem(t.reject, e) }
func (t *SMTTheorySeq) isStep(e ast.ASTHandle) bool   { return t.isSkolem(t.step, e) }

// isStep6 is is_step(e, s, idx, re, i, j, t).
func (t *SMTTheorySeq) isStep6(e ast.ASTHandle) (s, idx, re, i, j, acc ast.ASTHandle, ok bool) {
	if !t.isStep(e) {
		return
	}
	return t.arg(e, 0), t.arg(e, 1), t.arg(e, 2), t.arg(e, 3), t.arg(e, 4), t.arg(e, 5), true
}

// mkStep is mk_step(s, idx, re, i, j, acc), held in x.
func (t *SMTTheorySeq) mkStep(x *seqRefs, s, idx, re ast.ASTHandle, i, j uint32, acc ast.ASTHandle) ast.ASTHandle {
	boolSort, err := t.m.MkBoolSort()
	t.chk(err)
	args := []ast.ASTHandle{s, idx, re, t.mkInt(int64(i)), t.mkInt(int64(j)), acc}
	return t.ref(x, t.must(t.m.MkSequenceSkolem(t.step, args, boolSort)), nil)
}

// isSinkState is automaton::is_sink_state.
func (t *SMTTheorySeq) isSinkState(a *rewriter.StringRegexAutomaton, s uint32) bool {
	g, err := a.Graph()
	t.chk(err)
	if g.IsFinal(s) {
		return false
	}
	for _, mv := range g.ExpandedMovesFrom(s, true) {
		if mv.Destination != s {
			return false
		}
	}
	return true
}

// propagateAccRejLength:
//
//	acc(s, idx, re, i) -> len(s) >= idx    if i is final
//	rej(s, idx, re, i) -> len(s) >= idx    if i is non-final
//	acc(s, idx, re, i) -> len(s) > idx     if i is non-final
//	rej(s, idx, re, i) -> len(s) > idx     if i is final
func (t *SMTTheorySeq) propagateAccRejLength(lit SMTLiteral, e ast.ASTHandle) {
	ar, isAcc := t.isAccRej(t.accept, e)
	if !isAcc {
		ar, _ = t.isAccRej(t.reject, e)
	}
	if t.isStrOp(ar.idx, seqOpLength) {
		return
	}
	if t.isSinkState(ar.aut, ar.i) {
		t.propagateLit(nil, []SMTLiteral{lit}, SMTFalseLiteral)
		return
	}
	g, err := ar.aut.Graph()
	t.chk(err)
	if g.IsFinal(ar.i) == isAcc {
		t.propagateLit(nil, []SMTLiteral{lit}, t.mkLiteral(t.mkGe(t.mkLength(ar.s), ar.idx)))
	} else {
		t.propagateLit(nil, []SMTLiteral{lit}, t.mkLiteral(t.mkLe(t.mkLength(ar.s), ar.idx)).Negated())
	}
}

func (t *SMTTheorySeq) exprLiteral(e ast.ASTHandle) SMTLiteral {
	l, err := t.state.Literal(e)
	t.chk(err)
	return l
}

// addAccept2step:
//
//	acc(s, idx, re, i) ->  \/ step(s, idx, re, i, j, t)                if i is non-final
//	acc(s, idx, re, i) -> len(s) <= idx \/ step(s, idx, re, i, j, t)   if i is final
func (t *SMTTheorySeq) addAccept2step(acc ast.ASTHandle, change *bool) bool {
	ar, _ := t.isAccRej(t.accept, acc)
	if ar.aut == nil || t.isStrOp(ar.idx, seqOpLength) {
		return false
	}
	g, err := ar.aut.Graph()
	t.chk(err)
	mvs := g.ExpandedMovesFrom(ar.i, true)
	x := t.loc()
	defer x.drop()
	ln := t.ref(x, t.mkLength(ar.s), nil)
	lits := []SMTLiteral{t.exprLiteral(acc).Negated()}
	if g.IsFinal(ar.i) {
		lits = append(lits, t.mkLiteral(t.mkLe(ln, ar.idx)))
		switch t.assignment(lits[len(lits)-1]) {
		case lbool.True:
			return false
		case lbool.Undef:
			*change = true
			t.forcePhase(lits[len(lits)-1])
			return true
		}
	}
	hasUndef := false
	start := int(t.state.random.Next())
	for i := range mvs {
		j := (i + start) % len(mvs)
		mv := mvs[j]
		nth := t.ref(x, t.mkNth(ar.s, ar.idx), nil)
		a := t.ref(x, t.must(ar.aut.Accept(mv.Condition, nth)), nil)
		step := t.mkStep(x, ar.s, ar.idx, ar.re, ar.i, mv.Destination, a)
		lits = append(lits, t.mkLiteral(step))
		switch t.assignment(lits[len(lits)-1]) {
		case lbool.True:
			return false
		case lbool.Undef:
			hasUndef = true
		}
	}
	*change = true
	if hasUndef && len(mvs) == 1 {
		lit := lits[len(lits)-1]
		lits = lits[:len(lits)-1]
		for i := range lits {
			lits[i] = lits[i].Negated()
		}
		t.propagateLit(nil, lits, lit)
		return false
	}
	if hasUndef {
		return true
	}
	for i := range lits {
		lits[i] = lits[i].Negated()
	}
	t.setConflict(nil, lits)
	return false
}

// addStep2accept: acc(s, idx, re, i) & step(s, idx, re, i, j, t) => acc(s, idx + 1, re, j)
func (t *SMTTheorySeq) addStep2accept(step ast.ASTHandle, change *bool) bool {
	s, idx, re, i, j, _, _ := t.isStep6(step)
	acc1 := t.mkAccept(s, idx, re, i)
	switch t.assignment(acc1) {
	case lbool.Undef:
		*change = true
		return true
	case lbool.True:
		*change = true
		r, _ := t.isNumeral(idx)
		x := t.loc()
		defer x.drop()
		idx1 := t.ref(x, t.mkInt(r.Num().Int64()+1), nil)
		acc2 := t.mkAccept(s, idx1, re, j)
		lits := []SMTLiteral{acc1, t.exprLiteral(step), acc2.Negated()}
		switch t.assignment(acc2) {
		case lbool.Undef:
			t.propagateLit(nil, lits[:2], acc2)
		case lbool.False:
			t.setConflict(nil, lits)
		}
	}
	return false
}

// addReject2reject:
//
//	rej(s, idx, re, i) & nth(s, idx) = t & idx < len(s) => rej(s, idx + 1, re, j)
func (t *SMTTheorySeq) addReject2reject(rej ast.ASTHandle, change *bool) bool {
	ar, _ := t.isAccRej(t.reject, rej)
	if ar.aut == nil || t.isStrOp(ar.idx, seqOpLength) {
		return false
	}
	r, _ := t.isNumeral(ar.idx)
	x := t.loc()
	defer x.drop()
	idx1 := t.ref(x, t.mkInt(r.Num().Int64()+1), nil)
	g, err := ar.aut.Graph()
	t.chk(err)
	mvs := g.ExpandedMovesFrom(ar.i, true)
	rej1 := t.exprLiteral(rej)
	ln := t.ref(x, t.mkLength(ar.s), nil)
	lenLeIdx := t.mkLiteral(t.mkLe(ln, ar.idx))
	switch t.assignment(lenLeIdx) {
	case lbool.True:
		return false
	case lbool.Undef:
		t.forcePhase(lenLeIdx)
		return true
	}
	nth := t.ref(x, t.mkNth(ar.s, ar.idx), nil)
	t.ensureNth(lenLeIdx.Negated(), ar.s, ar.idx)
	var eqs []SMTLiteral
	hasUndef := false
	for _, mv := range mvs {
		eq := t.mkLiteral(t.must(ar.aut.Accept(mv.Condition, nth)))
		if t.assignment(eq) == lbool.Undef {
			t.forcePhase(eq.Negated())
			hasUndef = true
		}
		eqs = append(eqs, eq)
	}
	*change = true
	if hasUndef {
		return true
	}
	for i, mv := range mvs {
		eq := eqs[i]
		if t.assignment(eq) == lbool.True {
			rej2 := t.mkReject(ar.s, idx1, ar.re, t.mkInt(int64(mv.Destination)))
			t.addAxiom(rej1.Negated(), eq.Negated(), lenLeIdx, rej2)
		}
	}
	return false
}

// propagateNotSuffix:
//
//	!suffix(e1,e2) => e1 != ""
//	!suffix(e1,e2) => e2 = "" or e1 = ycx & (e2 = zdx & c != d or x = e2)
func (t *SMTTheorySeq) propagateNotSuffix(e ast.ASTHandle) {
	e1, e2, _ := t.binaryArgs(e, seqOpSuffix)
	lit := t.exprLiteral(e)
	if t.canonizes(false, e) {
		return
	}
	t.propagateNonEmpty(lit.Negated(), e1)
	x0 := t.loc()
	defer x0.drop()
	emp := t.ref(x0, t.mkEmpty(t.sortOf(e1)), nil)
	e2IsEmp := t.mkSeqEq(e2, emp)
	charSort := t.seqElemSort(t.sortOf(e1))
	null := ast.ASTHandle{}
	x := t.ref(x0, t.mkSkolem(t.symbol("seq.suffix.x"), e1, e2, null, null), nil)
	y := t.ref(x0, t.mkSkolem(t.symbol("seq.suffix.y"), e1, e2, null, null), nil)
	z := t.ref(x0, t.mkSkolem(t.symbol("seq.suffix.z"), e1, e2, null, null), nil)
	c := t.ref(x0, t.mkSkolem(t.symbol("seq.suffix.c"), e1, e2, null, charSort), nil)
	d := t.ref(x0, t.mkSkolem(t.symbol("seq.suffix.d"), e1, e2, null, charSort), nil)
	t.addAxiom(lit, e2IsEmp, t.mkSeqEq(e1, t.mkConcat3(y, t.mkUnit(c), x)))
	// GCC evaluates call arguments right to left.
	eqX := t.mkSeqEq(e2, x)
	t.addAxiom(lit, e2IsEmp, t.mkSeqEq(e2, t.mkConcat3(z, t.mkUnit(d), x)), eqX)
	eqX = t.mkSeqEq(e2, x)
	t.addAxiom(lit, e2IsEmp, t.mkEq(c, d, false).Negated(), eqX)
}

// addPrefix2prefix: !prefix -> e2 = emp \/ nth(e1,0) != nth(e2,0) \/ !prefix(tail(e1),tail(e2))
func (t *SMTTheorySeq) addPrefix2prefix(e ast.ASTHandle, change *bool) bool {
	e1, e2, _ := t.binaryArgs(e, seqOpPrefix)
	if t.canonizes(false, e) {
		return false
	}
	x := t.loc()
	defer x.drop()
	e2IsEmp := t.mkEqEmpty(e2, true)
	switch t.assignment(e2IsEmp) {
	case lbool.True:
		return false // done
	case lbool.Undef:
		return true // retry
	}
	head2, tail2 := t.mkDecompose(x, e2)
	conc := t.ref(x, t.mkConcat2(head2, tail2), nil)
	t.propagateEqLitOnly(e2IsEmp.Negated(), e2, conc, true)

	e1IsEmp := t.mkEqEmpty(e1, false)
	switch t.assignment(e1IsEmp) {
	case lbool.True:
		t.addAxiom(t.exprLiteral(e), e1IsEmp.Negated())
		return false // done
	case lbool.Undef:
		return true // retry
	}
	head1, tail1 := t.mkDecompose(x, e1)
	conc = t.ref(x, t.mkConcat2(head1, tail1), nil)
	t.propagateEqLitOnly(e1IsEmp.Negated(), e1, conc, true)

	lit := t.mkEq(head1, head2, false)
	switch t.assignment(lit) {
	case lbool.False:
		return false
	case lbool.Undef:
		t.forcePhase(lit.Negated())
		return true
	}
	*change = true
	lits := []SMTLiteral{t.exprLiteral(e).Negated(), e2IsEmp.Negated(), lit}
	t.propagateLit(nil, lits, t.mkLiteral(t.must(t.m.MkSequencePrefix(tail1, tail2))).Negated())
	return false
}

// addSuffix2suffix: !suffix(e1, e2) -> e2 = emp \/ last(e1) != last(e2) \/ !suffix(first(e1), first(e2))
func (t *SMTTheorySeq) addSuffix2suffix(e ast.ASTHandle, change *bool) bool {
	e1, e2, _ := t.binaryArgs(e, seqOpSuffix)
	if t.canonizes(false, e) {
		return false
	}
	x := t.loc()
	defer x.drop()
	e2IsEmp := t.mkEqEmpty(e2, true)
	switch t.assignment(e2IsEmp) {
	case lbool.True:
		return false // done
	case lbool.Undef:
		t.forcePhase(e2IsEmp)
		return true // retry
	}
	first2 := t.ref(x, t.mkFirst(e2), nil)
	last2 := t.ref(x, t.mkLast(e2), nil)
	conc2 := t.ref(x, t.mkConcat2(first2, t.mkUnit(last2)), nil)
	t.propagateEqLitOnly(e2IsEmp.Negated(), e2, conc2, true)

	e1IsEmp := t.mkEqEmpty(e1, true)
	switch t.assignment(e1IsEmp) {
	case lbool.True:
		return false // done
	case lbool.Undef:
		t.forcePhase(e1IsEmp)
		return true // retry
	}
	first1 := t.ref(x, t.mkFirst(e1), nil)
	last1 := t.ref(x, t.mkLast(e1), nil)
	conc1 := t.ref(x, t.mkConcat2(first1, t.mkUnit(last1)), nil)
	t.propagateEqLitOnly(e1IsEmp.Negated(), e1, conc1, true)

	lastEq := t.mkEq(last1, last2, false)
	switch t.assignment(lastEq) {
	case lbool.False:
		return false // done
	case lbool.Undef:
		t.forcePhase(lastEq.Negated())
		return true
	}
	*change = true
	lits := []SMTLiteral{t.exprLiteral(e).Negated(), e2IsEmp.Negated(), lastEq}
	t.propagateLit(nil, lits, t.mkLiteral(t.must(t.m.MkSequenceSuffix(first1, first2))).Negated())
	return false
}

func (t *SMTTheorySeq) canonizes(sign bool, e ast.ASTHandle) bool {
	var deps *seqDep
	x := t.loc()
	defer x.drop()
	cont := t.canonize(x, e, &deps)
	if (t.m.IsTrue(cont) && !sign) || (t.m.IsFalse(cont) && sign) {
		t.propagateLit(deps, nil, t.exprLiteral(e))
		return true
	}
	if (t.m.IsFalse(cont) && !sign) || (t.m.IsTrue(cont) && sign) {
		return true
	}
	return false
}

// addContains2contains:
//
//	!contains(e1, e2) -> !prefix(e2, e1)
//	!contains(e1, e2) -> e1 = emp \/ !contains(tail(e1), e2)
func (t *SMTTheorySeq) addContains2contains(e ast.ASTHandle, change *bool) bool {
	e1, e2, _ := t.binaryArgs(e, seqOpContains)
	if t.canonizes(false, e) {
		return false
	}
	e1IsEmp := t.mkEqEmpty(e1, true)
	switch t.assignment(e1IsEmp) {
	case lbool.True:
		return false // done
	case lbool.Undef:
		t.forcePhase(e1IsEmp)
		return true // retry
	}
	*change = true
	x := t.loc()
	defer x.drop()
	head, tail := t.mkDecompose(x, e1)
	conc := t.ref(x, t.mkConcat2(head, tail), nil)
	t.propagateEqLitOnly(e1IsEmp.Negated(), e1, conc, true)
	lits := []SMTLiteral{t.exprLiteral(e).Negated(), e1IsEmp.Negated()}
	t.propagateLit(nil, lits, t.mkLiteral(t.must(t.m.MkSequenceContains(tail, e2))).Negated())
	return false
}

func (t *SMTTheorySeq) propagateAutomata() bool {
	if t.atomsQhead == len(t.atoms) {
		return false
	}
	head := t.atomsQhead
	t.pushTrail(func(th *SMTTheorySeq) error { // value_trail(m_atoms_qhead)
		th.atomsQhead = head
		return nil
	})
	var reAdd []ast.ASTHandle
	change := false
	for t.atomsQhead < len(t.atoms) && !t.state.inconsistent {
		e := t.atoms[t.atomsQhead]
		reQ := false
		if t.isAccept(e) {
			reQ = t.addAccept2step(e, &change)
		} else if t.isReject(e) {
			reQ = t.addReject2reject(e, &change)
		} else if t.isStep(e) {
			reQ = t.addStep2accept(e, &change)
		} else if t.isStrOp(e, seqOpPrefix) {
			reQ = t.addPrefix2prefix(e, &change)
		} else if t.isStrOp(e, seqOpSuffix) {
			reQ = t.addSuffix2suffix(e, &change)
		} else if t.isStrOp(e, seqOpContains) {
			reQ = t.addContains2contains(e, &change)
		}
		if reQ {
			reAdd = append(reAdd, e)
			change = true
		}
		t.atomsQhead++
	}
	t.atoms = append(t.atoms, reAdd...)
	return change || t.state.inconsistent
}

// getConcatRep is get_concat(e, concats), following m_rep.
func (t *SMTTheorySeq) getConcatRep(e ast.ASTHandle, concats *[]ast.ASTHandle) {
	for {
		e = t.rep.find(e)
		if e1, e2, ok := t.isConcat2(e); ok {
			t.getConcatRep(e1, concats)
			e = e2
			continue
		}
		*concats = append(*concats, e)
		return
	}
}
