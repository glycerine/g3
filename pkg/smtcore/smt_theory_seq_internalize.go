package smtcore

// Source: src/smt/theory_seq.cpp:2223-2466 (internalize_eq_eh through
// apply_sort_cnstr), 2602-2621 (collect_statistics, init_search_eh) and
// 2827-2846 (mk_var, can_propagate).

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func (t *SMTTheorySeq) internalizeAtom(a ast.ASTHandle) bool { return t.internalizeTerm(a) }

func (t *SMTTheorySeq) internalizeTerm(term ast.ASTHandle) bool {
	if e := t.enodeOf(term); e != nil {
		t.mkVar(e)
		return true
	}
	for i := uint32(0); i < t.numArgs(term); i++ {
		t.mkVar(t.ensureEnode(t.arg(term, i)))
	}
	isBool := t.isBoolExpr(term)
	if isBool {
		bv, err := t.state.MkBoolVar(term)
		t.chk(err)
		t.chk(t.state.SetVariableTheory(bv, SMTTheoryID(t.FamilyID())))
		t.chk(t.state.markRelevant(term))
	}
	e := t.enodeOf(term)
	if e == nil {
		var err error
		e, err = t.state.MkEnode(term, false, isBool, true)
		t.chk(err)
	}
	t.mkVar(e)
	return true
}

func (t *SMTTheorySeq) isBoolExpr(e ast.ASTHandle) bool {
	b, err := t.m.MkBoolSort()
	t.chk(err)
	return t.sortOf(e) == b
}

func (t *SMTTheorySeq) hasLength(e ast.ASTHandle) bool { return t.mustBool(t.length.contains(e)) }

func (t *SMTTheorySeq) addLength(e ast.ASTHandle) {
	t.insertObjTrail(&t.length, e)
}

// enforceLengthNode ensures that all elements in the equivalence class occur
// under an application of 'length'.
func (t *SMTTheorySeq) enforceLengthNode(n *SMTEnode) {
	n1 := n
	for {
		o := n.Owner()
		if !t.hasLength(o) {
			x := t.loc()
			ln := t.ref(x, t.mkLength(o), nil)
			t.enqueAxiom(ln)
			t.addLength(o)
			x.drop()
		}
		n = n.Next()
		if n == n1 {
			return
		}
	}
}

func (t *SMTTheorySeq) addIntString(e ast.ASTHandle) {
	t.chk(t.intString.pushBack(e))
	t.pushTrail(func(th *SMTTheorySeq) error { // push_back_vector
		return th.intString.shrink(th.intString.size() - 1)
	})
}

func (t *SMTTheorySeq) checkIntString() bool {
	change := false
	for _, e := range append([]ast.ASTHandle(nil), t.intString.handles()...) {
		if t.isStrOp(e, strOpItos) && t.addItosValAxiom(e) {
			change = true
		} else if t.isStrOp(e, strOpStoi) && t.addStoiValAxiom(e) {
			change = true
		}
	}
	return change
}

// mkNumRe is (0-9)+.
func (t *SMTTheorySeq) mkNumRe(x *seqRefs) ast.ASTHandle {
	numRe := t.ref(x, t.must(t.m.MkRegexRange(t.mkString("0"), t.mkString("9"))), nil)
	return t.ref(x, t.must(t.m.MkRegexPlus(numRe)), nil)
}

func (t *SMTTheorySeq) addStoiAxiom(e ast.ASTHandle) {
	s, _ := t.unaryArg(e, strOpStoi)
	// stoi(s) >= -1
	l := t.mkSimplifiedLiteral(t.mkGe(e, t.mkInt(-1)))
	t.addAxiom(l)
	// stoi(s) >= 0 <=> s in (0-9)+
	x := t.loc()
	defer x.drop()
	numRe := t.mkNumRe(x)
	inRe := t.ref(x, t.must(t.m.MkSequenceInRegex(s, numRe)), nil)
	ge0 := t.mkSimplifiedLiteral(t.mkGe(e, t.mkInt(0)))
	t.addAxiom(ge0.Negated(), t.mkLiteral(inRe))
	t.addAxiom(ge0, t.mkLiteral(inRe).Negated())
}

func (t *SMTTheorySeq) insertRationalTrail(set map[string]struct{}, val *big.Rat) {
	key := val.RatString()
	t.pushTrail(func(*SMTTheorySeq) error { // insert_map
		delete(set, key)
		return nil
	})
}

