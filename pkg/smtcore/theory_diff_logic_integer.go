package smtcore

// Source: retained integer fragment of src/smt/theory_diff_logic.h and
// src/smt/theory_diff_logic_def.h.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
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
	graph           *SMTRationalDiffGraph
	zero            SMTTheoryVar
	atoms           []smtIntegerDiffAtom
	atomByBool      map[int32]int
	assertions      []smtIntegerDiffAssertion
	assertionHead   int
	scopes          []smtIntegerDiffScope
	nonDifference   bool
	addBinaryBounds bool
	eagerEqAxioms   bool
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
		graph:         NewSMTRationalDiffGraph(),
		zero:          SMTNullTheoryVar,
		atomByBool:    make(map[int32]int),
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

func (t *SMTIntegerDiffTheory) numeral(expression ast.ASTHandle) (*big.Rat, bool, error) {
	state, err := t.State()
	if err != nil {
		return nil, false, err
	}
	value, _, matched, err := state.manager.ArithmeticNumeralBigRat(expression)
	if err != nil || !matched {
		if matched {
			return nil, false, ast.ErrASTDomain
		}
		return nil, false, err
	}
	return value, true, nil
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

func (t *SMTIntegerDiffTheory) offset(expression ast.ASTHandle) (ast.ASTHandle, *big.Rat, bool, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, nil, false, err
	}
	op, arithmetic, err := t.operation(expression)
	if err != nil || !arithmetic || op != smtArithOpAdd {
		return ast.ASTHandle{}, nil, false, err
	}
	count, err := state.manager.GetNumArgs(expression)
	if err != nil || count != 2 {
		return ast.ASTHandle{}, nil, false, err
	}
	left, err := state.manager.GetArg(expression, 0)
	if err != nil {
		return ast.ASTHandle{}, nil, false, err
	}
	right, err := state.manager.GetArg(expression, 1)
	if err != nil {
		return ast.ASTHandle{}, nil, false, err
	}
	if value, numeral, numeralErr := t.numeral(left); numeralErr != nil {
		return ast.ASTHandle{}, nil, false, numeralErr
	} else if numeral {
		return right, value, true, nil
	}
	if value, numeral, numeralErr := t.numeral(right); numeralErr != nil {
		return ast.ASTHandle{}, nil, false, numeralErr
	} else if numeral {
		return left, value, true, nil
	}
	return ast.ASTHandle{}, nil, false, nil
}

func (t *SMTIntegerDiffTheory) addBoundAxioms(source, target SMTTheoryVar, weight *big.Rat, literal SMTLiteral) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	equal, lower, upper := SMTNullLiteral, SMTNullLiteral, SMTNullLiteral
	var lowerWeight, upperWeight big.Rat
	for _, edgeID := range t.graph.outEdges[source] {
		edge := &t.graph.edges[edgeID]
		if edge.target != int32(target) || edge.explanation == SMTNullLiteral {
			continue
		}
		switch edge.weight.Cmp(weight) {
		case 0:
			equal = edge.explanation
		case -1:
			if lower == SMTNullLiteral || edge.weight.Cmp(&lowerWeight) > 0 {
				lower, lowerWeight = edge.explanation, edge.weight
			}
		case 1:
			if upper == SMTNullLiteral || edge.weight.Cmp(&upperWeight) < 0 {
				upper, upperWeight = edge.explanation, edge.weight
			}
		}
	}
	if equal != SMTNullLiteral {
		if err = state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{equal.Negated(), literal}); err != nil {
			return err
		}
		return state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{literal.Negated(), equal})
	}
	if lower != SMTNullLiteral {
		if err = state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{lower.Negated(), literal}); err != nil {
			return err
		}
	}
	if upper != SMTNullLiteral {
		return state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{literal.Negated(), upper})
	}
	return nil
}

func (t *SMTIntegerDiffTheory) InternalizeTerm(expression ast.ASTHandle) (bool, error) {
	return t.internalizeDiffTerm(expression)
}

