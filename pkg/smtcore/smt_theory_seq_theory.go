package smtcore

// Source: src/smt/theory_seq.cpp: the constructor and init (194-243),
// final_check_eh (245-319), the model functions (2616-2825) and the
// virtual theory members, as the context calls them.

import (
	"errors"
	"math/big"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

// NewSMTTheorySeq is theory_seq(m). Members that need the manager are made
// when the theory is attached to its context.
func NewSMTTheorySeq(family ast.FamilyID) (*SMTTheorySeq, error) {
	base, err := NewSMTTheoryBase(family)
	if err != nil {
		return nil, err
	}
	return &SMTTheorySeq{SMTTheoryBase: *base}, nil
}

// Initialize is theory::init(ctx) followed by the member construction of
// theory_seq's constructor and theory_seq::init.
func (t *SMTTheorySeq) Initialize(state *SMTInternalizerState) (result error) {
	if err := t.SMTTheoryBase.Initialize(state); err != nil {
		return err
	}
	m := state.manager
	t.m = m
	// The internalizer drops a theory whose init fails without closing it.
	defer func() {
		if result != nil {
			result = errors.Join(result, t.Close())
		}
	}()
	defer t.recoverErr(&result)
	t.rep = newSeqSolutionMap(m, &t.dm)
	t.eqs = seqEqVector{th: t}
	t.nqs = seqNeVector{th: t}
	t.ncs = seqNcVector{th: t}
	t.exclude = seqExclusionTable{manager: m, table: make(map[seqExprPair]struct{}), lhs: newSeqExprVector(m), rhs: newSeqExprVector(m)}
	t.axioms = newSeqExprVector(m)
	t.axiomSet = make(map[ast.ASTHandle]struct{})
	t.intString = newSeqExprVector(m)
	t.itosAxioms = make(map[string]struct{})
	t.stoiAxioms = make(map[string]struct{})
	t.length = newSeqObjHashtable(m)
	t.fixed = newSeqObjHashtable(m)
	t.ls, t.rs = newSeqExprVector(m), newSeqExprVector(m)
	t.lhsV, t.rhsV = newSeqExprVector(m), newSeqExprVector(m)
	t.res = newSeqExprVector(m)
	t.re2aut = make(map[ast.ASTHandle]*rewriter.StringRegexAutomaton)
	t.branchStart = make(map[uint32]int)
	t.tmps = seqRefs{manager: m}
	var err error
	if t.rewrite, err = newSeqRewriterOwner(m); err != nil {
		return err
	}
	if t.seqRewrite, err = rewriter.NewSequenceRewriter(m); err != nil {
		return err
	}
	t.prefix = t.symbol("seq.p.suffix")
	t.suffix = t.symbol("seq.s.prefix")
	t.accept = t.symbol("aut.accept")
	t.reject = t.symbol("aut.reject")
	t.tail = t.symbol("seq.tail")
	t.nth = t.symbol("seq.nth")
	t.seqFirst = t.symbol("seq.first")
	t.seqLast = t.symbol("seq.last")
	t.indexofLeft = t.symbol("seq.idx.left")
	t.indexofRight = t.symbol("seq.idx.right")
	t.step = t.symbol("aut.step")
	t.pre = t.symbol("seq.pre")   // (seq.pre s l): prefix of string s of length l
	t.post = t.symbol("seq.post") // (seq.post s l): suffix of string s of length l
	t.eqSym = t.symbol("seq.eq")
	return nil
}

// Setup runs the rest of theory_seq::init: m_mk_aut.set_solver(alloc(
// seq_expr_solver, m, get_fparams())). The source registers theory_seq, and
// so builds this kernel, when the context is set up for its first check or
// push; g3 installs the theory when the solver is created, so the kernel is
// built here, at the setup boundary, instead. Standalone fixtures without
// SMT parameters have no nested solver.
func (t *SMTTheorySeq) Setup() error {
	if t.exprSolver != nil || t.setupSkipped || t.state == nil || t.state.smtParams == nil {
		return nil
	}
	solver, err := newSMTStringExpressionSolver(t.m, t.state.smtParams)
	if err != nil {
		return err
	}
	t.exprSolver = solver
	return nil
}

// leave ends an entry point: it recovers the port's internal errors and
// releases the expr_ref temporaries of mk_sub.
func (t *SMTTheorySeq) leave(result *error) {
	// recover must be called by the deferred function itself.
	if r := recover(); r != nil {
		p, ok := r.(seqPanic)
		if !ok {
			panic(r)
		}
		*result = errors.Join(*result, p.err)
	}
	*result = errors.Join(*result, t.tmps.release())
}

func (*SMTTheorySeq) Name() string { return "seq" }

// FinalCheck is final_check_eh.
func (t *SMTTheorySeq) FinalCheck() (status SMTFinalCheckStatus, result error) {
	defer t.leave(&result)
	if t.resetCacheFlag {
		t.chk(t.rep.resetCache())
		t.resetCacheFlag = false
	}
	t.newPropagation = false
	if t.simplifyAndSolveEqs() {
		t.stats.solveEqs++
		return SMTFinalCheckContinue, nil
	}
	if t.checkContains() {
		t.stats.propagateContains++
		return SMTFinalCheckContinue, nil
	}
	if t.solveNqs(0) {
		t.stats.solveNqs++
		return SMTFinalCheckContinue, nil
	}
	if t.fixedLengthAll() {
		t.stats.fixedLength++
		return SMTFinalCheckContinue, nil
	}
	if t.checkIntString() {
		t.stats.intString++
		return SMTFinalCheckContinue, nil
	}
	if t.reduceLengthEqAll() {
		t.stats.branchVariable++
		return SMTFinalCheckContinue, nil
	}
	if t.branchUnitVariable() {
		t.stats.branchVariable++
		return SMTFinalCheckContinue, nil
	}
	if t.branchBinaryVariableAll() || t.branchVariableMb() {
		t.stats.branchVariable++
		return SMTFinalCheckContinue, nil
	}
	if t.branchVariable() {
		t.stats.branchVariable++
		return SMTFinalCheckContinue, nil
	}
	if t.checkLengthCoherence() {
		t.stats.checkLengthCoherence++
		return SMTFinalCheckContinue, nil
	}
	if !t.checkExtensionality() {
		t.stats.extensionality++
		return SMTFinalCheckContinue, nil
	}
	if t.propagateAutomata() {
		t.stats.propagateAutomata++
		return SMTFinalCheckContinue, nil
	}
	if t.isSolved() {
		return SMTFinalCheckDone, nil
	}
	return SMTFinalCheckGiveUp, nil
}

func (t *SMTTheorySeq) InternalizeAtom(a ast.ASTHandle, _ bool) (ok bool, result error) {
	defer t.leave(&result)
	return t.internalizeAtom(a), nil
}

func (t *SMTTheorySeq) InternalizeTerm(term ast.ASTHandle) (ok bool, result error) {
	defer t.leave(&result)
	return t.internalizeTerm(term), nil
}

// InternalizeEquality is internalize_eq_eh, which is empty.
func (t *SMTTheorySeq) InternalizeEquality(ast.ASTHandle, int32) error { return nil }

func (t *SMTTheorySeq) ApplySortConstraint(n *SMTEnode, _ ast.ASTHandle) (result error) {
	defer t.leave(&result)
	t.applySortCnstr(n)
	return nil
}

func (t *SMTTheorySeq) NewEquality(v1, v2 SMTTheoryVar) (result error) {
	defer t.leave(&result)
	t.newEqEh(v1, v2)
	return nil
}

func (t *SMTTheorySeq) NewDisequality(v1, v2 SMTTheoryVar) (result error) {
	defer t.leave(&result)
	t.newDiseqEh(v1, v2)
	return nil
}

func (t *SMTTheorySeq) Assign(v int32, isTrue bool) (result error) {
	defer t.leave(&result)
	t.assignEh(v, isTrue)
	return nil
}

func (t *SMTTheorySeq) Relevant(n ast.ASTHandle) (result error) {
	defer t.leave(&result)
	t.relevantEh(n)
	return nil
}

func (t *SMTTheorySeq) CanPropagate() bool { return t.canPropagate() }

func (t *SMTTheorySeq) Propagate() (result error) {
	defer t.leave(&result)
	t.propagateSeq()
	return nil
}

func (t *SMTTheorySeq) InitializeSearch() (result error) {
	defer t.leave(&result)
	t.initSearchEh()
	return nil
}

func (t *SMTTheorySeq) PushScope() (result error) {
	defer t.leave(&result)
	t.pushScopeEh()
	return nil
}

func (t *SMTTheorySeq) PopScopes(n uint32) (result error) {
	defer t.leave(&result)
	t.popScopeEh(int(n))
	return nil
}

func (t *SMTTheorySeq) CollectStatistics(st *statistics.Statistics) error {
	if t == nil || st == nil {
		return ast.ErrASTHandle
	}
	t.collectStatistics(st)
	return nil
}

// ---- model ----

// InitializeModel is init_model(mg).
func (t *SMTTheorySeq) InitializeModel(mg *SMTModelGenerator) (result error) {
	defer t.leave(&result)
	t.rep.pushScope()
	factory, err := protomodel.NewSequenceFactory(mg.model, t.FamilyID())
	t.chk(err)
	t.factory = factory
	factory.SetTrailSink(&mg.modelTrail)
	t.chk(mg.RegisterFactory(factory))
	for i := 0; i < t.nqs.len(); i++ {
		n := t.nqs.at(i)
		t.chk(t.factory.RegisterValue(n.l))
		t.chk(t.factory.RegisterValue(n.r))
	}
	for i := 0; i < t.nqs.len(); i++ {
		n := t.nqs.at(i)
		for j := range n.lhs {
			t.initModelVec(n.lhs[j].handles())
			t.initModelVec(n.rhs[j].handles())
		}
	}
	return nil
}

// initModelVec is init_model(es).
func (t *SMTTheorySeq) initModelVec(es []ast.ASTHandle) {
	x := t.loc()
	defer x.drop()
	for _, e := range es {
		var eqs *seqDep
		s := t.canonize(x, e, &eqs)
		if t.isVar(s) {
			newS := t.ref(x, t.must(t.factory.FreshValue(t.sortOf(s))), nil)
			t.chk(t.rep.update(s, newS, eqs))
		}
	}
}

// FinalizeModel is finalize_model.
func (t *SMTTheorySeq) FinalizeModel(*SMTModelGenerator) (result error) {
	defer t.leave(&result)
	t.chk(t.rep.popScope(1))
	return nil
}

type seqSource uint8

const (
	seqUnitSource seqSource = iota
	seqIntSource
	seqStringSource
)

// seqValueProc is theory_seq::seq_value_proc.
type seqValueProc struct {
	th           *SMTTheorySeq
	sort         ast.ASTHandle
	dependencies []SMTModelValueDependency
	strings      []ast.ASTHandle
	source       []seqSource
}

func (p *seqValueProc) addUnit(n *SMTEnode) error {
	d, err := NewSMTEnodeValueDependency(n)
	if err != nil {
		return err
	}
	p.dependencies = append(p.dependencies, d)
	p.source = append(p.source, seqUnitSource)
	return nil
}

func (p *seqValueProc) addInt(n *SMTEnode) error {
	d, err := NewSMTEnodeValueDependency(n)
	if err != nil {
		return err
	}
	p.dependencies = append(p.dependencies, d)
	p.source = append(p.source, seqIntSource)
	return nil
}

func (p *seqValueProc) addString(n ast.ASTHandle) {
	p.strings = append(p.strings, n)
	p.source = append(p.source, seqStringSource)
}

func (p *seqValueProc) Dependencies() []SMTModelValueDependency { return p.dependencies }
func (p *seqValueProc) IsFresh() bool                           { return false }

func (p *seqValueProc) MakeValue(_ *SMTModelGenerator, values []ast.ASTHandle) (value ast.ASTHandle, result error) {
	th := p.th
	defer th.leave(&result)
	x := th.loc()
	defer x.drop()
	j, k := 0, 0
	isString, err := th.m.IsStringSort(p.sort)
	th.chk(err)
	var res ast.ASTHandle
	if isString {
		var sbuffer strings.Builder
		for _, src := range p.source {
			switch src {
			case seqUnitSource:
				val, _, ok, err := th.m.BitVectorNumeralBigInt(values[j])
				th.chk(err)
				if !ok {
					th.chk(ast.ErrASTDomain)
				}
				j++
				sbuffer.WriteByte(byte(val.Uint64()))
			case seqStringSource:
				var deps *seqDep
				tmp := th.canonize(x, p.strings[k], &deps)
				if zs, ok := th.isString(tmp); ok {
					sbuffer.WriteString(zs)
				}
				k++
			case seqIntSource:
				val, ok := th.isNumeral(values[j])
				if !ok {
					th.chk(ast.ErrASTDomain)
				}
				j++
				if val.Sign() < 0 {
					sbuffer.WriteString("-")
				}
				sbuffer.WriteString(new(big.Int).Abs(val.Num()).String())
			}
		}
		res = th.ref(x, th.mkString(sbuffer.String()), nil)
	} else {
		var args []ast.ASTHandle
		for _, src := range p.source {
			switch src {
			case seqUnitSource:
				args = append(args, th.ref(x, th.mkUnit(values[j]), nil))
				j++
			case seqStringSource:
				args = append(args, p.strings[k])
				k++
			case seqIntSource:
				th.chk(ast.ErrASTDomain) // UNREACHABLE
			}
		}
		res = th.ref(x, th.mkConcatSorted(args, p.sort), nil)
		res = th.rewriteTh(x, res)
	}
	th.chk(th.factory.AddTrail(res)) // m_factory->add_trail(result)
	return res, nil
}

// ModelValue is mk_value(enode, mg).
func (t *SMTTheorySeq) ModelValue(n *SMTEnode, mg *SMTModelGenerator) (proc SMTModelValueProcedure, result error) {
	defer t.leave(&result)
	e := n.Owner()
	if t.isIte(e) {
		e2, e3 := t.arg(e, 1), t.arg(e, 2)
		n2, n3 := t.enodeOf(e2), t.enodeOf(e3)
		if n2 != nil && n3 != nil && (n2.Root() == n.Root() || n3.Root() == n.Root()) {
			if n2.Root() == n.Root() {
				return t.ModelValue(n2, mg)
			}
			return t.ModelValue(n3, mg)
		}
	}
	if t.isSeq(e) {
		var concats []ast.ASTHandle
		t.getConcatRep(e, &concats)
		sv := &seqValueProc{th: t, sort: t.sortOf(e)}
		for _, c := range concats {
			if c1, ok := t.isUnitArg(c); ok {
				if t.eInternalized(c1) {
					t.chk(sv.addUnit(t.enodeOf(c1)))
				}
			} else if c1, ok := t.unaryArg(c, strOpItos); ok {
				if t.eInternalized(c1) {
					t.chk(sv.addInt(t.enodeOf(c1)))
				}
			} else if t.isStringConst(c) {
				sv.addString(c)
			} else {
				sv.addString(t.mkValueApp(c))
			}
		}
		return sv, nil
	}
	p, err := NewSMTExpressionValueProcedure(t.mkValueApp(e))
	t.chk(err)
	return p, nil
}

// mkValueApp is mk_value(app* e).
func (t *SMTTheorySeq) mkValueApp(e ast.ASTHandle) ast.ASTHandle {
	x := t.loc()
	defer x.drop()
	result := t.ref(x, t.rep.find(e), nil)
	if t.isVar(result) {
		val, err := t.factory.SomeValue(t.sortOf(result))
		t.chk(err)
		if !val.IsNull() {
			result = t.ref(x, val, nil)
		}
	} else {
		result = t.rewriteTh(x, result)
	}
	t.chk(t.factory.AddTrail(result)) // m_factory->add_trail(result)
	t.chk(t.rep.update(e, result, nil))
	return result
}

// Close destroys the theory's members.
func (t *SMTTheorySeq) Close() (result error) {
	if t == nil || t.closed || t.m == nil {
		return nil
	}
	defer t.recoverErr(&result)
	t.closed = true
	t.trailStack, t.trailLim = nil, nil
	t.resetReplay()
	t.closeAutomata()
	result = errors.Join(result,
		t.res.reset(), t.rhsV.reset(), t.lhsV.reset(), t.rs.reset(), t.ls.reset(),
		t.seqRewrite.Close(),
		t.intString.reset(), t.axioms.reset(), t.exclude.rhs.reset(), t.exclude.lhs.reset(),
		t.rep.rhs.reset(), t.rep.lhs.reset(), t.rep.cache.reset(), t.tmps.release())
	for t.eqs.len() > 0 {
		result = errors.Join(result, t.eqs.popBack())
	}
	for t.nqs.len() > 0 {
		result = errors.Join(result, t.nqs.popBack())
	}
	for t.ncs.len() > 0 {
		result = errors.Join(result, t.ncs.popBack())
	}
	if t.rewrite != nil {
		result = errors.Join(result, t.rewrite.close())
		t.rewrite = nil
	}
	if t.exprSolver != nil {
		result = errors.Join(result, t.exprSolver.Close())
		t.exprSolver = nil
	}
	return result
}

var _ SMTTheoryInternalizer = (*SMTTheorySeq)(nil)
var _ SMTModelValueTheory = (*SMTTheorySeq)(nil)
var _ SMTModelFinalizer = (*SMTTheorySeq)(nil)
var _ = lbool.True
