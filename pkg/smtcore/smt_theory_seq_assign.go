package smtcore

// Source: src/smt/theory_seq.cpp:3935-4164, assign_eh through relevant_eh,
// and util/union_find.h for m_find.

import (
	"github.com/glycerine/g3/pkg/ast"
)

// seqUnionFind is union_find<theory_seq> on the theory's trail; theory_seq's
// merge_eh, after_merge_eh and unmerge_eh are empty.
type seqUnionFind struct {
	find, size, next []uint32
}

func (t *SMTTheorySeq) ufMkVar() {
	u := &t.uf
	r := uint32(len(u.find))
	u.find = append(u.find, r)
	u.size = append(u.size, 1)
	u.next = append(u.next, r)
	t.pushTrail(func(th *SMTTheorySeq) error { // mk_var_trail
		u := &th.uf
		u.find = u.find[:len(u.find)-1]
		u.size = u.size[:len(u.size)-1]
		u.next = u.next[:len(u.next)-1]
		return nil
	})
}

func (t *SMTTheorySeq) ufFind(v uint32) uint32 {
	for {
		nv := t.uf.find[v]
		if nv == v {
			return v
		}
		v = nv
	}
}

func (t *SMTTheorySeq) ufMerge(v1, v2 uint32) {
	u := &t.uf
	r1 := t.ufFind(v1)
	r2 := t.ufFind(v2)
	if r1 == r2 {
		return
	}
	if u.size[r1] > u.size[r2] {
		r1, r2 = r2, r1
	}
	u.find[r1] = r2
	u.size[r2] += u.size[r1]
	u.next[r1], u.next[r2] = u.next[r2], u.next[r1]
	t.pushTrail(func(th *SMTTheorySeq) error { // merge_trail: unmerge(r1)
		u := &th.uf
		r2 := u.find[r1]
		u.size[r2] -= u.size[r1]
		u.find[r1] = r1
		u.next[r1], u.next[r2] = u.next[r2], u.next[r1]
		return nil
	})
}

func (t *SMTTheorySeq) assignEh(v int32, isTrue bool) {
	e, err := t.state.BoolExpression(v)
	t.chk(err)
	x := t.loc()
	defer x.drop()
	change := false
	lit := NewSMTLiteral(v, !isTrue)

	if e1, e2, ok := t.binaryArgs(e, seqOpPrefix); ok {
		if isTrue {
			f := t.ref(x, t.mkSkolem(t.prefix, e1, e2, ast.ASTHandle{}, ast.ASTHandle{}), nil)
			f = t.ref(x, t.mkConcat2(e1, f), nil)
			t.propagateEqLitOnly(lit, f, e2, true)
		} else {
			t.propagateNonEmpty(lit, e1)
			if t.addPrefix2prefix(e, &change) {
				t.addAtom(e)
			}
		}
	} else if e1, e2, ok := t.binaryArgs(e, seqOpSuffix); ok {
		if isTrue {
			f := t.ref(x, t.mkSkolem(t.suffix, e1, e2, ast.ASTHandle{}, ast.ASTHandle{}), nil)
			f = t.ref(x, t.mkConcat2(f, e1), nil)
			t.propagateEqLitOnly(lit, f, e2, true)
		} else {
			t.propagateNotSuffix(e)
		}
	} else if e1, e2, ok := t.binaryArgs(e, seqOpContains); ok {
		if isTrue {
			f1 := t.ref(x, t.mkSkolem(t.indexofLeft, e1, e2, ast.ASTHandle{}, ast.ASTHandle{}), nil)
			f2 := t.ref(x, t.mkSkolem(t.indexofRight, e1, e2, ast.ASTHandle{}, ast.ASTHandle{}), nil)
			f := t.ref(x, t.mkConcat3(f1, e2, f2), nil)
			t.propagateEqLitOnly(lit, f, e1, true)
		} else if !t.canonizes(false, e) {
			t.propagateNonEmpty(lit, e2)
			dep := t.dm.mkLeaf(seqAssumption{lit: lit})
			t.ncs.pushBack(t.mkNc(e, dep))
		}
	} else if t.isAccept(e) {
		if isTrue {
			t.propagateAccRejLength(lit, e)
			if t.addAccept2step(e, &change) {
				t.addAtom(e)
			}
		}
	} else if t.isReject(e) {
		if isTrue {
			t.propagateAccRejLength(lit, e)
			t.addAtom(e)
		}
	} else if t.isStep(e) {
		if isTrue {
			t.propagateStep(lit, e)
			if t.addStep2accept(e, &change) {
				t.addAtom(e)
			}
		}
	} else if e1, e2, ok := t.isEqSk(e); ok {
		if isTrue {
			t.propagateEqLitOnly(lit, e1, e2, true)
		}
	} else if t.isStrOp(e, seqOpInRe) {
		t.propagateInRe(e, isTrue)
	} else if t.isSkolem(t.symbol("seq.split"), e) {
		// propagate equalities
	} else if t.isSkolem(t.symbol("seq.is_digit"), e) {
	} else {
		// UNREACHABLE(): reached by Boolean atoms the theory owns without
		// a case here, such as (seq.nth s i) over (Seq Bool). Release builds
		// compile it to __builtin_unreachable, and z3 continues as if the
		// assignment were ignored.
	}
}

func (t *SMTTheorySeq) addAtom(e ast.ASTHandle) {
	t.pushTrail(func(th *SMTTheorySeq) error { // push_back_vector
		th.atoms = th.atoms[:len(th.atoms)-1]
		return nil
	})
	t.atoms = append(t.atoms, e)
}