func (t *SMTIntegerDiffTheory) sign(expression ast.ASTHandle) (bool, bool, error) {
	if value, numeral, err := t.numeral(expression); err != nil {
		return false, false, err
	} else if numeral {
		if value.Cmp(big.NewRat(1, 1)) == 0 {
			return true, true, nil
		}
		if value.Cmp(big.NewRat(-1, 1)) == 0 {
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

// InternalizeAtom is theory_diff_logic<idl_ext>::internalize_atom.
func (t *SMTIntegerDiffTheory) InternalizeAtom(expression ast.ASTHandle, _ bool) (bool, error) {
	atom, ok, err := t.internalizeDiffAtom(expression)
	if err != nil || !ok {
		return false, err
	}
	source, target, literal := atom.source, atom.target, atom.literal
	// internalize_atom: "x - y <= 5 => x - y <= 7" axioms only under
	// m_arith_add_binary_bounds.
	if t.addBinaryBounds {
		if err = t.addBoundAxioms(source, target, &atom.k, literal); err != nil {
			return false, err
		}
	}
	positive, err := t.graph.AddEdge(int32(source), int32(target), &atom.k, literal)
	if err != nil {
		return false, err
	}
	// k.neg(); k -= 1 for an Int lhs, else k -= m_epsilon, which is 1 for
	// idl_ext.
	negativeWeight := new(big.Rat).Neg(&atom.k)
	negativeWeight.Sub(negativeWeight, big.NewRat(1, 1))
	negative, err := t.graph.AddEdge(int32(target), int32(source), negativeWeight, literal.Negated())
	if err != nil {
		return false, err
	}
	index := len(t.atoms)
	t.atoms = append(t.atoms, smtIntegerDiffAtom{variable: atom.variable, positive: positive, negative: negative})
	t.atomByBool[atom.variable] = index
	return true, nil
}

func (t *SMTIntegerDiffTheory) Assign(variable int32, truth bool) error {
	index, found := t.atomByBool[variable]
	if !found {
		return ast.ErrASTDomain
	}
	t.atoms[index].truth = truth
	t.assertions = append(t.assertions, smtIntegerDiffAssertion{atom: index, truth: truth})
	t.statistics.assertions++
	return nil
}

func (t *SMTIntegerDiffTheory) CanPropagate() bool {
	return t != nil && t.assertionHead < len(t.assertions)
}

func (t *SMTIntegerDiffTheory) Propagate() error {
	state, err := t.State()
	if err != nil {
		return err
	}
	for !state.inconsistent && t.CanPropagate() {
		assertion := t.assertions[t.assertionHead]
		t.assertionHead++
		atom := &t.atoms[assertion.atom]
		edge := atom.negative
		if assertion.truth {
			edge = atom.positive
		}
		feasible, enableErr := t.graph.EnableEdge(edge)
		if enableErr != nil {
			return enableErr
		}
		if !feasible {
			literals, explainErr := t.graph.NegativeCycleExplanations(true)
			if explainErr != nil {
				return explainErr
			}
			kept := literals[:0]
			for _, literal := range literals {
				if literal != SMTNullLiteral {
					kept = append(kept, literal)
				}
			}
			if err = state.SetTheoryConflict(t.FamilyID(), kept); err == nil {
				t.statistics.conflicts++
			}
			return err
		}
	}
	return nil
}

func (t *SMTIntegerDiffTheory) PushScope() error {
	if t == nil || t.SMTTheoryBase == nil || t.graph == nil {
		return ast.ErrASTHandle
	}
	if err := t.SMTTheoryBase.PushScope(); err != nil {
		return err
	}
	if err := t.graph.Push(); err != nil {
		_ = t.SMTTheoryBase.PopScopes(1)
		return err
	}
	if err := t.equalityAdapter.PushScope(); err != nil {
		_ = t.graph.Pop(1)
		_ = t.SMTTheoryBase.PopScopes(1)
		return err
	}
	t.scopes = append(t.scopes, smtIntegerDiffScope{len(t.atoms), len(t.assertions), t.assertionHead, t.nonDifference})
	return nil
}

func (t *SMTIntegerDiffTheory) PopScopes(count uint32) error {
	if t == nil || count == 0 || int(count) > len(t.scopes) {
		return ast.ErrASTDomain
	}
	level := len(t.scopes) - int(count)
	scope := t.scopes[level]
	for index := len(t.atoms); index > scope.atoms; index-- {
		delete(t.atomByBool, t.atoms[index-1].variable)
	}
	t.atoms = t.atoms[:scope.atoms]
	t.assertions = t.assertions[:scope.assertions]
	t.assertionHead = scope.head
	t.nonDifference = scope.nonDifference
	t.scopes = t.scopes[:level]
	if err := t.equalityAdapter.PopScopes(count); err != nil {
		return err
	}
	if err := t.graph.Pop(count); err != nil {
		return err
	}
	return t.SMTTheoryBase.PopScopes(count)
}

func (t *SMTIntegerDiffTheory) FinalCheck() (SMTFinalCheckStatus, error) {
	if t.CanPropagate() {
		if err := t.Propagate(); err != nil {
			return SMTFinalCheckGiveUp, err
		}
		return SMTFinalCheckContinue, nil
	}
	if err := t.graph.SetToZero(int32(t.zero)); err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if t.nonDifference {
		return SMTFinalCheckGiveUp, nil
	}
	return SMTFinalCheckDone, nil
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
			_, isNumeral, err := t.numeral(rhs)
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
	if _, numeral, err := t.numeral(a1); err != nil {
		return false, err
	} else if numeral {
		a0, a1 = a1, a0
	}
	if value, numeral, err := t.numeral(a0); err != nil {
		return false, err
	} else if numeral && value.Cmp(big.NewRat(-1, 1)) == 0 {
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
	if value, numeral, err := t.numeral(a2); err != nil || !numeral || value.Cmp(big.NewRat(1, 1)) != 0 {
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

func (t *SMTIntegerDiffTheory) ModelValue(node *SMTEnode, generator *SMTModelGenerator) (SMTModelValueProcedure, error) {
	if t == nil || generator == nil || node == nil || !t.HasVariable(node.TheoryVar(t.FamilyID())) {
		return nil, ast.ErrASTDomain
	}
	state, err := t.State()
	if err != nil {
		return nil, err
	}
	if generator.manager != state.manager {
		return nil, ast.ErrASTDomain
	}
	variable := node.TheoryVar(t.FamilyID())
	value, err := t.graph.Assignment(int32(variable))
	if err != nil {
		return nil, err
	}
	// mk_num_value(num, m_util.is_int(n->get_owner())).
	sort, err := state.manager.GetSort(node.owner)
	if err != nil {
		return nil, err
	}
	integer, err := t.integerSort(sort)
	if err != nil {
		return nil, err
	}
	numeral, err := state.manager.ArithmeticNumeralRat(value, integer)
	if err != nil {
		return nil, err
	}
	return NewSMTExpressionValueProcedure(numeral)
}
