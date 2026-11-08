package smtcore

// Source: src/smt/theory_seq.cpp:1841-2222, solve_nqs through
// simplify_and_solve_eqs.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (t *SMTTheorySeq) solveNqs(i int) bool {
	for ; !t.state.inconsistent && i < t.nqs.len(); i++ {
		if t.solveNe(i) {
			if i+1 != t.nqs.len() {
				n, err := t.copyNe(t.nqs.at(t.nqs.len() - 1))
				t.chk(err)
				t.chk(t.nqs.set(i, n))
				i--
			}
			t.chk(t.nqs.popBack())
		}
	}
	return t.newPropagation || t.state.inconsistent
}

// seqVecs is vector<expr_ref_vector>; releaseSeqVecs destroys it.
func (t *SMTTheorySeq) releaseSeqVecs(vs []seqExprVector) {
	for i := len(vs) - 1; i >= 0; i-- {
		t.chk(vs[i].reset())
	}
}

func (t *SMTTheorySeq) copyVec(hs []ast.ASTHandle) seqExprVector {
	v := newSeqExprVector(t.m)
	t.chk(v.append(hs))
	return v
}

func (t *SMTTheorySeq) solveNe(idx int) bool {
	n := t.nqs.at(idx)
	numUndefLits := 0
	for _, lit := range n.lits {
		switch t.assignment(lit) {
		case lbool.False:
			return true
		case lbool.Undef:
			numUndefLits++
		}
	}

	updated := false
	newDeps := n.dep
	var newLs, newRs []seqExprVector
	defer func() { t.releaseSeqVecs(newRs); t.releaseSeqVecs(newLs) }()
	newLits := append([]SMTLiteral(nil), n.lits...)
	for i := 0; i < len(n.lhs); i++ {
		ls, rs, lhs, rhs := &t.ls, &t.rs, &t.lhsV, &t.rhsV
		t.chk(ls.reset())
		t.chk(rs.reset())
		t.chk(lhs.reset())
		t.chk(rhs.reset())
		var deps *seqDep
		change := false
		change = t.canonizeVec(n.lhs[i].handles(), ls, &deps) || change
		change = t.canonizeVec(n.rhs[i].handles(), rs, &deps) || change

		lsS := append([]ast.ASTHandle(nil), ls.handles()...)
		rsS := append([]ast.ASTHandle(nil), rs.handles()...)
		var lhsS, rhsS []ast.ASTHandle
		ok, err := t.seqRewrite.ReduceEqVec(&lsS, &rsS, &lhsS, &rhsS, &change)
		t.chk(err)
		t.resetExprVector(ls, lsS)
		t.resetExprVector(rs, rsS)
		t.resetExprVector(lhs, lhsS)
		t.resetExprVector(rhs, rhsS)
		if !ok {
			return true
		} else if !change {
			if updated {
				newLs = append(newLs, t.copyVec(n.lhs[i].handles()))
				newRs = append(newRs, t.copyVec(n.rhs[i].handles()))
			}
			continue
		}
		if !updated {
			for j := 0; j < i; j++ {
				newLs = append(newLs, t.copyVec(n.lhs[j].handles()))
				newRs = append(newRs, t.copyVec(n.rhs[j].handles()))
			}
		}
		updated = true
		if !ls.empty() || !rs.empty() {
			newLs = append(newLs, t.copyVec(ls.handles()))
			newRs = append(newRs, t.copyVec(rs.handles()))
		}
		for j := 0; j < lhs.size(); j++ {
			nl := lhs.get(j)
			nr := rhs.get(j)
			if t.isSeq(nl) || t.isRe(nl) {
				t.resetExprVector(ls, t.getConcat(nl))
				t.resetExprVector(rs, t.getConcat(nr))
				newLs = append(newLs, t.copyVec(ls.handles()))
				newRs = append(newRs, t.copyVec(rs.handles()))
			} else if nl != nr {
				lit := t.mkEq(nl, nr, false)
				t.markLitRelevant(lit)
				newLits = append(newLits, lit)
				switch t.assignment(lit) {
				case lbool.False:
					return true
				case lbool.Undef:
					numUndefLits++
					t.newPropagation = true
				}
			}
		}
		newDeps = t.dm.mkJoin(deps, newDeps)
	}

	if !updated && numUndefLits == 0 {
		return false
	}
	if !updated {
		for j := 0; j < len(n.lhs); j++ {
			newLs = append(newLs, t.copyVec(n.lhs[j].handles()))
			newRs = append(newRs, t.copyVec(n.rhs[j].handles()))
		}
	}

	if numUndefLits == 1 && len(newLs) == 0 {
		var lits []SMTLiteral
		undefLit := SMTNullLiteral
		for _, lit := range newLits {
			switch t.assignment(lit) {
			case lbool.True:
				lits = append(lits, lit)
			case lbool.Undef:
				undefLit = lit
			}
		}
		t.propagateLit(newDeps, lits, undefLit.Negated())
		return true
	}
	if updated {
		if numUndefLits == 0 && len(newLs) == 0 {
			var deps1 *seqDep
			if t.explainEq(n.l, n.r, &deps1) {
				diseq := t.mkEq(n.l, n.r, false)
				if t.assignment(diseq) == lbool.False {
					newLits = []SMTLiteral{diseq.Negated()}
					newDeps = deps1
				}
			}
			t.setConflict(newDeps, newLits)
		} else {
			t.nqs.pushBack(t.mkNe(n.l, n.r, newLs, newRs, newLits, newDeps))
		}
	}
	return updated
}

