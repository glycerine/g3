package smtcore

// Source: retained real fragment of src/smt/theory_diff_logic.h and
// src/smt/theory_diff_logic_def.h.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
)

// SMTRealDiffTheory reuses the arithmetic syntax decomposition from the
// integer specialization and owns the concrete infinitesimal graph required
// for strict real complements.
type SMTRealDiffTheory struct {
	*SMTIntegerDiffTheory
	mixed             bool
	realGraph         *SMTInfRationalDiffGraph
	realAtoms         []smtIntegerDiffAtom
	realAtomByBool    map[int32]int
	realAssertions    []smtIntegerDiffAssertion
	realAssertionHead int
	realScopes        []smtIntegerDiffScope
	delta             big.Rat
	realStatistics    smtDiffStatistics
}

func NewSMTRealDiffTheory(family ast.FamilyID) (*SMTRealDiffTheory, error) {
	syntax, err := NewSMTIntegerDiffTheory(family)
	if err != nil {
		return nil, err
	}
	theory := &SMTRealDiffTheory{
		SMTIntegerDiffTheory: syntax,
		realGraph:            NewSMTInfRationalDiffGraph(),
		realAtomByBool:       make(map[int32]int),
	}
	syntax.graphExt = theory
	return theory, nil
}

// The concrete mi_ext graph uses the same infinitesimal rational values, but
// integer atoms use a unit strict-complement step and publish Int numerals.
func NewSMTMixedDiffTheory(family ast.FamilyID) (*SMTRealDiffTheory, error) {
	t, err := NewSMTRealDiffTheory(family)
	if err == nil {
		t.mixed = true
		t.mkNumZeroEnode = true
	}
	return t, err
}

// Initialize is theory_diff_logic<rdl_ext>::init. It uses
// m_util.mk_numeral(rational(0), true), an Int zero, for the real extension
// too.
func (t *SMTRealDiffTheory) Initialize(state *SMTInternalizerState) error {
	if t == nil || t.SMTTheoryBase == nil || t.realGraph == nil {
		return ast.ErrASTHandle
	}
	return t.initDiffLogic(state)
}

func (t *SMTRealDiffTheory) diffGraphInitVar(v SMTTheoryVar) error {
	return t.realGraph.InitVariable(int32(v))
}

// diffGraphEnableFixedEdges is the inf_int_rational form of the integer
// graph's pair of enabled edges; enable_edge's result is ignored.
func (t *SMTRealDiffTheory) diffGraphEnableFixedEdges(source, target SMTTheoryVar, k *big.Rat) error {
	forwardWeight := finiteSMTInfRational(k)
	forward, err := t.realGraph.AddEdge(int32(source), int32(target), &forwardWeight, SMTNullLiteral)
	if err != nil {
		return err
	}
	if _, err = t.realGraph.EnableEdge(forward); err != nil {
		return err
	}
	backwardWeight := finiteSMTInfRational(new(big.Rat).Neg(k))
	backward, err := t.realGraph.AddEdge(int32(target), int32(source), &backwardWeight, SMTNullLiteral)
	if err != nil {
		return err
	}
	_, err = t.realGraph.EnableEdge(backward)
	return err
}

func finiteSMTInfRational(value *big.Rat) smtInfRational {
	return newSMTInfRational(value, nil)
}