func (t *SMTTheorySeq) addStoiValAxiom(e ast.ASTHandle) bool {
	n, _ := t.unaryArg(e, strOpStoi)
	val, ok := t.getNumValue(e)
	if !ok {
		return false
	}
	x := t.loc()
	defer x.drop()
	if _, has := t.stoiAxioms[val.RatString()]; !has {
		t.stoiAxioms[val.RatString()] = struct{}{}
		if val.Cmp(big.NewRat(-1, 1)) != 0 {
			e1 := t.ref(x, t.mkString(val.RatString()), nil)
			n1 := t.ref(x, t.mkIntRat(val), nil)
			eq1 := t.mkEq(e, n1, false)
			eq2 := t.mkEq(n, e1, false)
			t.addAxiom(eq1.Negated(), eq2)
			t.addAxiom(eq2.Negated(), eq1)
			t.forcePhase(eq1)
			t.forcePhase(eq2)
			t.insertRationalTrail(t.stoiAxioms, val)
			t.pushReplay(seqReplayAxiom, e)
			return true
		}
	}
	// val is reused for the bound and the length, as in the source.
	if hi, ok := t.upperBound(n); ok {
		val = hi
		if ln, ok := t.getLengthValue(n); ok {
			val = ln
			if _, has := t.stoiAxioms[val.RatString()]; val.Sign() > 0 && !has {
				sz := val.Num().Int64()
				len1 := t.ref(x, t.mkLength(n), nil)
				len2 := t.ref(x, t.mkInt(sz), nil)
				lit := t.mkEq(len1, len2, false)
				lits := []SMTLiteral{lit.Negated()}
				nums := newSeqExprVector(t.m)
				defer func() { t.chk(nums.reset()) }()
				for i := int64(0); i < sz; i++ {
					ithChar := t.ref(x, t.mkNth(n, t.mkInt(i)), nil)
					lits = append(lits, t.isDigit(ithChar).Negated())
					t.chk(nums.pushBack(t.digit2int(x, ithChar)))
				}
				c := big.NewRat(1, 1)
				for i := sz - 1; i >= 0; i-- {
					coeff := t.ref(x, t.mkIntRat(c), nil)
					t.chk(nums.set(int(i), t.mkMul(coeff, nums.get(int(i)))))
					c.Mul(c, big.NewRat(10, 1))
				}
				num := t.ref(x, t.mkAddN(nums.handles()), nil)
				num = t.ctxRewrite(x, num)
				lits = append(lits, t.mkEq(e, num, false))
				t.stats.addAxiom++
				t.newPropagation = true
				for _, l := range lits {
					t.markLitRelevant(l)
				}
				t.chk(t.state.addTheoryAxiom(SMTTheoryID(t.FamilyID()), lits))
				t.stoiAxioms[val.RatString()] = struct{}{}
				t.insertRationalTrail(t.stoiAxioms, val)
				t.pushReplay(seqReplayAxiom, e)
				return true
			}
		}
	}
	return false
}

// mkAddN is m_autil.mk_add(n, args): one n-ary application.
func (t *SMTTheorySeq) mkAddN(args []ast.ASTHandle) ast.ASTHandle {
	fid, err := t.m.ArithmeticFamilyID()
	t.chk(err)
	return t.must(t.m.MkFamilyApp(fid, smtArithOpAdd, nil, args))
}

func (t *SMTTheorySeq) intSort() ast.ASTHandle {
	fid, err := t.m.ArithmeticFamilyID()
	t.chk(err)
	return t.must(t.m.MkFamilySort(fid, smtArithIntSort, nil))
}

// ctxRewrite is ctx.get_rewriter()(e), held in x.
func (t *SMTTheorySeq) ctxRewrite(x *seqRefs, e ast.ASTHandle) ast.ASTHandle {
	if t.state.theoryRewriter == nil {
		t.chk(ast.ErrASTDomain)
	}
	return t.rewriteWith(x, e, t.state.theoryRewriter)
}

func (t *SMTTheorySeq) isDigit(ch ast.ASTHandle) SMTLiteral {
	x := t.loc()
	defer x.drop()
	boolSort, err := t.m.MkBoolSort()
	t.chk(err)
	isd := t.mkLiteral(t.mkSkolem(t.symbol("seq.is_digit"), ch, ast.ASTHandle{}, ast.ASTHandle{}, boolSort))
	d2i := t.digit2int(x, ch)
	_lo := t.ref(x, t.must(t.m.MkBitVectorULE(t.must(t.m.BitVectorNumeralUint64('0', 8)), ch)), nil)
	_hi := t.ref(x, t.must(t.m.MkBitVectorULE(ch, t.must(t.m.BitVectorNumeralUint64('9', 8)))), nil)
	lo := t.mkLiteral(_lo)
	hi := t.mkLiteral(_hi)
	t.addAxiom(lo.Negated(), hi.Negated(), isd)
	t.addAxiom(isd.Negated(), lo)
	t.addAxiom(isd.Negated(), hi)
	for i := int64(0); i < 10; i++ {
		t.addAxiom(t.mkEq(ch, t.must(t.m.BitVectorNumeralUint64(uint64('0'+i), 8)), false).Negated(),
			t.mkEq(d2i, t.mkInt(i), false))
	}
	return isd
}