// mkNe is ne(l, r, ls, rs, lits, dep): the new element holds copies.
func (t *SMTTheorySeq) mkNe(l, r ast.ASTHandle, ls, rs []seqExprVector, lits []SMTLiteral, dep *seqDep) seqNe {
	n := seqNe{dep: dep, refs: seqRefs{manager: t.m}, lits: append([]SMTLiteral(nil), lits...)}
	var err error
	n.l, err = n.refs.hold(l)
	t.chk(err)
	n.r, err = n.refs.hold(r)
	t.chk(err)
	for i := range ls {
		n.lhs = append(n.lhs, t.copyVec(ls[i].handles()))
	}
	for i := range rs {
		n.rhs = append(n.rhs, t.copyVec(rs[i].handles()))
	}
	return n
}

// mkNe2 is ne(l, r, dep): one side pair [l] != [r].
func (t *SMTTheorySeq) mkNe2(l, r ast.ASTHandle, dep *seqDep) seqNe {
	n := seqNe{dep: dep, refs: seqRefs{manager: t.m}}
	var err error
	n.l, err = n.refs.hold(l)
	t.chk(err)
	n.r, err = n.refs.hold(r)
	t.chk(err)
	n.lhs = append(n.lhs, t.copyVec([]ast.ASTHandle{l}))
	n.rhs = append(n.rhs, t.copyVec([]ast.ASTHandle{r}))
	return n
}

// mkNc is nc(c, dep).
func (t *SMTTheorySeq) mkNc(c ast.ASTHandle, dep *seqDep) seqNc {
	n := seqNc{dep: dep, refs: seqRefs{manager: t.m}}
	var err error
	n.contains, err = n.refs.hold(c)
	t.chk(err)
	return n
}

func (t *SMTTheorySeq) solveNc(idx int) bool {
	n := t.ncs.at(idx)
	deps := n.dep
	x := t.loc()
	defer x.drop()
	c := t.ref(x, t.canonize(n.contains, &deps), nil)
	if t.m.IsTrue(c) {
		t.setConflict(deps, nil)
		return true
	}
	if t.m.IsFalse(c) {
		return true
	}
	if c != n.contains {
		t.ncs.pushBack(t.mkNc(c, deps))
		t.newPropagation = true
		return true
	}
	if e1, e2, ok := t.isEqApp(c); ok {
		eq := t.mkEq(e1, e2, false)
		t.propagateLit(deps, nil, eq.Negated())
		return true
	}
	if t.isOrApp(c) {
		for i := uint32(0); i < t.numArgs(c); i++ {
			ci := t.ref(x, t.arg(c, i), nil)
			t.ncs.pushBack(t.mkNc(ci, deps))
		}
		t.newPropagation = true
		return true
	}
	return false
}

// isEqApp is m.is_eq(c, e1, e2).
func (t *SMTTheorySeq) isEqApp(c ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool) {
	op, basic, err := t.state.basicOperation(c)
	t.chk(err)
	if !basic || op != ast.ASTOpEq {
		return ast.ASTHandle{}, ast.ASTHandle{}, false
	}
	return t.arg(c, 0), t.arg(c, 1), true
}

