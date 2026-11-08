package rewriter

// Source: src/ast/rewriter/seq_rewriter.cpp:26-193,203-342,1147-1326.

import (
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// SequenceExpressionSolver is the retained expr_solver hook used by symbolic
// regex intersection and complement construction.
type SequenceExpressionSolver interface {
	CheckSatisfiable(ast.ASTHandle) (lbool.LBool, error)
}

func (r *SequenceRewriter) SetSolver(solver SequenceExpressionSolver) error {
	if err := r.open(); err != nil {
		return err
	}
	r.solver = solver
	return nil
}

type sequenceAutomatonLabelKind uint8

const (
	sequenceLabelCharacter sequenceAutomatonLabelKind = iota
	sequenceLabelRange
	sequenceLabelAny
	sequenceLabelIntersection
	sequenceLabelNegation
	sequenceLabelNone
)

// sequenceAutomatonLabel is sym_expr: t_char (character), t_range (lower and
// upper bounds) or t_pred, a formula over var(0) of sort. The predicate kinds
// are Any = mk_pred(true), None = mk_pred(false), and the results of mk_and
// and mk_not.
//
// Like sym_expr, a label is reference counted (m_ref, starting at 0) by the
// automaton moves and ref_t locals that hold it, and owns its expressions
// through m_t and m_s: (c, c) for a character, (lo, hi) for a range and
// (f, f) for a predicate. When the count drops to 0 the label is deallocated,
// which destroys m_s and then m_t.
//
// concrete is the label's byte set when it is known from numerals alone; only
// the Go-only enumeration helpers and the solver-less fallback read it. It is
// computed when the label is made so that no label keeps its operands.
type sequenceAutomatonLabel struct {
	kind          sequenceAutomatonLabelKind
	character     ast.ASTHandle
	lowerBound    ast.ASTHandle
	upperBound    ast.ASTHandle
	predicate     ast.ASTHandle
	sort          ast.ASTHandle
	manager       *ast.ASTManager
	t, s          ast.ASTRef
	refs          int
	freed         bool
	concrete      [256]bool
	concreteKnown bool
}

func (l *sequenceAutomatonLabel) isPredicate() bool {
	return l.kind != sequenceLabelCharacter && l.kind != sequenceLabelRange
}

// newSequenceLabel is the sym_expr constructor: m_t(t), m_s(s), m_ref(0).
func newSequenceLabel(m *ast.ASTManager, kind sequenceAutomatonLabelKind, t, s ast.ASTHandle) (*sequenceAutomatonLabel, error) {
	tRef, err := m.NewRef(t)
	if err != nil {
		return nil, err
	}
	sRef, err := m.NewRef(s)
	if err != nil {
		return nil, errors.Join(err, m.DelRef(&tRef))
	}
	return &sequenceAutomatonLabel{kind: kind, manager: m, t: tRef, s: sRef}, nil
}

// retain is sym_expr::inc_ref.
func (l *sequenceAutomatonLabel) retain() {
	if l != nil {
		l.refs++
	}
}

// release is sym_expr::dec_ref: at 0 the label is deallocated.
func (l *sequenceAutomatonLabel) release() error {
	if l == nil {
		return nil
	}
	if l.refs <= 0 || l.freed {
		return ast.ErrASTHandle
	}
	l.refs--
	if l.refs > 0 {
		return nil
	}
	return l.free()
}

// free destroys m_s and then m_t.
func (l *sequenceAutomatonLabel) free() error {
	if l.freed {
		return nil
	}
	l.freed = true
	err := l.manager.DelRef(&l.s)
	return errors.Join(err, l.manager.DelRef(&l.t))
}

// releaseUnowned deallocates a label that nothing ever referenced, as when a
// ref_t takes and drops a fresh sym_expr.
func (l *sequenceAutomatonLabel) releaseUnowned() error {
	if l == nil || l.refs != 0 {
		return nil
	}
	return l.free()
}

// characterLabel is sym_expr::mk_char(m, ch).
func (b *sequenceAutomatonBuilder) characterLabel(ch ast.ASTHandle) (*sequenceAutomatonLabel, error) {
	m := b.rewriter.manager
	l, err := newSequenceLabel(m, sequenceLabelCharacter, ch, ch)
	if err != nil {
		return nil, err
	}
	l.character = ch
	value, numeral, err := m.SequenceUnitByteElementValue(ch)
	if err != nil {
		return nil, errors.Join(err, l.free())
	}
	if numeral {
		l.concrete[value] = true
		l.concreteKnown = true
	}
	return l, nil
}

// rangeLabel is sym_expr::mk_range(lo, hi).
func (b *sequenceAutomatonBuilder) rangeLabel(lo, hi ast.ASTHandle) (*sequenceAutomatonLabel, error) {
	m := b.rewriter.manager
	l, err := newSequenceLabel(m, sequenceLabelRange, lo, hi)
	if err != nil {
		return nil, err
	}
	l.lowerBound, l.upperBound = lo, hi
	loValue, loNumeral, err := m.SequenceUnitByteElementValue(lo)
	if err != nil {
		return nil, errors.Join(err, l.free())
	}
	hiValue, hiNumeral, err := m.SequenceUnitByteElementValue(hi)
	if err != nil {
		return nil, errors.Join(err, l.free())
	}
	if loNumeral && hiNumeral {
		for value := int(loValue); value <= int(hiValue); value++ {
			l.concrete[value] = true
		}
		l.concreteKnown = true
	}
	return l, nil
}

type sequenceAutomatonMove struct {
	source, destination int
	label               *sequenceAutomatonLabel
}

// sequenceAutomaton is automaton<sym_expr, sym_expr_manager> from
// math/automata/automaton.h as re2automaton builds it. moves is m_delta;
// finals is m_final_set and finalStates is m_final_states, in insertion order.
type sequenceAutomaton struct {
	initial     int
	finals      map[int]bool
	finalStates []int
	moves       [][]sequenceAutomatonMove
}

func newSequenceAutomaton(states int) *sequenceAutomaton {
	if states < 1 {
		states = 1
	}
	return &sequenceAutomaton{finals: make(map[int]bool), moves: make([][]sequenceAutomatonMove, states)}
}

// sequenceAutomatonOf is automaton(m, init, final, mvs). The source sizes
// m_delta from the moves alone; a final or initial state outside the moves is
// also given a row here so it can be indexed.
func sequenceAutomatonOf(initial int, final []int, moves []sequenceAutomatonMove) *sequenceAutomaton {
	n := 1
	for _, mv := range moves {
		if mv.source+1 > n {
			n = mv.source + 1
		}
		if mv.destination+1 > n {
			n = mv.destination + 1
		}
	}
	for _, f := range final {
		if f+1 > n {
			n = f + 1
		}
	}
	if initial+1 > n {
		n = initial + 1
	}
	a := newSequenceAutomaton(n)
	a.initial = initial
	for _, f := range final {
		a.addFinal(f)
	}
	for _, mv := range moves {
		a.add(mv)
	}
	return a
}

// addFinal is add_to_final_states.
func (a *sequenceAutomaton) addFinal(state int) {
	if !a.finals[state] {
		a.finals[state] = true
		a.finalStates = append(a.finalStates, state)
	}
}

// add is automaton::add: it skips a move equal to the last move from the
// same source (is_duplicate_cheap). The kept move holds its label (its
// m_delta and m_delta_inv copies are counted as one reference here: nothing
// removes one copy without the other in these automata).
func (a *sequenceAutomaton) add(mv sequenceAutomatonMove) {
	out := a.moves[mv.source]
	if n := len(out); n > 0 && out[n-1].destination == mv.destination && out[n-1].label == mv.label {
		return
	}
	mv.label.retain()
	a.moves[mv.source] = append(out, mv)
}

// release is the automaton destructor (scoped_ptr reset/dealloc): its moves
// release their labels in m_delta order, after the m_delta_inv copies, which
// can never free a label.
func (a *sequenceAutomaton) release() (err error) {
	if a == nil {
		return nil
	}
	for _, out := range a.moves {
		for _, mv := range out {
			err = errors.Join(err, mv.label.release())
		}
	}
	a.moves = nil
	return err
}

// releaseMoves is the destructor of a moves vector holding its labels.
func releaseSequenceMoves(moves []sequenceAutomatonMove) (err error) {
	for _, mv := range moves {
		err = errors.Join(err, mv.label.release())
	}
	return err
}

// retainSequenceMoves is moves::push_back of copies.
func retainSequenceMoves(moves []sequenceAutomatonMove) {
	for _, mv := range moves {
		mv.label.retain()
	}
}

// sequenceEmptyAutomaton is automaton(m): one state, no final state.
func sequenceEmptyAutomaton() *sequenceAutomaton { return newSequenceAutomaton(1) }

// sequenceEpsilonAutomaton is mk_epsilon.
func sequenceEpsilonAutomaton() *sequenceAutomaton {
	return sequenceAutomatonOf(0, []int{0}, nil)
}

// sequenceLabelAutomaton is automaton(m, t).
func sequenceLabelAutomaton(label *sequenceAutomatonLabel) *sequenceAutomaton {
	a := newSequenceAutomaton(2)
	a.addFinal(1)
	a.add(sequenceAutomatonMove{source: 0, destination: 1, label: label})
	return a
}

// sequenceLoopAutomaton is mk_loop.
func sequenceLoopAutomaton(label *sequenceAutomatonLabel) *sequenceAutomaton {
	return sequenceAutomatonOf(0, []int{0}, []sequenceAutomatonMove{{source: 0, destination: 0, label: label}})
}

func (a *sequenceAutomaton) numStates() int { return len(a.moves) }

func (a *sequenceAutomaton) isEmpty() bool { return len(a.finalStates) == 0 }

// initialStateIsSource is m_delta_inv[m_init].empty().
func (a *sequenceAutomaton) initialStateIsSource() bool {
	for _, out := range a.moves {
		for _, mv := range out {
			if mv.destination == a.initial {
				return false
			}
		}
	}
	return true
}

// appendMoves is append_moves.
func (a *sequenceAutomaton) appendMoves(offset int, moves []sequenceAutomatonMove) []sequenceAutomatonMove {
	for _, out := range a.moves {
		for _, mv := range out {
			moves = append(moves, sequenceAutomatonMove{source: mv.source + offset, destination: mv.destination + offset, label: mv.label})
		}
	}
	return moves
}

// appendFinal is append_final.
func (a *sequenceAutomaton) appendFinal(offset int, final []int) []int {
	for _, state := range a.finalStates {
		final = append(final, state+offset)
	}
	return final
}

func (a *sequenceAutomaton) clone() *sequenceAutomaton {
	return sequenceAutomatonOf(a.initial, a.appendFinal(0, nil), a.appendMoves(0, nil))
}

// sequenceUnionAutomaton is mk_union.
func sequenceUnionAutomaton(left, right *sequenceAutomaton) *sequenceAutomaton {
	if left.isEmpty() {
		return right.clone()
	}
	if right.isEmpty() {
		return left.clone()
	}
	offset1 := 1
	offset2 := left.numStates() + 1
	moves := []sequenceAutomatonMove{{source: 0, destination: left.initial + offset1}, {source: 0, destination: right.initial + offset2}}
	moves = left.appendMoves(offset1, moves)
	moves = right.appendMoves(offset2, moves)
	final := left.appendFinal(offset1, nil)
	final = right.appendFinal(offset2, final)
	return sequenceAutomatonOf(0, final, moves)
}

// sequenceOptionalAutomaton is mk_opt.
func sequenceOptionalAutomaton(a *sequenceAutomaton) *sequenceAutomaton {
	var moves []sequenceAutomatonMove
	offset := 0
	initial := a.initial
	if !a.initialStateIsSource() {
		offset = 1
		initial = 0
		moves = append(moves, sequenceAutomatonMove{source: 0, destination: a.initial + offset})
	}
	if a.isEmpty() {
		return a.clone()
	}
	moves = append(moves, sequenceAutomatonMove{source: initial, destination: a.finalStates[0] + offset})
	moves = a.appendMoves(offset, moves)
	return sequenceAutomatonOf(initial, a.appendFinal(offset, nil), moves)
}

// sequenceConcatAutomaton is mk_concat. Its is_epsilon shortcuts test
// m_delta.empty(), which never holds, so they are not ported.
func sequenceConcatAutomaton(left, right *sequenceAutomaton) *sequenceAutomaton {
	if left.isEmpty() {
		return left.clone()
	}
	if right.isEmpty() {
		return right.clone()
	}
	offset1 := 1
	offset2 := left.numStates() + offset1
	moves := []sequenceAutomatonMove{{source: 0, destination: left.initial + offset1}}
	moves = left.appendMoves(offset1, moves)
	for _, state := range left.finalStates {
		moves = append(moves, sequenceAutomatonMove{source: state + offset1, destination: right.initial + offset2})
	}
	moves = right.appendMoves(offset2, moves)
	return sequenceAutomatonOf(0, right.appendFinal(offset2, nil), moves)
}

// addFinalToInitMoves is add_final_to_init_moves. The source's duplicate
// check loop never runs (it starts with found = false), so every final state
// other than the initial state gets an epsilon move to it.
func (a *sequenceAutomaton) addFinalToInitMoves() {
	for i := 0; i < len(a.finalStates); i++ {
		state := a.finalStates[i]
		if state != a.initial {
			a.add(sequenceAutomatonMove{source: state, destination: a.initial})
		}
	}
}

// addInitToFinalStates is add_init_to_final_states.
func (a *sequenceAutomaton) addInitToFinalStates() { a.addFinal(a.initial) }

func (a *sequenceAutomaton) epsilonClosure(state int) []int {
	seen := map[int]bool{state: true}
	stack := []int{state}
	result := make([]int, 0, 4)
	for len(stack) > 0 {
		state = stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		result = append(result, state)
		for _, move := range a.moves[state] {
			if move.label == nil && !seen[move.destination] {
				seen[move.destination] = true
				stack = append(stack, move.destination)
			}
		}
	}
	return result
}

func (a *sequenceAutomaton) closureIsFinal(state int) bool {
	for _, candidate := range a.epsilonClosure(state) {
		if a.finals[candidate] {
			return true
		}
	}
	return false
}

func (a *sequenceAutomaton) movesFrom(state int, closeDestination bool) []sequenceAutomatonMove {
	result := make([]sequenceAutomatonMove, 0, 4)
	for _, source := range a.epsilonClosure(state) {
		for _, move := range a.moves[source] {
			if move.label == nil {
				continue
			}
			if !closeDestination {
				result = append(result, move)
				continue
			}
			for _, destination := range a.epsilonClosure(move.destination) {
				result = append(result, sequenceAutomatonMove{source: source, destination: destination, label: move.label})
			}
		}
	}
	return result
}

func (a *sequenceAutomaton) statesReachingFinal() []bool {
	result := make([]bool, len(a.moves))
	reverse := make([][]int, len(a.moves))
	work := make([]int, 0, len(a.finals))
	for source, moves := range a.moves {
		for _, move := range moves {
			reverse[move.destination] = append(reverse[move.destination], source)
		}
	}
	for state := range a.finals {
		result[state] = true
		work = append(work, state)
	}
	for len(work) != 0 {
		state := work[len(work)-1]
		work = work[:len(work)-1]
		for _, predecessor := range reverse[state] {
			if result[predecessor] {
				continue
			}
			result[predecessor] = true
			work = append(work, predecessor)
		}
	}
	return result
}

type sequenceAutomatonBuilder struct {
	rewriter           *SequenceRewriter
	refs               []ast.ASTRef
	allowUnknownLabels bool
}

func (b *sequenceAutomatonBuilder) close() (err error) {
	for i := len(b.refs) - 1; i >= 0; i-- {
		err = errors.Join(err, b.rewriter.manager.DelRef(&b.refs[i]))
	}
	return err
}

func (b *sequenceAutomatonBuilder) hold(h ast.ASTHandle) error {
	ref, err := b.rewriter.manager.NewRef(h)
	if err != nil {
		return err
	}
	b.refs = append(b.refs, ref)
	return nil
}

// sequence is re2automaton::seq2aut. Its scoped_ptr<eautomaton> a, b are
// destroyed when it returns, b first.
func (b *sequenceAutomatonBuilder) sequence(expression ast.ASTHandle) (*sequenceAutomaton, error) {
	m := b.rewriter.manager
	if left, right, matched, err := m.SequenceConcatParts(expression); err != nil {
		return nil, err
	} else if matched {
		a, err := b.sequence(left)
		if err != nil || a == nil {
			return nil, errors.Join(err, a.release())
		}
		c, err := b.sequence(right)
		if err != nil || c == nil {
			return nil, errors.Join(err, c.release(), a.release())
		}
		result := sequenceConcatAutomaton(a, c)
		return result, errors.Join(c.release(), a.release())
	}
	if element, unit, err := m.SequenceUnitArgument(expression); err != nil {
		return nil, err
	} else if unit {
		label, err := b.characterLabel(element)
		if err != nil {
			return nil, err
		}
		return sequenceLabelAutomaton(label), nil
	}
	if empty, err := m.IsSequenceEmpty(expression); err != nil {
		return nil, err
	} else if empty {
		return sequenceEpsilonAutomaton(), nil
	}
	text, stringValue, err := m.SequenceStringValue(expression)
	if err != nil || !stringValue {
		return nil, err
	}
	moves := make([]sequenceAutomatonMove, 0, len(text))
	for k := 0; k < len(text); k++ {
		// sym_expr::mk_char(m, u.str.mk_char(s, k))
		character, err := m.BitVectorNumeralUnsigned(uint32(text[k]), 8)
		if err != nil {
			return nil, err
		}
		label, err := b.characterLabel(character)
		if err != nil {
			return nil, err
		}
		moves = append(moves, sequenceAutomatonMove{source: k, destination: k + 1, label: label})
	}
	return sequenceAutomatonOf(0, []int{len(text)}, moves), nil
}

// predicateLabel is sym_expr::mk_pred(formula, sort); concrete and known are
// the label's byte set when it follows from its operands.
func (b *sequenceAutomatonBuilder) predicateLabel(kind sequenceAutomatonLabelKind, formula, sort ast.ASTHandle, concrete [256]bool, known bool) (*sequenceAutomatonLabel, error) {
	l, err := newSequenceLabel(b.rewriter.manager, kind, formula, formula)
	if err != nil {
		return nil, err
	}
	l.predicate, l.sort = formula, sort
	l.concrete, l.concreteKnown = concrete, known
	return l, nil
}

// trueLabel and falseLabel are mk_pred(m.mk_true()/m.mk_false(), sort).
func (b *sequenceAutomatonBuilder) trueLabel(sort ast.ASTHandle) (*sequenceAutomatonLabel, error) {
	t, err := b.rewriter.manager.MkTrue()
	if err != nil {
		return nil, err
	}
	var all [256]bool
	for i := range all {
		all[i] = true
	}
	return b.predicateLabel(sequenceLabelAny, t, sort, all, true)
}

func (b *sequenceAutomatonBuilder) falseLabel(sort ast.ASTHandle) (*sequenceAutomatonLabel, error) {
	f, err := b.rewriter.manager.MkFalse()
	if err != nil {
		return nil, err
	}
	return b.predicateLabel(sequenceLabelNone, f, sort, [256]bool{}, true)
}

// regexCharacterLabelSort is the char_s of re2aut's full_seq/full_char cases.
func (b *sequenceAutomatonBuilder) regexCharacterSort(expression ast.ASTHandle) (ast.ASTHandle, error) {
	m := b.rewriter.manager
	sort, err := m.GetSort(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	seq, ok, err := m.RegexSequenceSort(sort)
	if err != nil || !ok {
		return ast.ASTHandle{}, errors.Join(err, ast.ErrASTDomain)
	}
	element, ok, err := m.SequenceSortElement(seq)
	if err != nil || !ok {
		return ast.ASTHandle{}, errors.Join(err, ast.ErrASTDomain)
	}
	return element, nil
}

// automatonLocals are expr_ref/var_ref locals, released in reverse
// declaration order when the source function returns.
type automatonLocals struct {
	manager *ast.ASTManager
	refs    []ast.ASTRef
}

func (l *automatonLocals) own(ref ast.ASTRef) ast.ASTRef {
	l.refs = append(l.refs, ref)
	return ref
}

func (l *automatonLocals) hold(h ast.ASTHandle) error {
	ref, err := l.manager.NewRef(h)
	if err != nil {
		return err
	}
	l.refs = append(l.refs, ref)
	return nil
}

func (l *automatonLocals) release() (err error) {
	for i := len(l.refs) - 1; i >= 0; i-- {
		err = errors.Join(err, l.manager.DelRef(&l.refs[i]))
	}
	l.refs = nil
	return err
}

// intersectLabels is sym_expr_boolean_algebra::mk_and.
func (b *sequenceAutomatonBuilder) intersectLabels(x, y *sequenceAutomatonLabel) (result *sequenceAutomatonLabel, resultErr error) {
	m := b.rewriter.manager
	if x.kind == sequenceLabelCharacter && y.kind == sequenceLabelCharacter {
		if x.character == y.character {
			return x, nil
		}
		distinct, err := m.AreDistinct(x.character, y.character)
		if err != nil {
			return nil, err
		}
		if distinct {
			sort, err := b.labelSort(x)
			if err != nil {
				return nil, err
			}
			return b.falseLabel(sort)
		}
	}
	xs, err := b.labelSort(x)
	if err != nil {
		return nil, err
	}
	s := xs
	if boolean, err := m.MkBoolSort(); err != nil {
		return nil, err
	} else if s == boolean {
		if s, err = b.labelSort(y); err != nil {
			return nil, err
		}
	}
	locals := &automatonLocals{manager: m}
	defer func() { resultErr = errors.Join(resultErr, locals.release()) }()
	v, err := m.MkVar(0, s)
	if err != nil {
		return nil, err
	}
	if err = locals.hold(v); err != nil { // var_ref v
		return nil, err
	}
	r1, err := b.acceptOwned(x, v)
	if err != nil {
		return nil, err
	}
	f1, err := m.RefGet(locals.own(r1))
	if err != nil {
		return nil, err
	}
	r2, err := b.acceptOwned(y, v)
	if err != nil {
		return nil, err
	}
	f2, err := m.RefGet(locals.own(r2))
	if err != nil {
		return nil, err
	}
	if m.IsTrue(f1) {
		return y, nil
	}
	if m.IsTrue(f2) || f1 == f2 {
		return x, nil
	}
	out, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	locals.own(out) // expr_ref fml
	if err = SimplifyBooleanGateDefault(m, ast.ASTOpAnd, []ast.ASTHandle{f1, f2}, out); err != nil {
		return nil, err
	}
	f, err := m.RefGet(out)
	if err != nil {
		return nil, err
	}
	var concrete [256]bool
	known := x.concreteKnown && y.concreteKnown
	if known {
		for i := range concrete {
			concrete[i] = x.concrete[i] && y.concrete[i]
		}
	}
	return b.predicateLabel(sequenceLabelIntersection, f, xs, concrete, known)
}

// negateLabel is sym_expr_boolean_algebra::mk_not: mk_pred(m.mk_not(...)).
func (b *sequenceAutomatonBuilder) negateLabel(x *sequenceAutomatonLabel) (result *sequenceAutomatonLabel, resultErr error) {
	m := b.rewriter.manager
	sort, err := b.labelSort(x)
	if err != nil {
		return nil, err
	}
	locals := &automatonLocals{manager: m}
	defer func() { resultErr = errors.Join(resultErr, locals.release()) }()
	v, err := m.MkVar(0, sort)
	if err != nil {
		return nil, err
	}
	if err = locals.hold(v); err != nil { // var_ref v
		return nil, err
	}
	// expr_ref fml(m.mk_not(x->accept(v))): the accept temporary dies after
	// fml takes its reference.
	accepted, err := b.acceptOwned(x, v)
	if err != nil {
		return nil, err
	}
	body, err := m.RefGet(accepted)
	if err != nil {
		return nil, errors.Join(err, m.DelRef(&accepted))
	}
	f, err := m.MkNot(body)
	if err == nil {
		err = locals.hold(f)
	}
	if err = errors.Join(err, m.DelRef(&accepted)); err != nil {
		return nil, err
	}
	var concrete [256]bool
	if x.concreteKnown {
		for i := range concrete {
			concrete[i] = !x.concrete[i]
		}
	}
	return b.predicateLabel(sequenceLabelNegation, f, sort, concrete, x.concreteKnown)
}

// labelSort is sym_expr::get_sort (m_sort; get_sort(hi) for ranges).
func (b *sequenceAutomatonBuilder) labelSort(label *sequenceAutomatonLabel) (ast.ASTHandle, error) {
	switch label.kind {
	case sequenceLabelCharacter:
		return b.rewriter.manager.GetSort(label.character)
	case sequenceLabelRange:
		return b.rewriter.manager.GetSort(label.upperBound)
	default:
		if label.sort.IsNull() {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		return label.sort, nil
	}
}

func (b *sequenceAutomatonBuilder) concreteLabelBytes(label *sequenceAutomatonLabel) (values [256]bool, concrete bool, err error) {
	if label == nil || label.freed {
		return values, false, ast.ErrASTHandle
	}
	return label.concrete, label.concreteKnown, nil
}

// labelSatisfiable is sym_expr_boolean_algebra::is_sat: characters are
// satisfiable; every other label is accepted on a fresh constant "x" and
// decided by the expression solver unless that formula is true or false.
func (b *sequenceAutomatonBuilder) labelSatisfiable(label *sequenceAutomatonLabel) (satisfiable, defined bool, err error) {
	if label.kind == sequenceLabelCharacter {
		return true, true, nil
	}
	sort, err := b.labelSort(label)
	if err != nil {
		return false, false, err
	}
	locals := &automatonLocals{manager: b.rewriter.manager}
	defer func() { err = errors.Join(err, locals.release()) }()
	variable, err := b.rewriter.manager.MkFreshConst("x", sort)
	if err != nil {
		return false, false, err
	}
	if err = locals.hold(variable); err != nil { // expr_ref v
		return false, false, err
	}
	accepted, err := b.acceptOwned(label, variable)
	if err != nil {
		return false, false, err
	}
	formula, err := b.rewriter.manager.RefGet(locals.own(accepted)) // expr_ref fml
	if err != nil {
		return false, false, err
	}
	if b.rewriter.manager.IsTrue(formula) {
		return true, true, nil
	}
	if b.rewriter.manager.IsFalse(formula) {
		return false, true, nil
	}
	if b.rewriter.solver == nil {
		if values, concrete, err := b.concreteLabelBytes(label); err != nil {
			return false, false, err
		} else if concrete {
			for _, present := range values {
				if present {
					return true, true, nil
				}
			}
			return false, true, nil
		}
		if b.allowUnknownLabels {
			return true, true, nil
		}
		return false, false, nil
	}
	answer, err := b.rewriter.solver.CheckSatisfiable(formula)
	if err != nil {
		return false, false, err
	}
	if answer == lbool.Undef {
		return false, false, nil
	}
	return answer == lbool.True, true, nil
}

// product is symbolic_automata::mk_product. Each pair's ref_t ab holds the
// conjunction until the end of its iteration, so an unsatisfiable one is
// freed at once; mvs holds the kept moves until the function returns, after
// the result has taken the back-reachable ones.
func (b *sequenceAutomatonBuilder) product(left, right *sequenceAutomaton) (result *sequenceAutomaton, resultErr error) {
	type pair struct{ left, right int }
	initial := pair{left.initial, right.initial}
	ids := map[pair]int{initial: 0}
	todo := []pair{initial}
	var mvs []sequenceAutomatonMove
	defer func() { resultErr = errors.Join(resultErr, releaseSequenceMoves(mvs)) }()
	var final []int
	// Deliberate deviation: the source tests is_final_state, which misses a
	// final state reached by epsilon moves (("" | a) & ("" | b) would be
	// empty); finality here includes the epsilon closure.
	if left.closureIsFinal(left.initial) && right.closureIsFinal(right.initial) {
		final = append(final, 0)
	}
	n := 1
	for len(todo) > 0 {
		current := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		source := ids[current]
		mvsA := left.movesFrom(current.left, true)
		mvsB := right.movesFrom(current.right, true)
		for _, leftMove := range mvsA {
			for _, rightMove := range mvsB {
				ab, err := b.intersectLabels(leftMove.label, rightMove.label)
				if err != nil {
					return nil, err
				}
				ab.retain() // ref_t ab
				satisfiable, defined, err := b.labelSatisfiable(ab)
				if err != nil || !defined {
					return nil, errors.Join(err, ab.release())
				}
				if !satisfiable {
					if err = ab.release(); err != nil {
						return nil, err
					}
					continue
				}
				targetPair := pair{leftMove.destination, rightMove.destination}
				target, present := ids[targetPair]
				if !present {
					target = n
					n++
					ids[targetPair] = target
					todo = append(todo, targetPair)
					if left.closureIsFinal(targetPair.left) && right.closureIsFinal(targetPair.right) {
						final = append(final, target)
					}
				}
				ab.retain() // mvs.push_back(move_t(m, src, tgt, ab))
				mvs = append(mvs, sequenceAutomatonMove{source: source, destination: target, label: ab})
				if err = ab.release(); err != nil {
					return nil, err
				}
			}
		}
	}
	if len(final) == 0 {
		return sequenceEmptyAutomaton(), nil
	}
	inverse := make([][]int, n)
	for _, mv := range mvs {
		inverse[mv.destination] = append(inverse[mv.destination], mv.source)
	}
	backReachable := make([]bool, n)
	for _, state := range final {
		backReachable[state] = true
	}
	stack := append([]int(nil), final...)
	for len(stack) > 0 {
		state := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, predecessor := range inverse[state] {
			if !backReachable[predecessor] {
				backReachable[predecessor] = true
				stack = append(stack, predecessor)
			}
		}
	}
	var kept []sequenceAutomatonMove
	for _, mv := range mvs {
		if backReachable[mv.destination] {
			kept = append(kept, mv)
		}
	}
	if len(kept) == 0 {
		if left.closureIsFinal(left.initial) && right.closureIsFinal(right.initial) {
			return sequenceAutomatonOf(0, []int{0}, nil), nil
		}
		return sequenceEmptyAutomaton(), nil
	}
	return sequenceAutomatonOf(0, final, kept), nil
}

func sequenceAutomatonStateSet(states []int) ([]int, string) {
	seen := make(map[int]bool, len(states))
	unique := make([]int, 0, len(states))
	for _, state := range states {
		if !seen[state] {
			seen[state] = true
			unique = append(unique, state)
		}
	}
	sort.Ints(unique)
	var key strings.Builder
	for _, state := range unique {
		key.WriteString(strconv.Itoa(state))
		key.WriteByte(',')
	}
	return unique, key.String()
}

// deterministicMinterms is generate_min_terms. Each returned predicate is
// held once for the min_terms vector. In generate_min_terms_rec an
// undetermined minterm is dropped like an unsatisfiable one; without an
// expression solver (a Go-only mode) it makes the whole result undefined.
// new_pred_pos, neg and new_pred_neg live until the end of the else block,
// and curr_pred until generate_min_terms returns.
func (b *sequenceAutomatonBuilder) deterministicMinterms(labels []*sequenceAutomatonLabel) (assignments [][]bool, predicates []*sequenceAutomatonLabel, defined bool, err error) {
	defined = true
	bits := make([]bool, 0, len(labels))
	var visit func(int, *sequenceAutomatonLabel) error
	visit = func(index int, predicate *sequenceAutomatonLabel) (result error) {
		satisfiable, known, err := b.labelSatisfiable(predicate)
		if err != nil {
			return err
		}
		if !known && b.rewriter.solver == nil {
			defined = false
			return nil
		}
		if !known || !satisfiable {
			return nil
		}
		if index == len(labels) {
			copyBits := append([]bool(nil), bits...)
			assignments = append(assignments, copyBits)
			predicate.retain()
			predicates = append(predicates, predicate)
			return nil
		}
		var locals []*sequenceAutomatonLabel
		defer func() {
			for k := len(locals) - 1; k >= 0; k-- {
				result = errors.Join(result, locals[k].release())
			}
		}()
		bits = append(bits, true)
		positive, err := b.intersectLabels(predicate, labels[index])
		if err != nil {
			return err
		}
		positive.retain() // ref_t new_pred_pos
		locals = append(locals, positive)
		if err = visit(index+1, positive); err != nil || !defined {
			return err
		}
		bits = bits[:len(bits)-1]
		bits = append(bits, false)
		negated, err := b.negateLabel(labels[index])
		if err != nil {
			return err
		}
		negated.retain() // ref_t neg
		locals = append(locals, negated)
		negative, err := b.intersectLabels(predicate, negated)
		if err != nil {
			return err
		}
		negative.retain() // ref_t new_pred_neg
		locals = append(locals, negative)
		if err = visit(index+1, negative); err != nil || !defined {
			return err
		}
		bits = bits[:len(bits)-1]
		return nil
	}
	// ref_t curr_pred(m_ba.mk_true()): mk_pred(true, Bool).
	boolean, err := b.rewriter.manager.MkBoolSort()
	if err != nil {
		return nil, nil, false, err
	}
	root, err := b.trueLabel(boolean)
	if err != nil {
		return nil, nil, false, err
	}
	root.retain()
	err = visit(0, root)
	if err = errors.Join(err, root.release()); err != nil {
		return nil, nil, false, err
	}
	if !defined {
		return nil, nil, false, releaseSequenceLabels(predicates)
	}
	return assignments, predicates, defined, nil
}

// releaseSequenceLabels is the destructor of a vector of ref_t labels.
func releaseSequenceLabels(labels []*sequenceAutomatonLabel) (err error) {
	for _, l := range labels {
		err = errors.Join(err, l.release())
	}
	return err
}

// determinize is symbolic_automata::mk_determinstic_param. min_terms is
// reset for each state, and with new_mvs and the input moves it is destroyed
// after the result is made; with no final state the result is empty and
// new_mvs then frees every minterm it held.
func (b *sequenceAutomatonBuilder) determinize(input *sequenceAutomaton, flipAcceptance bool) (result *sequenceAutomaton, resultErr error) {
	initial, initialKey := sequenceAutomatonStateSet(input.epsilonClosure(input.initial))
	states := [][]int{initial}
	ids := map[string]int{initialKey: 0}
	todo := []int{0}
	var newMoves []sequenceAutomatonMove
	var newFinal []int
	var minTerms []*sequenceAutomatonLabel
	defer func() {
		resultErr = errors.Join(resultErr, releaseSequenceMoves(newMoves), releaseSequenceLabels(minTerms))
	}()
	setFinal := func(id int, set []int) {
		final := false
		for _, state := range set {
			final = final || input.finals[state]
		}
		if final != flipAcceptance {
			newFinal = append(newFinal, id)
		}
	}
	setFinal(0, initial)
	for len(todo) > 0 {
		stateID := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		// min_terms.reset()
		err := releaseSequenceLabels(minTerms)
		minTerms = nil
		if err != nil {
			return nil, err
		}
		moves := make([]sequenceAutomatonMove, 0, 4)
		for _, state := range states[stateID] {
			moves = append(moves, input.movesFrom(state, true)...)
		}
		labels := make([]*sequenceAutomatonLabel, len(moves))
		for i := range moves {
			labels[i] = moves[i].label
		}
		assignments, predicates, defined, err := b.deterministicMinterms(labels)
		if err != nil {
			return nil, err
		}
		if !defined {
			return nil, nil
		}
		minTerms = predicates
		for i, assignment := range assignments {
			targetStates := make([]int, 0, len(assignment))
			for j, selected := range assignment {
				if selected {
					targetStates = append(targetStates, moves[j].destination)
				}
			}
			targetSet, targetKey := sequenceAutomatonStateSet(targetStates)
			targetID, present := ids[targetKey]
			if !present {
				targetID = len(states)
				ids[targetKey] = targetID
				states = append(states, targetSet)
				setFinal(targetID, targetSet)
				todo = append(todo, targetID)
			}
			predicates[i].retain() // new_mvs.push_back(move_t(...))
			newMoves = append(newMoves, sequenceAutomatonMove{source: stateID, destination: targetID, label: predicates[i]})
		}
	}
	if len(newFinal) == 0 {
		return sequenceEmptyAutomaton(), nil
	}
	return sequenceAutomatonOf(0, newFinal, newMoves), nil
}

// regex is re2automaton::re2aut. Its scoped_ptr<eautomaton> a, b destroy
// the intermediate automata when it returns (b before a), and assigning to
// one destroys the automaton it held after the new one is made.
func (b *sequenceAutomatonBuilder) regex(expression ast.ASTHandle) (*sequenceAutomaton, error) {
	m := b.rewriter.manager
	if argument, matched, err := m.SequenceUnaryOperationArgument(expression, 11); err != nil {
		return nil, err
	} else if matched {
		return b.sequence(argument)
	}
	for _, row := range []struct {
		kind int32
		make func(*sequenceAutomaton, *sequenceAutomaton) *sequenceAutomaton
	}{{17, sequenceConcatAutomaton}, {18, sequenceUnionAutomaton}} {
		left, right, matched, err := m.SequenceBinaryOperationArguments(expression, row.kind)
		if err != nil {
			return nil, err
		}
		if matched {
			leftAutomaton, err := b.regex(left)
			if err != nil || leftAutomaton == nil {
				return nil, errors.Join(err, leftAutomaton.release())
			}
			rightAutomaton, err := b.regex(right)
			if err != nil || rightAutomaton == nil {
				return nil, errors.Join(err, rightAutomaton.release(), leftAutomaton.release())
			}
			result := row.make(leftAutomaton, rightAutomaton)
			return result, errors.Join(rightAutomaton.release(), leftAutomaton.release())
		}
	}
	for _, row := range []struct {
		kind int32
		make func(*sequenceAutomaton) *sequenceAutomaton
	}{{14, func(a *sequenceAutomaton) *sequenceAutomaton {
		a.addFinalToInitMoves()
		a.addInitToFinalStates()
		return a
	}},
		{13, func(a *sequenceAutomaton) *sequenceAutomaton {
			a.addFinalToInitMoves()
			return a
		}},
		{15, sequenceOptionalAutomaton}} {
		argument, matched, err := m.SequenceUnaryOperationArgument(expression, row.kind)
		if err != nil {
			return nil, err
		}
		if matched {
			automaton, err := b.regex(argument)
			if err != nil || automaton == nil {
				return nil, errors.Join(err, automaton.release())
			}
			result := row.make(automaton)
			if result != automaton { // a = eautomaton::mk_opt(*a)
				err = automaton.release()
			}
			return result, err
		}
	}
	if lower, upper, matched, err := m.SequenceBinaryOperationArguments(expression, 16); err != nil {
		return nil, err
	} else if matched {
		lo, loString, err := m.SequenceStringValue(lower)
		if err != nil {
			return nil, err
		}
		hi, hiString, err := m.SequenceStringValue(upper)
		if err != nil {
			return nil, err
		}
		if !loString || !hiString || len(lo) != 1 || len(hi) != 1 {
			return nil, nil
		}
		// expr_ref _start(...), _stop(...); sym_expr::mk_range(_start, _stop)
		loValue, err := m.BitVectorNumeralUnsigned(uint32(lo[0]), 8)
		if err != nil {
			return nil, err
		}
		hiValue, err := m.BitVectorNumeralUnsigned(uint32(hi[0]), 8)
		if err != nil {
			return nil, err
		}
		label, err := b.rangeLabel(loValue, hiValue)
		if err != nil {
			return nil, err
		}
		return sequenceLabelAutomaton(label), nil
	}
	body, lower, upper, bounded, loop, err := m.RegexLoopParameters(expression)
	if err != nil {
		return nil, err
	}
	if loop {
		if lower < 0 || bounded && upper < 0 {
			return nil, nil
		}
		a, err := b.regex(body)
		if err != nil || a == nil {
			return nil, errors.Join(err, a.release())
		}
		lo, hi := lower, upper
		var result *sequenceAutomaton
		if bounded {
			eps := sequenceEpsilonAutomaton()
			result = sequenceEpsilonAutomaton()
			for hi > lo {
				c := sequenceConcatAutomaton(a, result)
				next := sequenceUnionAutomaton(eps, c)
				err = errors.Join(err, result.release(), c.release())
				result = next
				hi--
			}
			for lo > 0 {
				next := sequenceConcatAutomaton(a, result)
				err = errors.Join(err, result.release())
				result = next
				lo--
			}
			err = errors.Join(err, eps.release())
		} else {
			result = a.clone()
			result.addFinalToInitMoves()
			result.addInitToFinalStates()
			for lo > 0 {
				next := sequenceConcatAutomaton(a, result)
				err = errors.Join(err, result.release())
				result = next
				lo--
			}
		}
		return result, errors.Join(err, a.release())
	}
	if left, right, intersection, err := m.SequenceBinaryOperationArguments(expression, 19); err != nil {
		return nil, err
	} else if intersection {
		leftAutomaton, err := b.regex(left)
		if err != nil || leftAutomaton == nil {
			return nil, errors.Join(err, leftAutomaton.release())
		}
		rightAutomaton, err := b.regex(right)
		if err != nil || rightAutomaton == nil {
			return nil, errors.Join(err, rightAutomaton.release(), leftAutomaton.release())
		}
		result, err := b.product(leftAutomaton, rightAutomaton)
		return result, errors.Join(err, rightAutomaton.release(), leftAutomaton.release())
	}
	if body, complement, err := m.SequenceUnaryOperationArgument(expression, 21); err != nil {
		return nil, err
	} else if complement {
		automaton, err := b.regex(body)
		if err != nil || automaton == nil {
			return nil, errors.Join(err, automaton.release())
		}
		result, err := b.determinize(automaton, true)
		return result, errors.Join(err, automaton.release())
	}
	// full_seq and full_char use sym_expr::mk_pred(true, char_s).
	fullTrue := func() (*sequenceAutomatonLabel, error) {
		charSort, err := b.regexCharacterSort(expression)
		if err != nil {
			return nil, err
		}
		return b.trueLabel(charSort)
	}
	for _, row := range []struct {
		kind int32
		make func() (*sequenceAutomaton, error)
	}{{22, func() (*sequenceAutomaton, error) { return sequenceEmptyAutomaton(), nil }}, {23, func() (*sequenceAutomaton, error) {
		label, err := fullTrue()
		if err != nil {
			return nil, err
		}
		return sequenceLoopAutomaton(label), nil
	}},
		{24, func() (*sequenceAutomaton, error) {
			label, err := fullTrue()
			if err != nil {
				return nil, err
			}
			return sequenceLabelAutomaton(label), nil
		}}} {
		matched, err := b.rewriter.sequenceOperation(expression, row.kind)
		if err != nil {
			return nil, err
		}
		if matched {
			return row.make()
		}
	}
	return nil, nil
}

// RegexByteStringCandidates enumerates concrete byte strings in breadth-first
// automaton order. Exhaustive is true when the concrete language was finite
// and completely traversed before limit was reached. Symbolic labels are left
// to the lazy automaton path.
func (r *SequenceRewriter) RegexByteStringCandidates(regex ast.ASTHandle, limit int) (words []string, exhaustive bool, result error) {
	if err := r.open(); err != nil {
		return nil, false, err
	}
	if limit <= 0 {
		return nil, false, ast.ErrASTDomain
	}
	builder := &sequenceAutomatonBuilder{rewriter: r}
	var automaton *sequenceAutomaton
	defer func() { result = errors.Join(result, builder.close(), automaton.release()) }()
	automaton, err := builder.regex(regex)
	if err != nil || automaton == nil {
		return nil, false, err
	}
	canReachFinal := automaton.statesReachingFinal()
	type candidate struct {
		state int
		word  string
	}
	queue := []candidate{{state: automaton.initial}}
	seenConfigurations := map[string]struct{}{strconv.Itoa(automaton.initial) + ":": {}}
	seenWords := make(map[string]struct{})
	symbolicPath := false
	for len(queue) != 0 {
		current := queue[0]
		queue = queue[1:]
		closure := automaton.epsilonClosure(current.state)
		for _, state := range closure {
			if automaton.finals[state] {
				if _, found := seenWords[current.word]; !found {
					seenWords[current.word] = struct{}{}
					words = append(words, current.word)
					if len(words) == limit {
						return words, false, nil
					}
				}
				break
			}
		}
		for _, source := range closure {
			for _, move := range automaton.moves[source] {
				if move.label == nil || !canReachFinal[move.destination] {
					continue
				}
				values, concrete, valueErr := builder.concreteLabelBytes(move.label)
				if valueErr != nil {
					return nil, false, valueErr
				}
				if !concrete {
					symbolicPath = true
					continue
				}
				for value, accepted := range values {
					if !accepted {
						continue
					}
					nextWord := current.word + string([]byte{byte(value)})
					key := strconv.Itoa(move.destination) + ":" + nextWord
					if _, found := seenConfigurations[key]; found {
						continue
					}
					seenConfigurations[key] = struct{}{}
					queue = append(queue, candidate{
						state: move.destination,
						word:  nextWord,
					})
				}
			}
		}
	}
	return words, !symbolicPath, nil
}

// RegexAcceptedLengths enumerates word lengths that have a structurally
// accepting automaton path. Symbolic labels are treated as potentially
// satisfiable; the exact fixed-word reducer supplies their constraints.
func (r *SequenceRewriter) RegexAcceptedLengths(regex ast.ASTHandle, limit int) (lengths []uint64, exhaustive bool, result error) {
	if err := r.open(); err != nil {
		return nil, false, err
	}
	if limit <= 0 {
		return nil, false, ast.ErrASTDomain
	}
	builder := &sequenceAutomatonBuilder{rewriter: r, allowUnknownLabels: true}
	var automaton *sequenceAutomaton
	defer func() { result = errors.Join(result, builder.close(), automaton.release()) }()
	automaton, err := builder.regex(regex)
	if err != nil || automaton == nil {
		return nil, false, err
	}
	type position struct {
		state  int
		length uint64
	}
	canReachFinal := automaton.statesReachingFinal()
	queue := []position{{state: automaton.initial}}
	seen := map[position]struct{}{{state: automaton.initial}: {}}
	seenLengths := make(map[uint64]struct{})
	for len(queue) != 0 {
		current := queue[0]
		queue = queue[1:]
		closure := automaton.epsilonClosure(current.state)
		for _, state := range closure {
			if !automaton.finals[state] {
				continue
			}
			if _, found := seenLengths[current.length]; !found {
				seenLengths[current.length] = struct{}{}
				lengths = append(lengths, current.length)
				if len(lengths) == limit {
					return lengths, false, nil
				}
			}
			break
		}
		for _, source := range closure {
			for _, move := range automaton.moves[source] {
				if move.label == nil || !canReachFinal[move.destination] {
					continue
				}
				values, concrete, valueErr := builder.concreteLabelBytes(move.label)
				if valueErr != nil {
					return nil, false, valueErr
				}
				if concrete {
					satisfiable := false
					for _, accepted := range values {
						satisfiable = satisfiable || accepted
					}
					if !satisfiable {
						continue
					}
				}
				next := position{state: move.destination, length: current.length + 1}
				if _, found := seen[next]; found {
					continue
				}
				seen[next] = struct{}{}
				queue = append(queue, next)
			}
		}
	}
	return lengths, true, nil
}

// ShortestRegexByteString returns the first concrete witness accepted by regex.
func (r *SequenceRewriter) ShortestRegexByteString(regex ast.ASTHandle) (string, bool, error) {
	words, _, err := r.RegexByteStringCandidates(regex, 1)
	if err != nil || len(words) == 0 {
		return "", false, err
	}
	return words[0], true, nil
}

// accept is sym_expr::accept for callers that keep the result for the
// builder's lifetime.
func (b *sequenceAutomatonBuilder) accept(label *sequenceAutomatonLabel, character ast.ASTHandle) (ast.ASTHandle, error) {
	ref, err := b.acceptOwned(label, character)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	b.refs = append(b.refs, ref)
	return b.rewriter.manager.RefGet(ref)
}

// acceptOwned is sym_expr::accept returning its expr_ref result; the caller
// releases it.
func (b *sequenceAutomatonBuilder) acceptOwned(label *sequenceAutomatonLabel, character ast.ASTHandle) (ast.ASTRef, error) {
	m := b.rewriter.manager
	own := func(h ast.ASTHandle, err error) (ast.ASTRef, error) {
		if err != nil {
			return ast.ASTRef{}, err
		}
		return m.NewRef(h)
	}
	switch label.kind {
	case sequenceLabelCharacter:
		return own(m.MkEq(character, label.character))
	case sequenceLabelRange:
		lo, loNumeral, err := m.SequenceUnitByteElementValue(label.lowerBound)
		if err != nil {
			return ast.ASTRef{}, err
		}
		value, valueNumeral, err := m.SequenceUnitByteElementValue(character)
		if err != nil {
			return ast.ASTRef{}, err
		}
		hi, hiNumeral, err := m.SequenceUnitByteElementValue(label.upperBound)
		if err != nil {
			return ast.ASTRef{}, err
		}
		if loNumeral && valueNumeral && hiNumeral {
			return own(m.MkBoolVal(lo <= value && value <= hi))
		}
		// m.mk_and(bv.mk_ule(m_t, e), bv.mk_ule(e, m_s)): GCC builds the
		// second argument first.
		upper, err := m.MkBitVectorULE(character, label.upperBound)
		if err != nil {
			return ast.ASTRef{}, err
		}
		lower, err := m.MkBitVectorULE(label.lowerBound, character)
		if err != nil {
			return ast.ASTRef{}, err
		}
		return own(m.MkAndArray([]ast.ASTHandle{lower, upper}))
	}
	// var_subst subst(m); subst(m_t, 1, &e, result).
	subst, err := NewVarSubstitution(m, true)
	if err != nil {
		return ast.ASTRef{}, err
	}
	out, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, errors.Join(err, subst.Close())
	}
	err = subst.Substitute(label.predicate, []ast.ASTHandle{character}, out)
	if err = errors.Join(err, subst.Close()); err != nil {
		return ast.ASTRef{}, errors.Join(err, m.DelRef(&out))
	}
	return out, nil
}
