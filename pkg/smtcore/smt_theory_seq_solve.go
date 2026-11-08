package smtcore

// Source: src/smt/theory_seq.cpp:1126-1553, check_extensionality through
// propagate_max_length.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (t *SMTTheorySeq) checkExtensionality() bool {
	sz := t.NumVariables()
	var seqs []SMTTheoryVar
	for v := 0; v < sz; v++ {
		n1, err := t.Enode(SMTTheoryVar(v))
		t.chk(err)
		o1 := n1.Owner()
		if n1 != n1.Root() {
			continue
		}
		if len(seqs) != 0 && t.state.isRelevant(o1) && t.isSeq(o1) && t.isShared(n1) {
			x := t.loc()
			var dep *seqDep
			e1 := t.canonize(x, o1, &dep)
			for _, sv := range seqs {
				n2, err := t.Enode(sv)
				t.chk(err)
				o2 := n2.Owner()
				if t.sortOf(o1) != t.sortOf(o2) {
					continue
				}
				if t.state.context.isDisequal(n1, n2) || t.mustBool(t.exclude.contains(o1, o2)) {
					continue
				}
				e2 := t.canonize(x, n2.Owner(), &dep)
				t.chk(t.lhsV.reset())
				t.chk(t.rhsV.reset())
				var lhs, rhs []ast.ASTHandle
				change := false
				ok, err := t.seqRewrite.ReduceEqExpr(e1, e2, &lhs, &rhs, &change)
				t.chk(err)
				t.chk(t.lhsV.append(lhs))
				t.chk(t.rhsV.append(rhs))
				if !ok {
					t.chk(t.exclude.update(o1, o2))
					continue
				}
				excluded := false
				for j := 0; !excluded && j < t.lhsV.size(); j++ {
					if t.mustBool(t.exclude.contains(t.lhsV.get(j), t.rhsV.get(j))) {
						excluded = true
					}
				}
				if excluded {
					continue
				}
				_, err = t.state.assumeEquality(n1, n2)
				t.chk(err)
				x.drop()
				return false
			}
			x.drop()
		}
		seqs = append(seqs, SMTTheoryVar(v))
	}
	return true
}

// isShared is context::is_shared(n); theory_seq keeps theory::is_shared,
// which answers false.
func (t *SMTTheorySeq) isShared(n *SMTEnode) bool {
	shared, _, err := t.state.sharedTheoryNode(n)
	t.chk(err)
	return shared
}

// checkContains checks the negated contains constraints.
func (t *SMTTheorySeq) checkContains() bool {
	for i := 0; !t.state.inconsistent && i < t.ncs.len(); i++ {
		if t.solveNc(i) {
			if i+1 != t.ncs.len() {
				n, err := t.copyNc(t.ncs.at(t.ncs.len() - 1))
				t.chk(err)
				t.chk(t.ncs.set(i, n))
				i--
			}
			t.chk(t.ncs.popBack())
		}
	}
	return t.newPropagation || t.state.inconsistent
}

// isSolved: no equations, every regex compiled, no non-contains left.
func (t *SMTTheorySeq) isSolved() bool {
	if t.eqs.len() != 0 {
		return false
	}
	for _, a := range t.automata {
		if a == nil {
			return false
		}
	}
	if t.ncs.len() != 0 {
		return false
	}
	return true
}

// linearize extracts dependency literals and equalities and reports whether
// all literals are asserted.
func (t *SMTTheorySeq) linearize(dep *seqDep, eqs *[]SMTEnodePair, lits *[]SMTLiteral) bool {
	asserted := true
	for _, a := range t.dm.linearize(dep) {
		if a.lit != SMTNullLiteral {
			*lits = append(*lits, a.lit)
			asserted = asserted && t.assignment(a.lit) == lbool.True
		}
		if a.n1 != nil {
			*eqs = append(*eqs, SMTEnodePair{First: a.n1, Second: a.n2})
		}
	}
	return asserted
}

