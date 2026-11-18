package smtcore

// Source: retained integer fragment of src/smt/theory_diff_logic.h and
// src/smt/theory_diff_logic_def.h.

import (
	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type smtIntegerDiffAtom struct {
	variable           int32
	positive, negative int32
	truth              bool
}

type smtIntegerDiffAssertion struct {
	atom  int
	truth bool
}

type smtIntegerDiffScope struct {
	atoms, assertions int
	head              int
	nonDifference     bool
}

type smtDiffStatistics struct {
	conflicts, assertions, coreEqualities, coreDisequalities uint32
}

// SMTIntegerDiffTheory is the concrete integer difference-logic theory. It
// accepts bounds of the form x-y <= k (and the equivalent >= form), where k
// is an integer numeral. Unsupported arithmetic is recorded for final check.
type SMTIntegerDiffTheory struct {
	*SMTTheoryBase
	// q is the AST manager's mpq manager (rational's g_mpq_manager), set
	// by init; the graph's numerals and the theory's rationals use it.
	q               *numeral.RationalManager
	graph           *SMTRationalDiffGraph
	zero            SMTTheoryVar
	nonDifference   bool
	addBinaryBounds bool
	eagerEqAxioms   bool
	dumpLemmas      bool // m_params.m_arith_dump_lemmas
	equalityAdapter *smtArithmeticEqualityAdapter
	optimization    smtDiffOptimization
	statistics      smtDiffStatistics
	// graphExt is the concrete Ext graph of the shared internalization;
	// nil selects this theory's rational graph.
	graphExt smtDiffLogicGraph
	// mkNumZeroEnode keeps a zero numeral other than the init zero as its
	// own enode (mixed extension only).
	mkNumZeroEnode bool
	mkVarPending   map[ast.ASTHandle]bool
}

func (t *SMTIntegerDiffTheory) ConfigureArithmetic(config SMTArithState) error {
	if t == nil || t.equalityAdapter == nil {
		return ast.ErrASTHandle
	}
	t.addBinaryBounds = config.AddBinaryBounds
	t.eagerEqAxioms = config.EagerEqAxioms
	t.dumpLemmas = config.DumpLemmas
	t.equalityAdapter.binary = config.AddBinaryBounds
	t.equalityAdapter.lazy = config.LazyAdapter
	return nil
}

func NewSMTIntegerDiffTheory(family ast.FamilyID) (*SMTIntegerDiffTheory, error) {
	base, err := NewSMTTheoryBase(family)
	if err != nil {
		return nil, err
	}
	theory := &SMTIntegerDiffTheory{
		SMTTheoryBase: base,
		graph:         NewSMTRationalDiffGraph(nil),
		zero:          SMTNullTheoryVar,
	}
	theory.equalityAdapter, err = newSMTArithmeticEqualityAdapter(base)
	if err != nil {
		return nil, err
	}
	// theory_diff_logic uses arith_eq_adapter::mk_axioms itself.
	theory.equalityAdapter.source = true
	return theory, nil
}

// Initialize is theory_diff_logic<Ext>::init.
func (t *SMTIntegerDiffTheory) Initialize(state *SMTInternalizerState) error {
	if t == nil || t.SMTTheoryBase == nil || t.graph == nil {
		return ast.ErrASTHandle
	}
	return t.initDiffLogic(state)
}

func (t *SMTIntegerDiffTheory) integerSort(sort ast.ASTHandle) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	family, err := state.manager.GetFamilyID(sort)
	if err != nil || family != t.FamilyID() {
		return false, err
	}
	kind, err := state.manager.GetDeclKind(sort)
	return kind == 1, err
}

// ApplySortConstraint is theory::apply_sort_cnstr, which
// theory_diff_logic does not override: variables are created only by
// mk_var.
func (t *SMTIntegerDiffTheory) ApplySortConstraint(*SMTEnode, ast.ASTHandle) error {
	return nil
}

// numeral is m_util.is_numeral(expression, out): it copies the numeral's
// rational into out.
func (t *SMTIntegerDiffTheory) numeral(expression ast.ASTHandle, out *numeral.Rational) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	t.bindQ(state)
	_, matched, err := state.manager.ArithmeticNumeralRational(expression, out)
	if err != nil || !matched {
		return false, err
	}
	return true, nil
}

// bindQ sets q from the AST manager: the dense and UTVPI theories use these
// helpers without running init.
func (t *SMTIntegerDiffTheory) bindQ(state *SMTInternalizerState) {
	if t.q == nil {
		if numerals := state.manager.NumeralService(); numerals != nil {
			t.q = numerals.Manager
		}
	}
}

// isNumeral is m_util.is_numeral(expression).
func (t *SMTIntegerDiffTheory) isNumeral(expression ast.ASTHandle) (bool, error) {
	r := smtQZero()
	ok, err := t.numeral(expression, &r)
	if e := smtQDel(t.q, &r); err == nil {
		err = e
	}
	return ok, err
}