// digit2int returns an expr_ref held in x.
func (t *SMTTheorySeq) digit2int(x *seqRefs, ch ast.ASTHandle) ast.ASTHandle {
	return t.ref(x, t.mkSkolem(t.symbol("seq.digit2int"), ch, ast.ASTHandle{}, ast.ASTHandle{}, t.intSort()), nil)
}

func (t *SMTTheorySeq) addItosAxiom(e ast.ASTHandle) {
	n, _ := t.unaryArg(e, strOpItos)
	x := t.loc()
	defer x.drop()
	// itos(n) = "" <=> n < 0
	e1 := t.ref(x, t.mkEmpty(t.sortOf(e)), nil)
	zero := t.ref(x, t.mkInt(0), nil)
	eq1 := t.mkEq(e1, e, false)
	ge0 := t.mkLiteral(t.mkGe(n, zero))
	// n >= 0 => itos(n) != ""
	// itos(n) = "" or n >= 0
	t.addAxiom(eq1.Negated(), ge0.Negated())
	t.addAxiom(eq1, ge0)
	// n >= 0 => stoi(itos(n)) = n
	stoi := t.ref(x, t.must(t.m.MkStringToInt(e)), nil)
	t.addAxiom(ge0.Negated(), t.mkEq(stoi, n, false))
	// n >= 0 => itos(n) in (0-9)+
	numRe := t.mkNumRe(x)
	inRe := t.ref(x, t.must(t.m.MkSequenceInRegex(e, numRe)), nil)
	t.addAxiom(ge0.Negated(), t.mkLiteral(inRe))
}

func (t *SMTTheorySeq) addItosValAxiom(e ast.ASTHandle) bool {
	n, _ := t.unaryArg(e, strOpItos)
	change := false
	if val, ok := t.getNumValue(n); ok && val.Sign() >= 0 {
		if _, has := t.itosAxioms[val.RatString()]; !has {
			t.itosAxioms[val.RatString()] = struct{}{}
			x := t.loc()
			defer x.drop()
			e1 := t.ref(x, t.mkString(val.RatString()), nil)
			n1 := t.ref(x, t.mkIntRat(val), nil)
			// itos(n) = "25" <=> n = 25
			eq1 := t.mkEq(n1, n, false)
			eq2 := t.mkEq(e, e1, false)
			t.addAxiom(eq1.Negated(), eq2)
			t.addAxiom(eq2.Negated(), eq1)
			t.forcePhase(eq1)
			t.forcePhase(eq2)
			t.insertRationalTrail(t.itosAxioms, val)
			t.pushReplay(seqReplayAxiom, e)
			change = true
		}
	}
	return change
}

func (t *SMTTheorySeq) applySortCnstr(n *SMTEnode) { t.mkVar(n) }

// collectStatistics is collect_statistics.
func (t *SMTTheorySeq) collectStatistics(st *statistics.Statistics) {
	st.UpdateUint("seq num splits", t.stats.numSplits)
	st.UpdateUint("seq num reductions", t.stats.numReductions)
	st.UpdateUint("seq unfold def", t.stats.propagateAutomata)
	st.UpdateUint("seq length coherence", t.stats.checkLengthCoherence)
	st.UpdateUint("seq branch", t.stats.branchVariable)
	st.UpdateUint("seq solve !=", t.stats.solveNqs)
	st.UpdateUint("seq solve =", t.stats.solveEqs)
	st.UpdateUint("seq add axiom", t.stats.addAxiom)
	st.UpdateUint("seq extensionality", t.stats.extensionality)
	st.UpdateUint("seq fixed length", t.stats.fixedLength)
	st.UpdateUint("seq int.to.str", t.stats.intString)
}

func (t *SMTTheorySeq) initSearchEh() {
	clear(t.re2aut)
	t.chk(t.res.reset())
	t.closeAutomata()
}

func (t *SMTTheorySeq) mkVar(n *SMTEnode) SMTTheoryVar {
	o := n.Owner()
	if !t.isSeq(o) && !t.isRe(o) {
		return SMTNullTheoryVar
	}
	if t.IsAttachedToVariable(n) {
		return n.TheoryVar(t.FamilyID())
	}
	v, err := t.CreateVariable(n)
	t.chk(err)
	t.chk(t.state.markRelevant(o))
	return v
}

func (t *SMTTheorySeq) canPropagate() bool {
	return t.axiomsHead < t.axioms.size() || len(t.replay) != 0 || t.newSolution
}
