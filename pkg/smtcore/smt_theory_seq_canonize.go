package smtcore

// Source: src/smt/theory_seq.cpp:2848-3135, canonize through deque_axiom.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// canonize is canonize(e, eqs); the expr_ref result is held in x.
func (t *SMTTheorySeq) canonize(x *seqRefs, e ast.ASTHandle, eqs **seqDep) ast.ASTHandle {
	result := t.expand(x, e, eqs)
	return t.rewriteTh(x, result)
}

// canonizeInto is canonize(e, es, eqs).
func (t *SMTTheorySeq) canonizeInto(e ast.ASTHandle, es *seqExprVector, eqs **seqDep) bool {
	x := t.loc()
	defer x.drop()
	e3 := t.ref(x, e, nil)
	change := false
	for {
		if e1, e2, ok := t.isConcat2(e3); ok {
			t.canonizeInto(e1, es, eqs)
			e3 = t.ref(x, e2, nil)
			change = true
		} else if t.isEmpty(e3) {
			return true
		} else {
			e4 := t.expand(x, e3, eqs)
			change = change || e4 != e3
			t.chk(es.append(t.getConcat(e4)))
			break
		}
	}
	return change
}

// canonizeVec is canonize(es, result, eqs).
func (t *SMTTheorySeq) canonizeVec(es []ast.ASTHandle, result *seqExprVector, eqs **seqDep) bool {
	change := false
	for _, e := range es {
		change = t.canonizeInto(e, result, eqs) || change
	}
	return change
}

// expand is expand(e, eqs); the result is held in x.
func (t *SMTTheorySeq) expand(x *seqRefs, e ast.ASTHandle, eqs **seqDep) ast.ASTHandle {
	sz := len(t.expandTodo)
	t.expandTodo = append(t.expandTodo, e)
	var result ast.ASTHandle
	for len(t.expandTodo) != sz {
		e := t.expandTodo[len(t.expandTodo)-1]
		result = t.expand1(x, e, eqs)
		if !result.IsNull() {
			t.expandTodo = t.expandTodo[:len(t.expandTodo)-1]
		}
	}
	return result
}

// tryExpand returns the cached expansion of e, or pushes e on the expand
// worklist and returns null.
func (t *SMTTheorySeq) tryExpand(x *seqRefs, e ast.ASTHandle, eqs **seqDep) ast.ASTHandle {
	if ed, ok := t.rep.findCache(e); ok {
		if e != ed.expr {
			*eqs = t.dm.mkJoin(*eqs, ed.dep)
		}
		return t.ref(x, ed.expr, nil)
	}
	t.expandTodo = append(t.expandTodo, e)
	return ast.ASTHandle{}
}

func (t *SMTTheorySeq) expand1(x *seqRefs, e0 ast.ASTHandle, eqs **seqDep) ast.ASTHandle {
	result := t.tryExpand(x, e0, eqs)
	if !result.IsNull() {
		return result
	}
	var deps *seqDep
	e := t.rep.findDep(e0, &deps)
	two := func(e1, e2 ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool) {
		arg1 := t.tryExpand(x, e1, &deps)
		arg2 := t.tryExpand(x, e2, &deps)
		return arg1, arg2, !arg1.IsNull() && !arg2.IsNull()
	}
	if e1, e2, ok := t.isConcat2(e); ok {
		arg1, arg2, ok := two(e1, e2)
		if !ok {
			return result
		}
		result = t.ref(x, t.mkConcat2(arg1, arg2), nil)
	} else if t.isEmpty(e) || t.isStringConst(e) {
		result = t.ref(x, e, nil)
	} else if e1, e2, ok := t.binaryArgs(e, seqOpPrefix); ok {
		arg1, arg2, ok := two(e1, e2)
		if !ok {
			return result
		}
		result = t.ref(x, t.must(t.m.MkSequencePrefix(arg1, arg2)), nil)
	} else if e1, e2, ok := t.binaryArgs(e, seqOpSuffix); ok {
		arg1, arg2, ok := two(e1, e2)
		if !ok {
			return result
		}
		result = t.ref(x, t.must(t.m.MkSequenceSuffix(arg1, arg2)), nil)
	} else if e1, e2, ok := t.binaryArgs(e, seqOpContains); ok {
		arg1, arg2, ok := two(e1, e2)
		if !ok {
			return result
		}
		result = t.ref(x, t.must(t.m.MkSequenceContains(arg1, arg2)), nil)
	} else if e1, ok := t.isUnitArg(e); ok {
		arg1 := t.tryExpand(x, e1, &deps)
		if arg1.IsNull() {
			return result
		}
		result = t.ref(x, t.mkUnit(arg1), nil)
	} else if t.isStrOp(e, seqOpIndex) && t.numArgs(e) == 2 {
		arg1, arg2, ok := two(t.arg(e, 0), t.arg(e, 1))
		if !ok {
			return result
		}
		result = t.ref(x, t.must(t.m.MkSequenceIndex(arg1, arg2, t.mkInt(0))), nil)
	} else if t.isStrOp(e, seqOpIndex) && t.numArgs(e) == 3 {
		e3 := t.arg(e, 2)
		arg1, arg2, ok := two(t.arg(e, 0), t.arg(e, 1))
		if !ok {
			return result
		}
		result = t.ref(x, t.must(t.m.MkSequenceIndex(arg1, arg2, e3)), nil)
	} else if t.isIte(e) {
		e1, e2, e3 := t.arg(e, 0), t.arg(e, 1), t.arg(e, 2)
		ne, n2, n3 := t.enodeOf(e), t.enodeOf(e2), t.enodeOf(e3)
		if ne != nil && n2 != nil && ne.Root() == n2.Root() {
			result = t.tryExpand(x, e2, &deps)
			if result.IsNull() {
				return result
			}
			t.addDependency(&deps, ne, n2)
		} else if ne != nil && n2 != nil && n3 != nil && ne.Root() == n3.Root() {
			result = t.tryExpand(x, e3, &deps)
			if result.IsNull() {
				return result
			}
			t.addDependency(&deps, ne, n3)
		} else {
			lit := t.mkLiteral(e1)
			switch t.assignment(lit) {
			case lbool.True:
				deps = t.dm.mkJoin(deps, t.dm.mkLeaf(seqAssumption{lit: lit}))
				result = t.tryExpand(x, e2, &deps)
				if result.IsNull() {
					return result
				}
			case lbool.False:
				deps = t.dm.mkJoin(deps, t.dm.mkLeaf(seqAssumption{lit: lit.Negated()}))
				result = t.tryExpand(x, e3, &deps)
				if result.IsNull() {
					return result
				}
			default:
				result = t.ref(x, e, nil)
				t.resetCacheFlag = true
			}
		}
	} else if e1, ok := t.unaryArg(e, strOpItos); ok {
		if val, ok := t.getNumValue(e1); ok {
			num := t.ref(x, t.mkIntRat(val), nil)
			if !t.eInternalized(num) {
				t.chk(t.state.Internalize(num, false))
			}
			n1 := t.enodeOf(num)
			n2 := t.enodeOf(e1)
			res := t.ref(x, t.mkString(val.RatString()), nil)
			if val.Sign() < 0 {
				result = t.ref(x, e, nil)
			} else if n1.Root() == n2.Root() {
				// TBD remove this: using roots is unsound for propagation.
				result = res
				deps = t.dm.mkJoin(deps, t.dm.mkLeaf(seqAssumption{n1: n1, n2: n2}))
			} else {
				t.addAxiom(t.mkEq(num, e1, false).Negated(), t.mkEq(e, res, false))
				t.addAxiom(t.mkEq(num, e1, false), t.mkEq(e, res, false).Negated())
				result = t.ref(x, e, nil)
			}
		} else {
			result = t.ref(x, e, nil)
		}
	} else {
		result = t.ref(x, e, nil)
	}
	if result == e0 {
		deps = nil
	}
	t.chk(t.rep.addCache(e0, seqExprDep{result, deps}))
	*eqs = t.dm.mkJoin(*eqs, deps)
	return result
}