// numeralIs is m_util.is_numeral(expression, r) && r == v for v = 0, 1 or
// -1 (is_zero, is_one, is_minus_one).
func (t *SMTIntegerDiffTheory) numeralIs(expression ast.ASTHandle, v int) (bool, error) {
	r := smtQZero()
	ok, err := t.numeral(expression, &r)
	ok = ok && (v == 0 && r.IsZero() || v == 1 && r.IsOne() || v == -1 && r.IsMinusOne())
	if e := smtQDel(t.q, &r); err == nil {
		err = e
	}
	return ok, err
}

func (t *SMTIntegerDiffTheory) operation(expression ast.ASTHandle) (int32, bool, error) {
	state, err := t.State()
	if err != nil {
		return 0, false, err
	}
	kind, err := state.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return 0, false, err
	}
	family, err := state.manager.GetFamilyID(expression)
	if err != nil || family != t.FamilyID() {
		return 0, false, err
	}
	op, err := state.manager.GetDeclKind(expression)
	return op, err == nil, err
}

// offset is is_offset(n, a, k): n = a + k with a numeral k (copied into
// k).
func (t *SMTIntegerDiffTheory) offset(expression ast.ASTHandle, k *numeral.Rational) (ast.ASTHandle, bool, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	op, arithmetic, err := t.operation(expression)
	if err != nil || !arithmetic || op != smtArithOpAdd {
		return ast.ASTHandle{}, false, err
	}
	count, err := state.manager.GetNumArgs(expression)
	if err != nil || count != 2 {
		return ast.ASTHandle{}, false, err
	}
	left, err := state.manager.GetArg(expression, 0)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	right, err := state.manager.GetArg(expression, 1)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if numeral, numeralErr := t.numeral(left, k); numeralErr != nil {
		return ast.ASTHandle{}, false, numeralErr
	} else if numeral {
		return right, true, nil
	}
	if numeral, numeralErr := t.numeral(right, k); numeralErr != nil {
		return ast.ASTHandle{}, false, numeralErr
	} else if numeral {
		return left, true, nil
	}
	return ast.ASTHandle{}, false, nil
}

func (t *SMTIntegerDiffTheory) InternalizeTerm(expression ast.ASTHandle) (bool, error) {
	return t.internalizeDiffTerm(expression)
}

func (t *SMTIntegerDiffTheory) sign(expression ast.ASTHandle) (bool, bool, error) {
	value := smtQZero()
	isNumeral, err := t.numeral(expression, &value)
	one, minusOne := value.IsOne(), value.IsMinusOne()
	if e := smtQDel(t.q, &value); err == nil {
		err = e
	}
	if err != nil {
		return false, false, err
	} else if isNumeral {
		if one {
			return true, true, nil
		}
		if minusOne {
			return false, true, nil
		}
	}
	op, arithmetic, err := t.operation(expression)
	if err != nil || !arithmetic || op != smtArithOpUMinus {
		return false, false, err
	}
	state, _ := t.State()
	argument, err := state.manager.GetArg(expression, 0)
	if err != nil {
		return false, false, err
	}
	sign, matched, err := t.sign(argument)
	return !sign, matched, err
}

func (t *SMTIntegerDiffTheory) decompose(expression ast.ASTHandle) ([]ast.ASTHandle, []bool, error) {
	state, err := t.State()
	if err != nil {
		return nil, nil, err
	}
	terms := []ast.ASTHandle{expression}
	signs := []bool{true}
	for index := 0; index < len(terms); index++ {
		op, arithmetic, opErr := t.operation(terms[index])
		if opErr != nil {
			return nil, nil, opErr
		}
		if !arithmetic {
			continue
		}
		switch op {
		case smtArithOpAdd:
			parent := terms[index]
			count, countErr := state.manager.GetNumArgs(parent)
			if countErr != nil || count == 0 {
				return nil, nil, ast.ErrASTDomain
			}
			first, argErr := state.manager.GetArg(parent, 0)
			if argErr != nil {
				return nil, nil, argErr
			}
			originalSign := signs[index]
			terms[index] = first
			for arg := uint32(1); arg < count; arg++ {
				child, childErr := state.manager.GetArg(parent, arg)
				if childErr != nil {
					return nil, nil, childErr
				}
				terms = append(terms, child)
				signs = append(signs, originalSign)
			}
			index--
		case smtArithOpMul:
			count, countErr := state.manager.GetNumArgs(terms[index])
			if countErr != nil {
				return nil, nil, countErr
			}
			if count != 2 {
				continue
			}
			left, leftErr := state.manager.GetArg(terms[index], 0)
			if leftErr != nil {
				return nil, nil, leftErr
			}
			right, rightErr := state.manager.GetArg(terms[index], 1)
			if rightErr != nil {
				return nil, nil, rightErr
			}
			coefficient, matched, signErr := t.sign(left)
			term := right
			if signErr != nil {
				return nil, nil, signErr
			}
			if !matched {
				coefficient, matched, signErr = t.sign(right)
				term = left
			}
			if signErr != nil {
				return nil, nil, signErr
			}
			if matched {
				terms[index] = term
				signs[index] = signs[index] == coefficient
				index--
			}
		case smtArithOpUMinus:
			child, childErr := state.manager.GetArg(terms[index], 0)
			if childErr != nil {
				return nil, nil, childErr
			}
			terms[index], signs[index] = child, !signs[index]
			index--
		}
	}
	return terms, signs, nil
}