func (t *SMTTheorySeq) propagateLit(dep *seqDep, _lits []SMTLiteral, lit SMTLiteral) {
	if lit == SMTTrueLiteral {
		return
	}
	lits := append([]SMTLiteral(nil), _lits...)
	if lit == SMTFalseLiteral {
		t.setConflict(dep, lits)
		return
	}
	t.markLitRelevant(lit)
	var eqs []SMTEnodePair
	if !t.linearize(dep, &eqs, &lits) {
		return
	}
	owner, err := NewSMTExtTheoryPropagationJustification(t.m, t.FamilyID(), lits, eqs, lit, nil)
	t.chk(err)
	justification, err := NewSMTBooleanJustification(owner)
	if err != nil {
		t.chk(errors.Join(err, owner.Release(t.m)))
	}
	t.newPropagation = true
	if err = t.state.AssignLiteral(lit, justification); err != nil {
		t.chk(errors.Join(err, owner.Release(t.m)))
	}
	t.state.ownedJustifications = append(t.state.ownedJustifications, owner)
}

func (t *SMTTheorySeq) setConflict(dep *seqDep, _lits []SMTLiteral) {
	var eqs []SMTEnodePair
	lits := append([]SMTLiteral(nil), _lits...)
	if !t.linearize(dep, &eqs, &lits) {
		return
	}
	t.newPropagation = true
	// context::set_conflict keeps the first conflict.
	if t.state.inconsistent {
		return
	}
	owner, err := NewSMTExtTheoryConflictJustification(t.m, t.FamilyID(), lits, eqs, nil)
	t.chk(err)
	wrapped, err := NewSMTBooleanJustification(owner)
	if err != nil {
		t.chk(errors.Join(err, owner.Release(t.m)))
	}
	t.state.ownedJustifications = append(t.state.ownedJustifications, owner)
	t.state.inconsistent = true
	t.state.conflict = wrapped
	t.state.conflictLiteral = SMTNullLiteral
}

// propagateEqNodes is propagate_eq(dep, n1, n2).
func (t *SMTTheorySeq) propagateEqNodes(dep *seqDep, n1, n2 *SMTEnode) {
	if n1.Root() == n2.Root() {
		return
	}
	var lits []SMTLiteral
	var eqs []SMTEnodePair
	if !t.linearize(dep, &eqs, &lits) {
		return
	}
	js, err := NewSMTExtTheoryEqualityPropagationJustification(t.m, t.FamilyID(), lits, eqs, n1, n2, nil)
	t.chk(err)
	t.state.ownedJustifications = append(t.state.ownedJustifications, js)
	t.chk(t.state.assignEquality(n1, n2, NewSMTEqualityJustification(js)))
	t.newPropagation = true
	t.enforceLengthCoherence(n1, n2)
}

// propagateEqDep is propagate_eq(dep, e1, e2, add_eq).
func (t *SMTTheorySeq) propagateEqDep(dep *seqDep, e1, e2 ast.ASTHandle, addEq bool) {
	t.propagateEqLits(dep, nil, e1, e2, addEq)
}

// propagateEqLit is propagate_eq(dep, lit, e1, e2, add_to_eqs).
func (t *SMTTheorySeq) propagateEqLit(dep *seqDep, lit SMTLiteral, e1, e2 ast.ASTHandle, addToEqs bool) {
	t.propagateEqLits(dep, []SMTLiteral{lit}, e1, e2, addToEqs)
}

func (t *SMTTheorySeq) enforceLengthCoherence(n1, n2 *SMTEnode) {
	o1, o2 := n1.Owner(), n2.Owner()
	if t.isConcat(o1) && t.isConcat(o2) {
		return
	}
	if t.hasLength(o1) && !t.hasLength(o2) {
		t.enforceLengthNode(n2)
	} else if t.hasLength(o2) && !t.hasLength(o1) {
		t.enforceLengthNode(n1)
	}
}

