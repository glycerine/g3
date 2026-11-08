package smtcore

// Shared helpers for the theory_seq port: the error convention, expr_ref
// locals, and the seq_util / arith_util / ast_manager calls the source makes.

import (
	"errors"
	"math/big"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// seqPanic carries an error out of the port's internals. Every exported
// theory entry point recovers it, so the source's straight-line code stays
// straight-line here.
type seqPanic struct{ err error }

func (t *SMTTheorySeq) chk(err error) {
	if err != nil {
		panic(seqPanic{err})
	}
}

func (t *SMTTheorySeq) must(h ast.ASTHandle, err error) ast.ASTHandle {
	t.chk(err)
	return h
}

func (t *SMTTheorySeq) mustBool(b bool, err error) bool {
	t.chk(err)
	return b
}

// recoverErr converts a seqPanic into the entry point's error result.
func (t *SMTTheorySeq) recoverErr(result *error) {
	if r := recover(); r != nil {
		p, ok := r.(seqPanic)
		if !ok {
			panic(r)
		}
		*result = errors.Join(*result, p.err)
	}
}

// loc starts the expr_ref locals of one source function; drop releases them
// in reverse order when the function returns.
func (t *SMTTheorySeq) loc() *seqRefs { return &seqRefs{manager: t.m} }

func (x *seqRefs) drop() {
	if err := x.release(); err != nil {
		panic(seqPanic{err})
	}
}

// ref is expr_ref r(e, m): hold e for the rest of the function.
func (t *SMTTheorySeq) ref(x *seqRefs, h ast.ASTHandle, err error) ast.ASTHandle {
	t.chk(err)
	h, err = x.hold(h)
	t.chk(err)
	return h
}

// ---- ast_manager ----

func (t *SMTTheorySeq) sortOf(e ast.ASTHandle) ast.ASTHandle { return t.must(t.m.GetSort(e)) }

func (t *SMTTheorySeq) arg(e ast.ASTHandle, i uint32) ast.ASTHandle { return t.must(t.m.GetArg(e, i)) }

func (t *SMTTheorySeq) numArgs(e ast.ASTHandle) uint32 {
	n, err := t.m.GetNumArgs(e)
	t.chk(err)
	return n
}

func (t *SMTTheorySeq) isApp(e ast.ASTHandle) bool {
	k, err := t.m.GetKind(e)
	t.chk(err)
	return k == ast.ASTApp
}

func (t *SMTTheorySeq) astID(e ast.ASTHandle) uint32 {
	id, err := t.m.GetASTID(e)
	t.chk(err)
	return id
}

func (t *SMTTheorySeq) mkEqExpr(a, b ast.ASTHandle) ast.ASTHandle { return t.must(t.m.MkEq(a, b)) }

func (t *SMTTheorySeq) mkNotExpr(a ast.ASTHandle) ast.ASTHandle { return t.must(t.m.MkNot(a)) }

func (t *SMTTheorySeq) areDistinct(a, b ast.ASTHandle) bool { return t.mustBool(t.m.AreDistinct(a, b)) }

func (t *SMTTheorySeq) areEqual(a, b ast.ASTHandle) bool { return t.mustBool(t.m.AreEqual(a, b)) }

// ---- seq_decl_plugin.h seq_op_kind ----

const (
	seqOpUnit int32 = iota
	seqOpEmpty
	seqOpConcat
	seqOpPrefix
	seqOpSuffix
	seqOpContains
	seqOpExtract
	seqOpReplace
	seqOpAt
	seqOpLength
	seqOpIndex
	seqOpToRe
	seqOpInRe
	reOpPlus
	reOpStar
	reOpOption
	reOpRange
	reOpConcat
	reOpUnion
	reOpIntersect
	reOpLoop
	reOpComplement
	reOpEmptySet
	reOpFullSeqSet
	reOpFullCharSet
	reOpOfPred
	strOpConst
	strOpItos
	strOpStoi
)

// ---- seq_util ----

func (t *SMTTheorySeq) isSeqSort(s ast.ASTHandle) bool {
	_, ok, err := t.m.SequenceSortElement(s)
	t.chk(err)
	return ok
}

// isSeq is m_util.is_seq(e).
func (t *SMTTheorySeq) isSeq(e ast.ASTHandle) bool { return t.isSeqSort(t.sortOf(e)) }

// seqElemSort is m_util.is_seq(sort, char_sort).
func (t *SMTTheorySeq) seqElemSort(s ast.ASTHandle) ast.ASTHandle {
	elem, ok, err := t.m.SequenceSortElement(s)
	t.chk(err)
	if !ok {
		t.chk(ast.ErrASTDomain)
	}
	return elem
}

// isRe is m_util.is_re(e).
func (t *SMTTheorySeq) isRe(e ast.ASTHandle) bool {
	_, ok, err := t.m.RegexSequenceSort(t.sortOf(e))
	t.chk(err)
	return ok
}

func (t *SMTTheorySeq) isStrOp(e ast.ASTHandle, kind int32) bool {
	return t.mustBool(t.m.IsSequenceOperation(e, kind))
}

func (t *SMTTheorySeq) isEmpty(e ast.ASTHandle) bool { return t.mustBool(t.m.IsSequenceEmpty(e)) }
func (t *SMTTheorySeq) isUnit(e ast.ASTHandle) bool  { return t.mustBool(t.m.IsSequenceUnit(e)) }

func (t *SMTTheorySeq) isConcat(e ast.ASTHandle) bool { return t.mustBool(t.m.IsSequenceConcat(e)) }

// isConcat2 is str.is_concat(e, e1, e2) on a binary concat.
func (t *SMTTheorySeq) isConcat2(e ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool) {
	l, r, ok, err := t.m.SequenceConcatParts(e)
	t.chk(err)
	return l, r, ok
}

// isString is str.is_string(e, s).
func (t *SMTTheorySeq) isString(e ast.ASTHandle) (string, bool) {
	s, ok, err := t.m.SequenceStringValue(e)
	t.chk(err)
	return s, ok
}

func (t *SMTTheorySeq) isStringConst(e ast.ASTHandle) bool {
	_, ok := t.isString(e)
	return ok
}

// isUnitArg is str.is_unit(e, a).
func (t *SMTTheorySeq) isUnitArg(e ast.ASTHandle) (ast.ASTHandle, bool) {
	a, ok, err := t.m.SequenceUnitArgument(e)
	t.chk(err)
	return a, ok
}

func (t *SMTTheorySeq) unaryArg(e ast.ASTHandle, kind int32) (ast.ASTHandle, bool) {
	a, ok, err := t.m.SequenceUnaryOperationArgument(e, kind)
	t.chk(err)
	return a, ok
}

func (t *SMTTheorySeq) binaryArgs(e ast.ASTHandle, kind int32) (ast.ASTHandle, ast.ASTHandle, bool) {
	a, b, ok, err := t.m.SequenceBinaryOperationArguments(e, kind)
	t.chk(err)
	return a, b, ok
}

func (t *SMTTheorySeq) mkEmpty(s ast.ASTHandle) ast.ASTHandle { return t.must(t.m.MkSequenceEmpty(s)) }
func (t *SMTTheorySeq) mkUnit(e ast.ASTHandle) ast.ASTHandle  { return t.must(t.m.MkSequenceUnit(e)) }
func (t *SMTTheorySeq) mkLength(e ast.ASTHandle) ast.ASTHandle {
	return t.must(t.m.MkSequenceLength(e))
}
func (t *SMTTheorySeq) mkString(s string) ast.ASTHandle { return t.must(t.m.MkString(s)) }

// mkChar is str.mk_char(s, idx): the 8-bit character of an ascii zstring.
func (t *SMTTheorySeq) mkChar(s string, idx int) ast.ASTHandle {
	return t.must(t.m.BitVectorNumeralUint64(uint64(s[idx]), 8))
}

func (t *SMTTheorySeq) mkConcat2(a, b ast.ASTHandle) ast.ASTHandle {
	return t.must(t.m.MkSequenceConcat(a, b))
}

// mkConcatN is str.mk_concat(n, es): es[0] for one element, otherwise the
// right-associative concat, built innermost first as mk_app does.
func (t *SMTTheorySeq) mkConcatN(es []ast.ASTHandle) ast.ASTHandle {
	if len(es) == 1 {
		return es[0]
	}
	j := len(es) - 1
	r := t.mkConcat2(es[j-1], es[j])
	for j--; j > 0; {
		j--
		r = t.mkConcat2(es[j], r)
	}
	return r
}

// mkConcat3 is str.mk_concat(a, b, c) = mk_concat(a, mk_concat(b, c)).
func (t *SMTTheorySeq) mkConcat3(a, b, c ast.ASTHandle) ast.ASTHandle {
	return t.mkConcat2(a, t.mkConcat2(b, c))
}

// mkConcatSorted is theory_seq::mk_concat(es, s): empty of sort s when es
// is empty.
func (t *SMTTheorySeq) mkConcatSorted(es []ast.ASTHandle, s ast.ASTHandle) ast.ASTHandle {
	if len(es) == 0 {
		return t.mkEmpty(s)
	}
	return t.mkConcatN(es)
}

func (t *SMTTheorySeq) getConcat(e ast.ASTHandle) []ast.ASTHandle {
	es, err := t.m.SequenceFlattenConcat(e)
	t.chk(err)
	return es
}

// ---- arith_util ----

func (t *SMTTheorySeq) mkInt(n int64) ast.ASTHandle {
	return t.must(t.m.ArithmeticIntegerNumeralText(strconv.FormatInt(n, 10)))
}

func (t *SMTTheorySeq) mkIntRat(r *big.Rat) ast.ASTHandle {
	return t.must(t.m.ArithmeticIntegerNumeralText(r.RatString()))
}

func (t *SMTTheorySeq) mkLe(a, b ast.ASTHandle) ast.ASTHandle {
	return t.must(t.m.MkArithmeticLE(a, b))
}
func (t *SMTTheorySeq) mkGe(a, b ast.ASTHandle) ast.ASTHandle {
	return t.must(t.m.MkArithmeticGE(a, b))
}
func (t *SMTTheorySeq) mkLt(a, b ast.ASTHandle) ast.ASTHandle {
	return t.must(t.m.MkArithmeticLT(a, b))
}
func (t *SMTTheorySeq) mkAdd(a, b ast.ASTHandle) ast.ASTHandle {
	return t.must(t.m.MkArithmeticAdd(a, b))
}
func (t *SMTTheorySeq) mkSubExpr(a, b ast.ASTHandle) ast.ASTHandle {
	return t.must(t.m.MkArithmeticSub(a, b))
}
func (t *SMTTheorySeq) mkMul(a, b ast.ASTHandle) ast.ASTHandle {
	return t.must(t.m.MkArithmeticMul(a, b))
}

// isNumeral is m_autil.is_numeral(e, r).
func (t *SMTTheorySeq) isNumeral(e ast.ASTHandle) (*big.Rat, bool) {
	text, _, matched, err := t.m.ArithmeticNumeralText(e)
	t.chk(err)
	if !matched {
		return nil, false
	}
	r, ok := new(big.Rat).SetString(text)
	return r, ok
}

// ---- context ----

// mkEq is theory::mk_eq(a, b, gate_ctx).
func (t *SMTTheorySeq) mkEq(a, b ast.ASTHandle, gate bool) SMTLiteral {
	l, err := t.MkEqualityLiteral(a, b, gate)
	t.chk(err)
	return l
}

func (t *SMTTheorySeq) assignment(l SMTLiteral) lbool.LBool {
	v, err := t.state.Assignment(l)
	t.chk(err)
	return v
}

func (t *SMTTheorySeq) markLitRelevant(l SMTLiteral) {
	e, err := t.state.BoolExpression(l.Var())
	t.chk(err)
	t.chk(t.state.markRelevant(e))
}

// forcePhase is context::force_phase(l).
func (t *SMTTheorySeq) forcePhase(l SMTLiteral) {
	data := &t.state.booleanData[l.Var()]
	data.PhaseAvailable = true
	data.Phase = !l.Sign()
}

func (t *SMTTheorySeq) atBaseLevel() bool { return t.state.scopeLevel == t.state.baseLevel }

func (t *SMTTheorySeq) enodeOf(e ast.ASTHandle) *SMTEnode { return t.state.context.enodes.get(e) }

func (t *SMTTheorySeq) eInternalized(e ast.ASTHandle) bool { return t.enodeOf(e) != nil }

// ---- trail ----

func (t *SMTTheorySeq) pushTrail(undo seqTrail) { t.trailStack = append(t.trailStack, undo) }

// ---- theory_seq::is_var ----

func (t *SMTTheorySeq) isVar(a ast.ASTHandle) bool {
	if !t.isSeq(a) || t.isConcat(a) || t.isEmpty(a) || t.isStringConst(a) || t.isUnit(a) {
		return false
	}
	if t.isStrOp(a, strOpItos) {
		return false
	}
	return !t.isIte(a)
}

// isIte is m.is_ite(e).
func (t *SMTTheorySeq) isIte(e ast.ASTHandle) bool {
	op, basic, err := t.state.basicOperation(e)
	t.chk(err)
	return basic && op == ast.ASTOpIte
}

// symbol is symbol(name).
func (t *SMTTheorySeq) symbol(name string) usymbol.Symbol {
	s, err := t.m.Symbols.Intern(name)
	t.chk(err)
	return s
}

// insertObjTrail is m_trail_stack.push(insert_obj_trail(table, e)) together
// with the insertion; the undo removes e.
func (t *SMTTheorySeq) insertObjTrail(table *seqObjHashtable, e ast.ASTHandle) {
	t.pushTrail(func(*SMTTheorySeq) error { return table.remove(e) })
	t.chk(table.insert(e))
}

// pushReplay is m_trail_stack.push(push_replay(alloc(apply, m, e))): the
// action holds e and is queued on m_replay when the scope is popped.
func (t *SMTTheorySeq) pushReplay(kind seqApplyKind, e ast.ASTHandle) {
	ref, err := t.m.NewRef(e)
	t.chk(err)
	a := seqApply{kind: kind, e: e, ref: ref}
	t.pushTrail(func(th *SMTTheorySeq) error {
		th.replay = append(th.replay, a)
		return nil
	})
}

// rewriteTh is m_rewrite(e) for an expr_ref e: the rewritten term, which
// the caller holds.
func (t *SMTTheorySeq) rewriteTh(e ast.ASTHandle) ast.ASTHandle {
	r, err := t.rewrite.rewriterFor(t.m)
	t.chk(err)
	out, err := t.m.NewRef(ast.ASTHandle{})
	t.chk(err)
	defer func() { t.chk(t.m.DelRef(&out)) }()
	t.chk(r.Rewrite(e, out))
	return t.must(t.m.RefGet(out))
}