func (t *SMTIntegerDiffTheory) InitializeSearch() error {
	return t.equalityAdapter.InitializeSearch()
}

func (t *SMTIntegerDiffTheory) Restart() error {
	return t.equalityAdapter.Restart()
}

// InternalizeEquality is theory_diff_logic::internalize_eq_eh. Equality
// axioms are forced only for (= (+ x (* -1 y)) k), which is not a
// difference-logic term, or eagerly when both sides are theory variables;
// otherwise new_eq_eh/new_diseq_eh create them through the adapter.
func (t *SMTIntegerDiffTheory) InternalizeEquality(expression ast.ASTHandle, variable int32) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	t.traceDL("arith", "internalize_eq_eh", 276, func() string { return t.dlPP(expression) + "\n" })
	lhs, err := state.manager.GetArg(expression, 0)
	if err != nil {
		return err
	}
	rhs, err := state.manager.GetArg(expression, 1)
	if err != nil {
		return err
	}
	op, arithmetic, err := t.operation(lhs)
	if err != nil {
		return err
	}
	if arithmetic && op == smtArithOpAdd {
		count, err := state.manager.GetNumArgs(lhs)
		if err != nil {
			return err
		}
		if count == 2 {
			second, err := state.manager.GetArg(lhs, 1)
			if err != nil {
				return err
			}
			negative, err := t.isNegative(second)
			if err != nil {
				return err
			}
			isNumeral, err := t.isNumeral(rhs)
			if err != nil {
				return err
			}
			if negative && isNumeral {
				// force axioms for (= (+ x (* -1 y)) k)
				return t.equalityAdapter.mkAxiomsSource(state.context.enodes.get(lhs), state.context.enodes.get(rhs))
			}
		}
	}
	if t.eagerEqAxioms {
		n1 := state.context.enodes.get(lhs)
		n2 := state.context.enodes.get(rhs)
		if n1 != nil && n2 != nil && n1 != n2 &&
			n1.TheoryVar(t.FamilyID()) != SMTNullTheoryVar && n2.TheoryVar(t.FamilyID()) != SMTNullTheoryVar {
			return t.equalityAdapter.mkAxiomsSource(n1, n2)
		}
	}
	return nil
}

// isNegative is theory_diff_logic::is_negative: (* -1 y), (* y -1),
// or the same products with (- 1) as the coefficient.
func (t *SMTIntegerDiffTheory) isNegative(expression ast.ASTHandle) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	op, arithmetic, err := t.operation(expression)
	if err != nil || !arithmetic || op != smtArithOpMul {
		return false, err
	}
	if count, err := state.manager.GetNumArgs(expression); err != nil || count != 2 {
		return false, err
	}
	a0, err := state.manager.GetArg(expression, 0)
	if err != nil {
		return false, err
	}
	a1, err := state.manager.GetArg(expression, 1)
	if err != nil {
		return false, err
	}
	isApp := func(h ast.ASTHandle) (bool, error) {
		kind, err := state.manager.GetKind(h)
		return kind == ast.ASTApp, err
	}
	if numeral, err := t.isNumeral(a1); err != nil {
		return false, err
	} else if numeral {
		a0, a1 = a1, a0
	}
	if minusOne, err := t.numeralIs(a0, -1); err != nil {
		return false, err
	} else if minusOne {
		return isApp(a1)
	}
	if op, arithmetic, err := t.operation(a1); err != nil {
		return false, err
	} else if arithmetic && op == smtArithOpUMinus {
		a0, a1 = a1, a0
	}
	op, arithmetic, err = t.operation(a0)
	if err != nil || !arithmetic || op != smtArithOpUMinus {
		return false, err
	}
	if count, err := state.manager.GetNumArgs(a0); err != nil || count != 1 {
		return false, err
	}
	a2, err := state.manager.GetArg(a0, 0)
	if err != nil {
		return false, err
	}
	if one, err := t.numeralIs(a2, 1); err != nil || !one {
		return false, err
	}
	return isApp(a1)
}

func (t *SMTIntegerDiffTheory) NewEquality(left, right SMTTheoryVar) error {
	return t.equalityAdapter.NewEquality(left, right)
}
func (t *SMTIntegerDiffTheory) NewDisequality(left, right SMTTheoryVar) error {
	return t.equalityAdapter.NewDisequality(left, right)
}

func (t *SMTIntegerDiffTheory) Graph() *SMTRationalDiffGraph {
	if t == nil {
		return nil
	}
	return t.graph
}

func (*SMTIntegerDiffTheory) Name() string { return "difference-logic" }

func (t *SMTIntegerDiffTheory) InitializeModel(generator *SMTModelGenerator) error {
	if t == nil || generator == nil {
		return ast.ErrASTHandle
	}
	state, err := t.State()
	if err != nil {
		return err
	}
	if generator.manager != state.manager {
		return ast.ErrASTDomain
	}
	factory, err := protomodel.NewArithmeticFactory(state.manager)
	if err != nil {
		return err
	}
	return generator.RegisterFactory(factory)
}