// simplifyEq is simplify_eq(ls, rs, deps); ls and rs are m_ls and m_rs.
func (t *SMTTheorySeq) simplifyEq(ls, rs *seqExprVector, deps *seqDep) bool {
	x := t.loc()
	defer x.drop()
	lsS := append([]ast.ASTHandle(nil), ls.handles()...)
	rsS := append([]ast.ASTHandle(nil), rs.handles()...)
	var lhs, rhs []ast.ASTHandle
	changed := false
	ok, err := t.seqRewrite.ReduceEqVec(&lsS, &rsS, &lhs, &rhs, &changed)
	t.chk(err)
	// ls and rs are edited in place by reduce_eq, possibly swapped.
	t.resetExprVector(ls, lsS)
	t.resetExprVector(rs, rsS)
	lhsV, rhsV := newSeqExprVector(t.m), newSeqExprVector(t.m)
	defer func() { t.chk(rhsV.reset()); t.chk(lhsV.reset()) }()
	t.chk(lhsV.append(lhs))
	t.chk(rhsV.append(rhs))
	if !ok {
		// equality is inconsistent.
		t.setConflict(deps, nil)
		return true
	}
	if !changed {
		return false
	}
	lhs, rhs = append([]ast.ASTHandle(nil), lhs...), append([]ast.ASTHandle(nil), rhs...)
	t.chk(t.seqRewrite.AddSeqs(ls.handles(), rs.handles(), &lhs, &rhs))
	t.resetExprVector(&lhsV, lhs)
	t.resetExprVector(&rhsV, rhs)
	if lhsV.empty() {
		return true
	}
	for i := 0; !t.state.inconsistent && i < lhsV.size(); i++ {
		li := t.ref(x, lhsV.get(i), nil)
		ri := t.ref(x, rhsV.get(i), nil)
		if t.solveUnitEq(li, ri, deps) {
			// no-op
		} else if t.isSeq(li) || t.isRe(li) {
			t.eqs.pushBack(t.mkEqdep(li, ri, deps))
		} else {
			rn := t.ensureEnode(ri) // arguments right to left
			t.propagateEqNodes(deps, t.ensureEnode(li), rn)
		}
	}
	return true
}

// resetExprVector replaces the contents of v with hs, holding the new
// elements before releasing the old ones, as element-wise assignment does.
func (t *SMTTheorySeq) resetExprVector(v *seqExprVector, hs []ast.ASTHandle) {
	nv := newSeqExprVector(t.m)
	t.chk(nv.append(hs))
	t.chk(v.reset())
	*v = nv
}

// mkEqdep is mk_eqdep(l, r, dep).
func (t *SMTTheorySeq) mkEqdep(l, r ast.ASTHandle, dep *seqDep) seqEq {
	e := seqEq{id: t.eqID, ls: newSeqExprVector(t.m), rs: newSeqExprVector(t.m), dep: dep}
	t.eqID++
	t.chk(e.ls.append(t.getConcat(l)))
	t.chk(e.rs.append(t.getConcat(r)))
	return e
}

// solveUnitEqVec is solve_unit_eq(l, r, deps) on vectors.
func (t *SMTTheorySeq) solveUnitEqVec(l, r []ast.ASTHandle, deps *seqDep) bool {
	if len(l) == 1 && t.isVar(l[0]) && !t.occursVec(l[0], r) && t.addSolution(l[0], t.mkConcatSorted(r, t.sortOf(l[0])), deps) {
		return true
	}
	if len(r) == 1 && t.isVar(r[0]) && !t.occursVec(r[0], l) && t.addSolution(r[0], t.mkConcatSorted(l, t.sortOf(r[0])), deps) {
		return true
	}
	return false
}

// reduceLength is reduce_length(l, r, lits).
func (t *SMTTheorySeq) reduceLength(l, r ast.ASTHandle, lits *[]SMTLiteral) bool {
	*lits = (*lits)[:0]
	x := t.loc()
	defer x.drop()
	len1, ok1 := t.getLengthLits(x, l, lits)
	if !ok1 {
		return false
	}
	len2, ok2 := t.getLengthLits(x, r, lits)
	return ok2 && len1 == len2
}