func (t *SMTRealDiffTheory) addRealBoundAxioms(source, target SMTTheoryVar, weight *smtInfRational, literal SMTLiteral) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	equal, lower, upper := SMTNullLiteral, SMTNullLiteral, SMTNullLiteral
	var lowerWeight, upperWeight smtInfRational
	for _, edgeID := range t.realGraph.outEdges[source] {
		edge := &t.realGraph.edges[edgeID]
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

func (t *SMTRealDiffTheory) InternalizeTerm(expression ast.ASTHandle) (bool, error) {
	return t.internalizeDiffTerm(expression)
}

// InternalizeAtom is theory_diff_logic<rdl_ext>::internalize_atom.
func (t *SMTRealDiffTheory) InternalizeAtom(expression ast.ASTHandle, _ bool) (bool, error) {
	atom, ok, err := t.internalizeDiffAtom(expression)
	if err != nil || !ok {
		return false, err
	}
	source, target, literal := atom.source, atom.target, atom.literal
	weight := finiteSMTInfRational(&atom.k)
	// internalize_atom: "x - y <= 5 => x - y <= 7" axioms only under
	// m_arith_add_binary_bounds.
	if t.addBinaryBounds {
		if err = t.addRealBoundAxioms(source, target, &weight, literal); err != nil {
			return false, err
		}
	}
	positive, err := t.realGraph.AddEdge(int32(source), int32(target), &weight, literal)
	if err != nil {
		return false, err
	}
	// k.neg(); then k -= 1 for an Int lhs, else k -= m_epsilon, the
	// infinitesimal (0, 1) of rdl_ext.
	var negativeWeight smtInfRational
	negativeWeight.Neg(&weight)
	if atom.integer {
		negativeWeight.rational.Sub(&negativeWeight.rational, big.NewRat(1, 1))
	} else {
		negativeWeight.infinitesimal.Sub(&negativeWeight.infinitesimal, big.NewRat(1, 1))
	}
	negative, err := t.realGraph.AddEdge(int32(target), int32(source), &negativeWeight, literal.Negated())
	if err != nil {
		return false, err
	}
	index := len(t.realAtoms)
	t.realAtoms = append(t.realAtoms, smtIntegerDiffAtom{variable: atom.variable, positive: positive, negative: negative})
	t.realAtomByBool[atom.variable] = index
	return true, nil
}

func (t *SMTRealDiffTheory) Assign(variable int32, truth bool) error {
	index, found := t.realAtomByBool[variable]
	if !found {
		return ast.ErrASTDomain
	}
	t.realAtoms[index].truth = truth
	t.realAssertions = append(t.realAssertions, smtIntegerDiffAssertion{atom: index, truth: truth})
	t.realStatistics.assertions++
	return nil
}

func (t *SMTRealDiffTheory) CanPropagate() bool {
	return t != nil && t.realAssertionHead < len(t.realAssertions)
}

func (t *SMTRealDiffTheory) Propagate() error {
	state, err := t.State()
	if err != nil {
		return err
	}
	for !state.inconsistent && t.CanPropagate() {
		assertion := t.realAssertions[t.realAssertionHead]
		t.realAssertionHead++
		atom := &t.realAtoms[assertion.atom]
		edge := atom.negative
		if assertion.truth {
			edge = atom.positive
		}
		feasible, enableErr := t.realGraph.EnableEdge(edge)
		if enableErr != nil {
			return enableErr
		}
		if !feasible {
			literals, explainErr := t.realGraph.NegativeCycleExplanations()
			if explainErr != nil {
				return explainErr
			}
			kept := literals[:0]
			for _, literal := range literals {
				if literal != SMTNullLiteral {
					kept = append(kept, literal)
				}
			}
			return t.setNegativeCycleConflict(kept)
		}
	}
	return nil
}

func (t *SMTRealDiffTheory) PushScope() error {
	if t == nil || t.SMTTheoryBase == nil || t.realGraph == nil {
		return ast.ErrASTHandle
	}
	if err := t.SMTTheoryBase.PushScope(); err != nil {
		return err
	}
	if err := t.realGraph.Push(); err != nil {
		_ = t.SMTTheoryBase.PopScopes(1)
		return err
	}
	if err := t.equalityAdapter.PushScope(); err != nil {
		_ = t.realGraph.Pop(1)
		_ = t.SMTTheoryBase.PopScopes(1)
		return err
	}
	t.realScopes = append(t.realScopes, smtIntegerDiffScope{len(t.realAtoms), len(t.realAssertions), t.realAssertionHead, t.nonDifference})
	return nil
}

func (t *SMTRealDiffTheory) PopScopes(count uint32) error {
	if t == nil || count == 0 || int(count) > len(t.realScopes) {
		return ast.ErrASTDomain
	}
	level := len(t.realScopes) - int(count)
	scope := t.realScopes[level]
	for index := len(t.realAtoms); index > scope.atoms; index-- {
		delete(t.realAtomByBool, t.realAtoms[index-1].variable)
	}
	t.realAtoms = t.realAtoms[:scope.atoms]
	t.realAssertions = t.realAssertions[:scope.assertions]
	t.realAssertionHead = scope.head
	t.nonDifference = scope.nonDifference
	t.realScopes = t.realScopes[:level]
	if err := t.equalityAdapter.PopScopes(count); err != nil {
		return err
	}
	oldEdges := t.realGraph.NumEdges()
	if err := t.realGraph.Pop(count); err != nil {
		return err
	}
	if oldEdges != t.realGraph.NumEdges() {
		t.optimization.resetSimplex()
	}
	return t.SMTTheoryBase.PopScopes(count)
}

func (t *SMTRealDiffTheory) FinalCheck() (SMTFinalCheckStatus, error) {
	if t.CanPropagate() {
		if err := t.Propagate(); err != nil {
			return SMTFinalCheckGiveUp, err
		}
		return SMTFinalCheckContinue, nil
	}
	if err := t.realGraph.SetToZero(int32(t.zero)); err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if t.nonDifference {
		return SMTFinalCheckGiveUp, nil
	}
	if t.mixed {
		t.computeModelDelta()
		for v, node := range t.variables {
			sort, err := t.state.manager.GetSort(node.owner)
			if err != nil {
				return SMTFinalCheckGiveUp, err
			}
			integer, err := t.integerSort(sort)
			if err != nil {
				return SMTFinalCheckGiveUp, err
			}
			if integer {
				var scaled, value big.Rat
				scaled.Mul(&t.delta, &t.realGraph.assignment[v].infinitesimal)
				value.Add(&t.realGraph.assignment[v].rational, &scaled)
				// The source rejects a fractional arithmetic-factory Int value.
				// Detect that model failure before publication.
				if !value.IsInt() {
					return SMTFinalCheckGiveUp, nil
				}
			}
		}
	}
	return SMTFinalCheckDone, nil
}

func (t *SMTRealDiffTheory) InitializeSearch() error {
	return t.equalityAdapter.InitializeSearch()
}

func (t *SMTRealDiffTheory) Restart() error {
	return t.equalityAdapter.Restart()
}

// InternalizeEquality is theory_diff_logic<rdl_ext>::internalize_eq_eh,
// the same template as the integer theory's.
func (t *SMTRealDiffTheory) InternalizeEquality(expression ast.ASTHandle, variable int32) error {
	return t.SMTIntegerDiffTheory.InternalizeEquality(expression, variable)
}

func (t *SMTRealDiffTheory) NewEquality(left, right SMTTheoryVar) error {
	return t.equalityAdapter.NewEquality(left, right)
}
func (t *SMTRealDiffTheory) NewDisequality(left, right SMTTheoryVar) error {
	return t.equalityAdapter.NewDisequality(left, right)
}

func (t *SMTRealDiffTheory) Graph() *SMTInfRationalDiffGraph {
	if t == nil {
		return nil
	}
	return t.realGraph
}

// Name is theory_diff_logic<Ext>::get_name, the same for every Ext.
func (t *SMTRealDiffTheory) Name() string { return "difference-logic" }

func (t *SMTRealDiffTheory) InitializeModel(generator *SMTModelGenerator) error {
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
	t.computeModelDelta()
	factory, err := protomodel.NewArithmeticFactory(state.manager)
	if err != nil {
		return err
	}
	return generator.RegisterFactory(factory)
}

func (t *SMTRealDiffTheory) ModelValue(node *SMTEnode, generator *SMTModelGenerator) (SMTModelValueProcedure, error) {
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
	assignment, err := t.realGraph.Assignment(int32(variable))
	if err != nil {
		return nil, err
	}
	var scaled, value big.Rat
	scaled.Mul(&t.delta, &assignment.infinitesimal)
	value.Add(&assignment.rational, &scaled)
	// mk_num_value(num, m_util.is_int(n->get_owner())), for every Ext.
	sort, err := state.manager.GetSort(node.owner)
	if err != nil {
		return nil, err
	}
	integer, err := t.integerSort(sort)
	if err != nil {
		return nil, err
	}
	numeral, err := state.manager.ArithmeticNumeralRat(&value, integer)
	if err != nil {
		return nil, err
	}
	return NewSMTExpressionValueProcedure(numeral)
}

func (t *SMTRealDiffTheory) computeModelDelta() {
	t.delta.SetInt64(1)
	for index := range t.realGraph.edges {
		edge := &t.realGraph.edges[index]
		if !edge.enabled {
			continue
		}
		target := &t.realGraph.assignment[edge.target]
		source := &t.realGraph.assignment[edge.source]
		var rationalLimit, infinitesimalLimit big.Rat
		rationalLimit.Add(&source.rational, &edge.weight.rational)
		infinitesimalLimit.Add(&source.infinitesimal, &edge.weight.infinitesimal)
		if target.rational.Cmp(&rationalLimit) >= 0 || target.infinitesimal.Cmp(&infinitesimalLimit) <= 0 {
			continue
		}
		var numerator, denominator, candidate big.Rat
		numerator.Sub(&rationalLimit, &target.rational)
		denominator.Sub(&target.infinitesimal, &infinitesimalLimit)
		candidate.Quo(&numerator, &denominator)
		if candidate.Cmp(&t.delta) < 0 {
			t.delta.Set(&candidate)
		}
	}
}