// isOrApp is m.is_or(c).
func (t *SMTTheorySeq) isOrApp(c ast.ASTHandle) bool {
	op, basic, err := t.state.basicOperation(c)
	t.chk(err)
	return basic && op == ast.ASTOpOr
}

func (t *SMTTheorySeq) mkCell(p *seqCell, e ast.ASTHandle, d *seqDep) *seqCell {
	c := &seqCell{parent: p, expr: e, dep: d}
	t.allCells = append(t.allCells, c)
	return c
}

func (t *SMTTheorySeq) unfold(c *seqCell, cons *[]*seqCell) {
	var dep *seqDep
	var a ast.ASTHandle
	if t.rep.find1(c.expr, &a, &dep) {
		c1 := t.mkCell(c, a, t.dm.mkJoin(dep, c.dep))
		t.unfold(c1, cons)
	} else if e1, e2, ok := t.isConcat2(c.expr); ok {
		c1 := t.mkCell(c, e1, c.dep)
		c2 := t.mkCell(nil, e2, nil)
		t.unfold(c1, cons)
		t.unfold(c2, cons)
	} else {
		*cons = append(*cons, c)
	}
	c.last = len(*cons) - 1
}

func (t *SMTTheorySeq) explainEq(e1, e2 ast.ASTHandle, dep **seqDep) bool {
	if e1 == e2 {
		return true
	}
	var v1, v2 []*seqCell
	cellsSz := len(t.allCells)
	c1 := t.mkCell(nil, e1, nil)
	c2 := t.mkCell(nil, e2, nil)
	t.unfold(c1, &v1)
	t.unfold(c2, &v2)
	i, j := 0, 0
	result := true
	for i < len(v1) || j < len(v2) {
		if i == len(v1) {
			for j < len(v2) && t.isEmpty(v2[j].expr) {
				*dep = t.dm.mkJoin(*dep, v2[j].dep)
				j++
			}
			result = j == len(v2)
			break
		}
		if j == len(v2) {
			for i < len(v1) && t.isEmpty(v1[i].expr) {
				*dep = t.dm.mkJoin(*dep, v1[i].dep)
				i++
			}
			result = i == len(v1)
			break
		}
		c1 := v1[i]
		c2 := v2[j]
		e1 = c1.expr
		e2 = c2.expr
		if e1 == e2 {
			if c1.parent != nil && c2.parent != nil && c1.parent.expr == c2.parent.expr {
				c1 = c1.parent
				c2 = c2.parent
				v1[c1.last] = c1
				i = c1.last
				v2[c2.last] = c2
				j = c2.last
			} else {
				*dep = t.dm.mkJoin(*dep, c1.dep)
				*dep = t.dm.mkJoin(*dep, c2.dep)
				i++
				j++
			}
		} else if t.isEmpty(e1) {
			*dep = t.dm.mkJoin(*dep, c1.dep)
			i++
		} else if t.isEmpty(e2) {
			*dep = t.dm.mkJoin(*dep, c2.dep)
			j++
		} else if a1, ok1 := t.isUnitArg(e1); ok1 {
			a2, ok2 := t.isUnitArg(e2)
			if !ok2 {
				result = false
				break
			}
			if t.explainEq(a1, a2, dep) {
				i++
				j++
			} else {
				result = false
				break
			}
		} else {
			result = false
			break
		}
	}
	t.allCells = t.allCells[:cellsSz]
	return result
}

func (t *SMTTheorySeq) explainEmpty(es *seqExprVector, dep **seqDep) bool {
	for !es.empty() {
		e := es.back()
		if t.isEmpty(e) {
			t.chk(es.shrink(es.size() - 1))
			continue
		}
		var a ast.ASTHandle
		if t.rep.find1(e, &a, dep) {
			t.chk(es.shrink(es.size() - 1))
			t.chk(es.append(t.getConcat(a)))
			continue
		}
		return false
	}
	return true
}

func (t *SMTTheorySeq) simplifyAndSolveEqs() bool {
	t.newSolution = true
	for t.newSolution && !t.state.inconsistent {
		t.newSolution = false
		t.solveEqs(0)
	}
	return t.newPropagation || t.state.inconsistent
}
