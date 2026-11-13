package smtcore

// Source: internalization of src/smt/theory_diff_logic_def.h, shared by the
// concrete graph instantiations (the rational graph of SMTIntegerDiffTheory
// and the inf-rational graph of SMTRealDiffTheory):
// init:67-75, internalize_term:79-87, found_non_diff_logic_expr:158-165,
// internalize_atom:168-271 (up to the graph edges), is_offset:700-716,
// mk_term:719-771, mk_num:775-800, mk_var(enode*):804-811 and
// mk_var(app*):814-836.

import (
	"math/big"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// smtDiffLogicGraph is the Ext::numeral dependent part of the shared
// internalization: m_graph.init_var, and the pair of enabled, unexplained
// edges that mk_num (v = k) and mk_term (n = a + k) add.
type smtDiffLogicGraph interface {
	diffGraphInitVar(v SMTTheoryVar) error
	diffGraphEnableFixedEdges(source, target SMTTheoryVar, k *big.Rat) error
}

func (t *SMTIntegerDiffTheory) diffGraphInitVar(v SMTTheoryVar) error {
	return t.graph.InitVariable(int32(v))
}

// diffGraphEnableFixedEdges is
//
//	m_graph.enable_edge(m_graph.add_edge(source, target, k, null_literal));
//	m_graph.enable_edge(m_graph.add_edge(target, source, -k, null_literal));
//
// The source ignores enable_edge's feasibility result.
func (t *SMTIntegerDiffTheory) diffGraphEnableFixedEdges(source, target SMTTheoryVar, k *big.Rat) error {
	forward, err := t.graph.AddEdge(int32(source), int32(target), k, SMTNullLiteral)
	if err != nil {
		return err
	}
	if _, err = t.graph.EnableEdge(forward); err != nil {
		return err
	}
	negative := new(big.Rat).Neg(k)
	backward, err := t.graph.AddEdge(int32(target), int32(source), negative, SMTNullLiteral)
	if err != nil {
		return err
	}
	_, err = t.graph.EnableEdge(backward)
	return err
}

func (t *SMTIntegerDiffTheory) diffGraph() (smtDiffLogicGraph, error) {
	if t == nil {
		return nil, ast.ErrASTHandle
	}
	if t.graphExt == nil {
		return t, nil
	}
	return t.graphExt, nil
}

// initDiffLogic is theory_diff_logic<Ext>::init.
func (t *SMTIntegerDiffTheory) initDiffLogic(state *SMTInternalizerState) error {
	if err := t.SMTTheoryBase.Initialize(state); err != nil {
		return err
	}
	zero, err := state.manager.ArithmeticNumeralRat(new(big.Rat), true)
	if err != nil {
		return err
	}
	node := state.context.enodes.get(zero)
	if node == nil {
		node, err = state.MkEnode(zero, false, false, true)
		if err != nil {
			return err
		}
	} else if t.IsAttachedToVariable(node) {
		// The source asserts !is_attached_to_var(e).
		t.zero = node.TheoryVar(t.FamilyID())
		return nil
	}
	t.zero, err = t.mkVarEnode(node)
	return err
}

// mkVarEnode is theory_diff_logic<Ext>::mk_var(enode*): theory::mk_var,
// m_graph.init_var and context::attach_th_var. Like the source, it does not
// test whether n already has a variable of this theory; attach_th_var then
// replaces it.
func (t *SMTIntegerDiffTheory) mkVarEnode(node *SMTEnode) (SMTTheoryVar, error) {
	state, err := t.State()
	if err != nil {
		return SMTNullTheoryVar, err
	}
	graph, err := t.diffGraph()
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if node == nil || node.manager != state.manager {
		return SMTNullTheoryVar, ast.ErrASTDomain
	}
	v := SMTTheoryVar(len(t.variables))
	t.variables = append(t.variables, node)
	if smtTraceEnabled("diff_logic_vars") {
		emitSMTTrace("diff_logic_vars", "mk_var", "src/smt/theory_diff_logic_def.h", 806, "mk_var: "+strconv.Itoa(int(v))+"\n")
	}
	if err = graph.diffGraphInitVar(v); err != nil {
		t.variables[v] = nil
		t.variables = t.variables[:v]
		return SMTNullTheoryVar, err
	}
	if err = state.attachTheoryVariable(node, t.family, v); err != nil {
		t.variables[v] = nil
		t.variables = t.variables[:v]
		return SMTNullTheoryVar, err
	}
	return v, nil
}

// mkVarApp is theory_diff_logic<Ext>::mk_var(app*).
func (t *SMTIntegerDiffTheory) mkVarApp(n ast.ASTHandle) (SMTTheoryVar, error) {
	state, err := t.State()
	if err != nil {
		return SMTNullTheoryVar, err
	}
	v := SMTNullTheoryVar
	node := state.context.enodes.get(n)
	if node != nil {
		v = node.TheoryVar(t.FamilyID())
	} else {
		if t.mkVarPending[n] {
			// mk_term reaches mk_var(n) for an arithmetic term n that it does
			// not decompose (to_real, to_int, ...); the source re-enters
			// ctx.internalize(n) without bound and never returns.
			return SMTNullTheoryVar, ast.ErrASTDomain
		}
		if t.mkVarPending == nil {
			t.mkVarPending = make(map[ast.ASTHandle]bool)
		}
		t.mkVarPending[n] = true
		err = state.Internalize(n, false)
		delete(t.mkVarPending, n)
		if err != nil {
			return SMTNullTheoryVar, err
		}
		node = state.context.enodes.get(n)
		if node == nil {
			// The source dereferences the missing enode.
			return SMTNullTheoryVar, ast.ErrASTDomain
		}
	}
	if v == SMTNullTheoryVar {
		if v, err = t.mkVarEnode(node); err != nil {
			return SMTNullTheoryVar, err
		}
	}
	// is_interpreted(n): get_family_id() == n->get_family_id().
	family, err := state.manager.GetFamilyID(n)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if family == t.FamilyID() {
		if smtTraceEnabled("non_diff_logic") {
			emitSMTTrace("non_diff_logic", "mk_var", "src/smt/theory_diff_logic_def.h", 830, "Variable should not be interpreted\n")
		}
		if err = t.foundNonDiffLogicExpr(n); err != nil {
			return SMTNullTheoryVar, err
		}
	}
	return v, nil
}

// foundNonDiffLogicExpr is theory_diff_logic<Ext>::found_non_diff_logic_expr.
// The source records the old value in a value_trail; the flag only moves
// from false to true, so the per-scope copy that PopScopes restores is the
// same undo.
func (t *SMTIntegerDiffTheory) foundNonDiffLogicExpr(n ast.ASTHandle) error {
	if t.nonDifference {
		return nil
	}
	state, err := t.State()
	if err != nil {
		return err
	}
	if smtTraceEnabled("non_diff_logic") {
		var body string
		tracePPTo(state.manager, n, func(text string) { body = text })
		emitSMTTrace("non_diff_logic", "found_non_diff_logic_expr", "src/smt/theory_diff_logic_def.h", 160,
			"found non diff logic expression:\n"+body+"\n")
	}
	// IF_VERBOSE(0, verbose_stream() << "(smt.diff_logic: non-diff logic
	// expression " << mk_pp(n, get_manager()) << ")\n";)
	enabled, err := diagnostic.ProcessVerboseEnabled(0)
	if err != nil {
		return err
	}
	if enabled {
		stream, err := diagnostic.BorrowProcessVerboseStream()
		if err != nil {
			return err
		}
		if err = stream.AppendCString("(smt.diff_logic: non-diff logic expression "); err != nil {
			return err
		}
		var writeErr error
		tracePPTo(state.manager, n, func(text string) {
			if writeErr == nil {
				writeErr = stream.AppendCString(text)
			}
		})
		if writeErr != nil {
			return writeErr
		}
		if err = stream.AppendCString(")\n"); err != nil {
			return err
		}
		if err = stream.Flush(); err != nil {
			return err
		}
	}
	t.nonDifference = true
	return nil
}

// internalizeDiffTerm is theory_diff_logic<Ext>::internalize_term.
func (t *SMTIntegerDiffTheory) internalizeDiffTerm(term ast.ASTHandle) (bool, error) {
	v, err := t.mkTerm(term)
	if err != nil {
		return false, err
	}
	result := v != SMTNullTheoryVar
	if !result {
		if smtTraceEnabled("non_diff_logic") {
			emitSMTTrace("non_diff_logic", "internalize_term", "src/smt/theory_diff_logic_def.h", 83, "Terms may not be internalized\n")
		}
		if err = t.foundNonDiffLogicExpr(term); err != nil {
			return false, err
		}
	}
	return result, nil
}

// mkTerm is theory_diff_logic<Ext>::mk_term.
func (t *SMTIntegerDiffTheory) mkTerm(n ast.ASTHandle) (SMTTheoryVar, error) {
	state, err := t.State()
	if err != nil {
		return SMTNullTheoryVar, err
	}
	r, numeral, err := t.numeral(n)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if numeral {
		return t.mkNum(n, r)
	}
	a, r, offset, err := t.offset(n)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if offset {
		// n = a + k
		source, err := t.mkVarApp(a)
		if err != nil {
			return SMTNullTheoryVar, err
		}
		count, err := state.manager.GetNumArgs(n)
		if err != nil {
			return SMTNullTheoryVar, err
		}
		for i := uint32(0); i < count; i++ {
			arg, err := state.manager.GetArg(n, i)
			if err != nil {
				return SMTNullTheoryVar, err
			}
			if state.context.enodes.get(arg) == nil {
				if err = state.Internalize(arg, false); err != nil {
					return SMTNullTheoryVar, err
				}
			}
		}
		// ctx.mk_enode(n, ...) on an internalized n is outside the source
		// contract; MkEnode refuses it.
		e, err := state.MkEnode(n, false, false, true)
		if err != nil {
			return SMTNullTheoryVar, err
		}
		target, err := t.mkVarEnode(e)
		if err != nil {
			return SMTNullTheoryVar, err
		}
		graph, err := t.diffGraph()
		if err != nil {
			return SMTNullTheoryVar, err
		}
		// target - source <= k, source - target <= -k
		if err = graph.diffGraphEnableFixedEdges(source, target, r); err != nil {
			return SMTNullTheoryVar, err
		}
		return target, nil
	}
	op, arithmetic, err := t.operation(n)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if arithmetic {
		switch op {
		case smtArithOpAdd, smtArithOpMul, smtArithOpDiv, smtArithOpIDiv, smtArithOpMod, smtArithOpRem:
			return SMTNullTheoryVar, nil
		}
	}
	return t.mkVarApp(n)
}

// mkNum is theory_diff_logic<Ext>::mk_num.
func (t *SMTIntegerDiffTheory) mkNum(n ast.ASTHandle, r *big.Rat) (SMTTheoryVar, error) {
	state, err := t.State()
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if r.Sign() == 0 && !t.mkNumZeroEnode {
		return t.zero, nil
	}
	if e := state.context.enodes.get(n); e != nil {
		return e.TheoryVar(t.FamilyID()), nil
	}
	zero := t.zero
	e, err := state.MkEnode(n, false, false, true)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	v, err := t.mkVarEnode(e)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	graph, err := t.diffGraph()
	if err != nil {
		return SMTNullTheoryVar, err
	}
	// v = k: v - zero <= k, zero - v <= - k
	if err = graph.diffGraphEnableFixedEdges(zero, v, r); err != nil {
		return SMTNullTheoryVar, err
	}
	return v, nil
}

// smtDiffAtomBound is the Ext-independent part of internalize_atom: the bound
// target - source <= k and its fresh Boolean variable.
type smtDiffAtomBound struct {
	source, target SMTTheoryVar
	k              big.Rat
	variable       int32
	literal        SMTLiteral
	integer        bool // m_util.is_int(lhs)
}

// internalizeDiffAtom is theory_diff_logic<Ext>::internalize_atom up to the
// m_arith_add_binary_bounds axioms. It reports false where the source
// returns false.
func (t *SMTIntegerDiffTheory) internalizeDiffAtom(n ast.ASTHandle) (smtDiffAtomBound, bool, error) {
	var atom smtDiffAtomBound
	state, err := t.State()
	if err != nil {
		return atom, false, err
	}
	op, arithmetic, err := t.operation(n)
	if err != nil {
		return atom, false, err
	}
	if !arithmetic || op != smtArithOpLE && op != smtArithOpGE {
		return atom, false, t.foundNonDiffLogicExpr(n)
	}
	isGE := op == smtArithOpGE
	lhs, err := state.manager.GetArg(n, 0)
	if err != nil {
		return atom, false, err
	}
	rhs, err := state.manager.GetArg(n, 1)
	if err != nil {
		return atom, false, err
	}
	if _, numeral, err := t.numeral(rhs); err != nil {
		return atom, false, err
	} else if !numeral {
		lhs, rhs = rhs, lhs
		isGE = !isGE
	}
	kr, numeral, err := t.numeral(rhs)
	if err != nil {
		return atom, false, err
	}
	if !numeral {
		return atom, false, t.foundNonDiffLogicExpr(n)
	}
	atom.k.Set(kr)
	terms, signs, err := t.decompose(lhs)
	if err != nil {
		return atom, false, err
	}
	if len(terms) == 2 && signs[0] != signs[1] {
		if atom.target, err = t.mkVarApp(terms[0]); err != nil {
			return atom, false, err
		}
		if atom.source, err = t.mkVarApp(terms[1]); err != nil {
			return atom, false, err
		}
		if !signs[0] {
			atom.target, atom.source = atom.source, atom.target
		}
	} else {
		if atom.target, err = t.mkVarApp(lhs); err != nil {
			return atom, false, err
		}
		atom.source = t.zero
	}
	if isGE {
		atom.target, atom.source = atom.source, atom.target
		atom.k.Neg(&atom.k)
	}
	if atom.variable, err = state.MkBoolVar(n); err != nil {
		return atom, false, err
	}
	if err = state.SetVariableTheory(atom.variable, t.FamilyID()); err != nil {
		return atom, false, err
	}
	atom.literal = NewSMTLiteral(atom.variable, false)
	sort, err := state.manager.GetSort(lhs)
	if err != nil {
		return atom, false, err
	}
	if atom.integer, err = t.integerSort(sort); err != nil {
		return atom, false, err
	}
	return atom, true, nil
}