func (t *SMTTheorySeq) addDependency(dep **seqDep, a, b *SMTEnode) {
	if a != b {
		*dep = t.dm.mkJoin(*dep, t.dm.mkLeaf(seqAssumption{n1: a, n2: b}))
	}
}

// propagateSeq is theory_seq::propagate.
func (t *SMTTheorySeq) propagateSeq() {
	for t.axiomsHead < t.axioms.size() && !t.state.inconsistent {
		x := t.loc()
		e := t.ref(x, t.axioms.get(t.axiomsHead), nil)
		t.dequeAxiom(e)
		t.axiomsHead++
		x.drop()
	}
	for len(t.replay) != 0 && !t.state.inconsistent {
		app := &t.replay[len(t.replay)-1]
		switch app.kind {
		case seqReplayLengthCoherence:
			t.checkLengthCoherenceE(app.e)
		case seqReplayFixedLength:
			t.fixedLength(app.e)
		case seqReplayAxiom:
			t.enqueAxiom(app.e)
		}
		app = &t.replay[len(t.replay)-1]
		t.chk(t.m.DelRef(&app.ref))
		t.replay = t.replay[:len(t.replay)-1]
	}
	if t.newSolution {
		t.simplifyAndSolveEqs()
		t.newSolution = false
	}
}

func (t *SMTTheorySeq) enqueAxiom(e ast.ASTHandle) {
	if _, ok := t.axiomSet[e]; !ok {
		t.chk(t.axioms.pushBack(e))
		t.axiomSet[e] = struct{}{}
		t.pushTrail(func(th *SMTTheorySeq) error { // push_back_vector
			return th.axioms.shrink(th.axioms.size() - 1)
		})
		t.pushTrail(func(th *SMTTheorySeq) error { // insert_obj_trail
			delete(th.axiomSet, e)
			return nil
		})
	}
}

func (t *SMTTheorySeq) dequeAxiom(n ast.ASTHandle) {
	if t.isStrOp(n, seqOpLength) {
		t.addLengthAxiom(n)
	} else if t.isEmpty(n) && !t.hasLength(n) && t.length.size != 0 {
		t.ensureEnode(n)
		t.enforceLengthNode(t.enodeOf(n))
	} else if t.isStrOp(n, seqOpIndex) {
		t.addIndexofAxiom(n)
	} else if t.isStrOp(n, seqOpReplace) {
		t.addReplaceAxiom(n)
	} else if t.isStrOp(n, seqOpExtract) {
		t.addExtractAxiom(n)
	} else if t.isStrOp(n, seqOpAt) {
		t.addAtAxiom(n)
	} else if t.isStringConst(n) {
		t.addElimStringAxiom(n)
	} else if t.isStrOp(n, strOpItos) {
		t.addItosAxiom(n)
	} else if t.isStrOp(n, strOpStoi) {
		t.addStoiAxiom(n)
	}
}