func (t *SMTTheorySeq) newEqEh(v1, v2 SMTTheoryVar) {
	n1, err := t.Enode(v1)
	t.chk(err)
	n2, err := t.Enode(v2)
	t.chk(err)
	deps := t.dm.mkLeaf(seqAssumption{n1: n1, n2: n2, lit: SMTNullLiteral})
	t.newEqEhDep(deps, n1, n2)
}

func (t *SMTTheorySeq) newEqEhDep(deps *seqDep, n1, n2 *SMTEnode) {
	if n1 != n2 && t.isSeq(n1.Owner()) {
		v1 := n1.TheoryVar(t.FamilyID())
		v2 := n2.TheoryVar(t.FamilyID())
		if t.ufFind(uint32(v1)) == t.ufFind(uint32(v2)) {
			return
		}
		t.ufMerge(uint32(v1), uint32(v2))
		x := t.loc()
		o1 := t.ref(x, n1.Owner(), nil)
		o2 := t.ref(x, n2.Owner(), nil)
		t.eqs.pushBack(t.mkEqdep(o1, o2, deps))
		t.solveEqs(t.eqs.len() - 1)
		t.enforceLengthCoherence(n1, n2)
		x.drop()
	}
	// regular expressions: ignored
}

func (t *SMTTheorySeq) newDiseqEh(v1, v2 SMTTheoryVar) {
	n1, err := t.Enode(v1)
	t.chk(err)
	n2, err := t.Enode(v2)
	t.chk(err)
	x := t.loc()
	defer x.drop()
	e1 := t.ref(x, n1.Owner(), nil)
	e2 := t.ref(x, n2.Owner(), nil)
	t.chk(t.exclude.update(e1, e2))
	eq := t.ref(x, t.mkEqExpr(e1, e2), nil)
	eq = t.rewriteTh(x, eq)
	if !t.m.IsFalse(eq) {
		lit := t.mkEq(e1, e2, false)
		if t.isEmpty(e2) {
			e1, e2 = e2, e1
		}
		dep := t.dm.mkLeaf(seqAssumption{lit: lit.Negated()})
		t.nqs.pushBack(t.mkNe2(e1, e2, dep))
		t.solveNqs(t.nqs.len() - 1)
	}
}

func (t *SMTTheorySeq) pushScopeEh() {
	t.chk(t.SMTTheoryBase.PushScope())
	t.rep.pushScope()
	t.exclude.pushScope()
	t.trailLim = append(t.trailLim, len(t.trailStack))
	head := t.axiomsHead
	t.pushTrail(func(th *SMTTheorySeq) error { // value_trail(m_axioms_head)
		th.axiomsHead = head
		return nil
	})
	t.eqs.pushScope()
	t.nqs.pushScope()
	t.ncs.pushScope()
	t.atomsLim = append(t.atomsLim, len(t.atoms))
}

func (t *SMTTheorySeq) popTrail(numScopes int) {
	lim := t.trailLim[len(t.trailLim)-numScopes]
	for i := len(t.trailStack) - 1; i >= lim; i-- {
		undo := t.trailStack[i]
		t.trailStack = t.trailStack[:i]
		t.chk(undo(t))
	}
	t.trailLim = t.trailLim[:len(t.trailLim)-numScopes]
}

func (t *SMTTheorySeq) popScopeEh(numScopes int) {
	// The base-level test reads the levels before the pop, as the source's
	// pop_scope_core calls pop_scope_eh before updating them.
	belowBase := t.state.baseLevel > t.state.scopeLevel-uint32(numScopes)
	t.popTrail(numScopes)
	t.chk(t.SMTTheoryBase.PopScopes(uint32(numScopes)))
	t.chk(t.rep.popScope(numScopes))
	t.chk(t.exclude.popScope(numScopes))
	t.chk(t.eqs.popScope(numScopes))
	t.chk(t.nqs.popScope(numScopes))
	t.chk(t.ncs.popScope(numScopes))
	t.atoms = t.atoms[:t.atomsLim[len(t.atomsLim)-numScopes]]
	t.atomsLim = t.atomsLim[:len(t.atomsLim)-numScopes]
	r, err := t.rewrite.rewriterFor(t.m)
	t.chk(err)
	t.chk(r.Reset())
	if belowBase {
		t.resetReplay()
	}
}

// resetReplay is m_replay.reset(): each apply releases its expr_ref.
func (t *SMTTheorySeq) resetReplay() {
	for i := range t.replay {
		t.chk(t.m.DelRef(&t.replay[i].ref))
	}
	t.replay = t.replay[:0]
}

func (t *SMTTheorySeq) relevantEh(n ast.ASTHandle) {
	if t.isStrOp(n, seqOpIndex) || t.isStrOp(n, seqOpReplace) || t.isStrOp(n, seqOpExtract) ||
		t.isStrOp(n, seqOpAt) || t.isEmpty(n) || t.isStringConst(n) ||
		t.isStrOp(n, strOpItos) || t.isStrOp(n, strOpStoi) {
		t.enqueAxiom(n)
	}
	if t.isStrOp(n, strOpItos) || t.isStrOp(n, strOpStoi) {
		t.addIntString(n)
	}
	if arg, ok := t.unaryArg(n, seqOpLength); ok && !t.hasLength(arg) && t.eInternalized(arg) {
		t.enforceLengthNode(t.enodeOf(arg))
	}
}