func (t *SMTTheorySeq) solveUnitEq(l, r ast.ASTHandle, deps *seqDep) bool {
	if l == r {
		return true
	}
	if t.isVar(l) && !t.occurs(l, r) && t.addSolution(l, r, deps) {
		return true
	}
	if t.isVar(r) && !t.occurs(r, l) && t.addSolution(r, l, deps) {
		return true
	}
	return false
}

func (t *SMTTheorySeq) occursVec(a ast.ASTHandle, b []ast.ASTHandle) bool {
	for _, e := range b {
		if a == e || t.isIte(e) {
			return true
		}
	}
	return false
}

// occurs: true if a occurs under an interpreted function or under a
// left/right selector.
func (t *SMTTheorySeq) occurs(a, b ast.ASTHandle) bool {
	t.todo = append(t.todo[:0], b)
	for len(t.todo) != 0 {
		b = t.todo[len(t.todo)-1]
		if a == b || t.isIte(b) {
			t.todo = t.todo[:0]
			return true
		}
		t.todo = t.todo[:len(t.todo)-1]
		if e1, e2, ok := t.isConcat2(b); ok {
			t.todo = append(t.todo, e1, e2)
		}
	}
	return false
}

func (t *SMTTheorySeq) addSolution(l, r ast.ASTHandle, deps *seqDep) bool {
	if l == r {
		return false
	}
	t.newSolution = true
	t.chk(t.rep.update(l, r, deps))
	n1 := t.ensureEnode(l)
	n2 := t.ensureEnode(r)
	if n1.Root() != n2.Root() {
		t.propagateEqNodes(deps, n1, n2)
	}
	return true
}

func (t *SMTTheorySeq) solveEqs(i int) bool {
	change := false
	for ; !t.state.inconsistent && i < t.eqs.len(); i++ {
		e := t.eqs.at(i)
		if t.solveEq(e.ls.handles(), e.rs.handles(), e.dep) {
			if i+1 != t.eqs.len() {
				e1, err := t.copyEq(t.eqs.at(t.eqs.len() - 1))
				t.chk(err)
				t.chk(t.eqs.set(i, e1))
				i--
			}
			t.stats.numReductions++
			t.chk(t.eqs.popBack())
			change = true
		}
	}
	return change || t.newPropagation || t.state.inconsistent
}

func (t *SMTTheorySeq) solveEq(l, r []ast.ASTHandle, deps *seqDep) bool {
	ls, rs := &t.ls, &t.rs
	t.chk(rs.reset())
	t.chk(ls.reset())
	var dep2 *seqDep
	change := t.canonizeVec(l, ls, &dep2)
	change = t.canonizeVec(r, rs, &dep2) || change
	deps = t.dm.mkJoin(dep2, deps)
	if !t.state.inconsistent && t.simplifyEq(ls, rs, deps) {
		return true
	}
	if ls.empty() && rs.empty() {
		return true
	}
	if !t.state.inconsistent && t.solveUnitEqVec(ls.handles(), rs.handles(), deps) {
		return true
	}
	if !t.state.inconsistent && t.solveBinaryEq(ls.handles(), rs.handles(), deps) {
		return true
	}
	if !t.state.inconsistent && change {
		e := seqEq{id: t.eqID, ls: newSeqExprVector(t.m), rs: newSeqExprVector(t.m), dep: deps}
		t.eqID++
		t.chk(e.ls.append(ls.handles()))
		t.chk(e.rs.append(rs.handles()))
		t.eqs.pushBack(e)
		return true
	}
	return false
}

func (t *SMTTheorySeq) propagateMaxLength(l, r ast.ASTHandle, deps *seqDep) bool {
	if t.isEmpty(l) {
		l, r = r, l
	}
	s, idx, ok := t.isTailIdx(l)
	if ok && t.hasLength(s) && t.isEmpty(r) {
		if _, has := t.upperBound(s); !has {
			bound := t.mkInt(idx + 1) // arguments right to left
			t.propagateLit(deps, nil, t.mkLiteral(t.mkLe(t.mkLength(s), bound)))
			return true
		}
	}
	return false
}
