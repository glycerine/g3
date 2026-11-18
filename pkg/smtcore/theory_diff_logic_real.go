package smtcore

// Source: retained real fragment of src/smt/theory_diff_logic.h and
// src/smt/theory_diff_logic_def.h.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/numeral"
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
	delta             numeral.Rational // m_delta
	realStatistics    smtDiffStatistics
	isLIA             bool // m_is_lia
}

func NewSMTRealDiffTheory(family ast.FamilyID) (*SMTRealDiffTheory, error) {
	syntax, err := NewSMTIntegerDiffTheory(family)
	if err != nil {
		return nil, err
	}
	theory := &SMTRealDiffTheory{
		SMTIntegerDiffTheory: syntax,
		realGraph:            NewSMTInfRationalDiffGraph(nil),
		realAtomByBool:       make(map[int32]int),
		delta:                smtQZero(),
		isLIA:                true,
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
	if numerals := state.manager.NumeralService(); numerals != nil {
		// init's mk_var reaches m_graph.init_var.
		t.realGraph.q = numerals.Manager
	}
	return t.initDiffLogic(state)
}

func (t *SMTRealDiffTheory) diffGraphInitVar(v SMTTheoryVar) error {
	return t.realGraph.InitVariable(int32(v))
}

// diffGraphEnableFixedEdges is the inf_int_rational form of the integer
// graph's pair of enabled edges; enable_edge's result is ignored.
func (t *SMTRealDiffTheory) diffGraphEnableFixedEdges(source, target SMTTheoryVar, k *numeral.Rational) error {
	weight := newSMTInfIntRational()
	defer weight.del(t.q)
	if err := t.q.Copy(&weight.rational, k); err != nil {
		return err
	}
	forward, err := t.realGraph.AddEdge(int32(source), int32(target), &weight, SMTNullLiteral)
	if err != nil {
		return err
	}
	if _, err = t.realGraph.EnableEdge(forward); err != nil {
		return err
	}
	if err = t.q.Neg(&weight.rational); err != nil {
		return err
	}
	backward, err := t.realGraph.AddEdge(int32(target), int32(source), &weight, SMTNullLiteral)
	if err != nil {
		return err
	}
	_, err = t.realGraph.EnableEdge(backward)
	return err
}

func (t *SMTRealDiffTheory) addRealBoundAxioms(source, target SMTTheoryVar, weight *smtInfIntRational, literal SMTLiteral) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	equal, lower, upper := SMTNullLiteral, SMTNullLiteral, SMTNullLiteral
	var lowerWeight, upperWeight *smtInfIntRational
	for _, edgeID := range t.realGraph.outEdges[source] {
		edge := &t.realGraph.edges[edgeID]
		if edge.target != int32(target) || edge.explanation == SMTNullLiteral {
			continue
		}
		c, err := smtInfIntCmp(t.q, &edge.weight, weight)
		if err != nil {
			return err
		}
		switch c {
		case 0:
			equal = edge.explanation
		case -1:
			if lower != SMTNullLiteral {
				if c, err = smtInfIntCmp(t.q, &edge.weight, lowerWeight); err != nil {
					return err
				}
			}
			if lower == SMTNullLiteral || c > 0 {
				lower, lowerWeight = edge.explanation, &edge.weight
			}
		case 1:
			if upper != SMTNullLiteral {
				if c, err = smtInfIntCmp(t.q, &edge.weight, upperWeight); err != nil {
					return err
				}
			}
			if upper == SMTNullLiteral || c < 0 {
				upper, upperWeight = edge.explanation, &edge.weight
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
	defer atom.del(t.q)
	if err != nil || !ok {
		return false, err
	}
	source, target, literal := atom.source, atom.target, atom.literal
	weight := newSMTInfIntRational()
	defer weight.del(t.q)
	if err = t.q.Copy(&weight.rational, &atom.k); err != nil {
		return false, err
	}
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
	// weight is k: k.neg(); then k -= 1 or k -= m_epsilon in place.
	if err = smtInfIntNeg(t.q, &weight); err != nil {
		return false, err
	}
	if atom.integer {
		if err = t.q.Sub(&weight.rational, smtQIntPtr(1), &weight.rational); err != nil {
			return false, err
		}
	} else {
		t.isLIA = false
		// inf_int_rational -= epsilon (0, 1): m_second -= 1 (C++ int; see
		// smtInfIntRational).
		weight.infinitesimal--
	}
	negativeWeight := &weight
	negative, err := t.realGraph.AddEdge(int32(target), int32(source), negativeWeight, literal.Negated())
	if err != nil {
		return false, err
	}
	index := len(t.realAtoms)
	t.realAtoms = append(t.realAtoms, smtIntegerDiffAtom{variable: atom.variable, positive: positive, negative: negative})
	t.realAtomByBool[atom.variable] = index
	t.traceDL("arith", "internalize_atom", 264, func() string {
		return t.dlPP(expression) + "\n" + "pos: " + t.realGraph.displayEdgeText(positive) + "neg: " + t.realGraph.displayEdgeText(negative)
	})
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
	// propagate_core: while (consistent && can_propagate()) consistent =
	// propagate_atom(m_asserted_atoms[m_asserted_qhead++]);
	for t.CanPropagate() {
		assertion := t.realAssertions[t.realAssertionHead]
		t.realAssertionHead++
		atom := &t.realAtoms[assertion.atom]
		t.traceDL("arith", "propagate_atom", 548, func() string {
			var out strings.Builder
			if printer, err := ast.NewSimpleASTPrinterContext(state.manager, &out, &out); err == nil {
				_ = displaySMTDiffAtom(&out, state, printer, atom)
			}
			return out.String() + "\n"
		})
		if state.inconsistent {
			return nil
		}
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
	t.traceDL("arith_final", "final_check_eh", 361, func() string {
		var out strings.Builder
		_ = t.Display(&out)
		return out.String()
	})
	if err := t.realGraph.SetToZero(int32(t.zero)); err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if t.nonDifference {
		return SMTFinalCheckGiveUp, nil
	}
	if t.mixed {
		if err := t.computeModelDelta(false); err != nil {
			return SMTFinalCheckGiveUp, err
		}
		value := smtQZero()
		defer smtQDel(t.q, &value)
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
				if err := t.modelNum(&t.realGraph.assignment[v], &value); err != nil {
					return SMTFinalCheckGiveUp, err
				}
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
	if err = t.computeModelDelta(true); err != nil {
		return err
	}
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
	assignment, num := newSMTInfIntRational(), smtQZero()
	defer assignment.del(t.q)
	defer smtQDel(t.q, &num)
	if err := t.realGraph.Assignment(int32(variable), &assignment); err != nil {
		return nil, err
	}
	if err := t.modelNum(&assignment, &num); err != nil {
		return nil, err
	}
	// mk_num_value(num, m_util.is_int(n->get_owner())), for every Ext.
	sort, err := state.manager.GetSort(node.owner)
	if err != nil {
		return nil, err
	}
	integer, err := t.integerSort(sort)
	if err != nil {
		return nil, err
	}
	t.traceDL("arith", "mk_value", 908, func() string {
		text, e := t.q.String(&num)
		t.realGraph.keep(e)
		return t.dlPP(node.owner) + " |-> " + text + "\n"
	})
	expression, err := state.manager.ArithmeticNumeralFromRational(&num, integer)
	if err != nil {
		return nil, err
	}
	return NewSMTExpressionValueProcedure(expression)
}

// modelNum is val.get_rational() + m_delta * val.get_infinitesimal().
func (t *SMTRealDiffTheory) modelNum(val *smtInfIntRational, out *numeral.Rational) error {
	scaled := smtQZero()
	defer smtQDel(t.q, &scaled)
	if err := t.q.SetInt64(&scaled, int64(val.infinitesimal)); err != nil {
		return err
	}
	if err := t.q.Mul(&t.delta, &scaled, &scaled); err != nil {
		return err
	}
	return t.q.Add(&val.rational, &scaled, out)
}

// computeModelDelta is compute_delta. The fractional-Int guard of FinalCheck
// is not a source call, so it passes trace false.
func (t *SMTRealDiffTheory) computeModelDelta(trace bool) error {
	q := t.q
	if err := q.SetInt64(&t.delta, 1); err != nil {
		return err
	}
	limit, numerator, denominator, candidate := smtQZero(), smtQZero(), smtQZero(), smtQZero()
	defer func() {
		for _, r := range []*numeral.Rational{&limit, &numerator, &denominator, &candidate} {
			smtQDel(q, r)
		}
	}()
	text := func(r *numeral.Rational) string {
		s, err := q.String(r)
		t.realGraph.keep(err)
		return s
	}
	for index := range t.realGraph.edges {
		edge := &t.realGraph.edges[index]
		if !edge.enabled {
			continue
		}
		target := &t.realGraph.assignment[edge.target]
		source := &t.realGraph.assignment[edge.source]
		if trace {
			t.traceDL("epsilon", "compute_delta", 881, func() string {
				return fmt.Sprintf("(n_x,k_x): %s, %s, (n_y,k_y): %s, %s, (n_c,k_c): %s, %s\n",
					text(&target.rational), strconv.Itoa(target.infinitesimal), text(&source.rational), strconv.Itoa(source.infinitesimal),
					text(&edge.weight.rational), strconv.Itoa(edge.weight.infinitesimal))
			})
		}
		// n_x < n_y + n_c && k_x > k_y + k_c
		if err := q.Add(&source.rational, &edge.weight.rational, &limit); err != nil {
			return err
		}
		c, err := q.Compare(&target.rational, &limit)
		if err != nil {
			return err
		}
		infinitesimalLimit := source.infinitesimal + edge.weight.infinitesimal
		if c >= 0 || target.infinitesimal <= infinitesimalLimit {
			continue
		}
		// new_delta = (n_y + n_c - n_x) / (k_x - k_y - k_c)
		if err := q.Sub(&limit, &target.rational, &numerator); err != nil {
			return err
		}
		if err := q.SetInt64(&denominator, int64(target.infinitesimal-infinitesimalLimit)); err != nil {
			return err
		}
		if err := q.Div(&numerator, &denominator, &candidate); err != nil {
			return err
		}
		if c, err = q.Compare(&candidate, &t.delta); err != nil {
			return err
		}
		if c < 0 {
			if trace {
				t.traceDL("epsilon", "compute_delta", 886, func() string { return "new delta: " + text(&candidate) + "\n" })
			}
			if err := q.Copy(&t.delta, &candidate); err != nil {
				return err
			}
		}
	}
	return t.realGraph.takeErr(nil)
}
