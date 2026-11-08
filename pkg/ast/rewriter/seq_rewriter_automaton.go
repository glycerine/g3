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
)

type sequenceAutomatonLabel struct {
	kind       sequenceAutomatonLabelKind
	character  ast.ASTHandle
	lowerBound ast.ASTHandle
	upperBound ast.ASTHandle
	operands   []*sequenceAutomatonLabel
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
// same source (is_duplicate_cheap).
func (a *sequenceAutomaton) add(mv sequenceAutomatonMove) {
	out := a.moves[mv.source]
	if n := len(out); n > 0 && out[n-1].destination == mv.destination && out[n-1].label == mv.label {
		return
	}
	a.moves[mv.source] = append(out, mv)
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

func (b *sequenceAutomatonBuilder) character(value byte) (ast.ASTHandle, error) {
	h, err := b.rewriter.manager.BitVectorNumeralUnsigned(uint32(value), 8)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = b.hold(h); err != nil {
		return ast.ASTHandle{}, err
	}
	return h, nil
}

// sequence is re2automaton::seq2aut.
func (b *sequenceAutomatonBuilder) sequence(expression ast.ASTHandle) (*sequenceAutomaton, error) {
	m := b.rewriter.manager
	if left, right, matched, err := m.SequenceConcatParts(expression); err != nil {
		return nil, err
	} else if matched {
		a, err := b.sequence(left)
		if err != nil || a == nil {
			return nil, err
		}
		c, err := b.sequence(right)
		if err != nil || c == nil {
			return nil, err
		}
		return sequenceConcatAutomaton(a, c), nil
	}
	if element, unit, err := m.SequenceUnitArgument(expression); err != nil {
		return nil, err
	} else if unit {
		return sequenceLabelAutomaton(&sequenceAutomatonLabel{kind: sequenceLabelCharacter, character: element}), nil
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
		character, err := b.character(text[k])
		if err != nil {
			return nil, err
		}
		moves = append(moves, sequenceAutomatonMove{source: k, destination: k + 1, label: &sequenceAutomatonLabel{kind: sequenceLabelCharacter, character: character}})
	}
	return sequenceAutomatonOf(0, []int{len(text)}, moves), nil
}

func (b *sequenceAutomatonBuilder) intersectLabels(left, right *sequenceAutomatonLabel) *sequenceAutomatonLabel {
	if left.kind == sequenceLabelAny {
		return right
	}
	if right.kind == sequenceLabelAny {
		return left
	}
	return &sequenceAutomatonLabel{kind: sequenceLabelIntersection, operands: []*sequenceAutomatonLabel{left, right}}
}

func (b *sequenceAutomatonBuilder) negateLabel(label *sequenceAutomatonLabel) *sequenceAutomatonLabel {
	if label.kind == sequenceLabelNegation && len(label.operands) == 1 {
		return label.operands[0]
	}
	return &sequenceAutomatonLabel{kind: sequenceLabelNegation, operands: []*sequenceAutomatonLabel{label}}
}

func (b *sequenceAutomatonBuilder) labelSort(label *sequenceAutomatonLabel) (ast.ASTHandle, error) {
	switch label.kind {
	case sequenceLabelCharacter:
		return b.rewriter.manager.GetSort(label.character)
	case sequenceLabelRange:
		return b.rewriter.manager.GetSort(label.lowerBound)
	case sequenceLabelIntersection, sequenceLabelNegation:
		if len(label.operands) == 0 {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		return b.labelSort(label.operands[0])
	default:
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
}

func (b *sequenceAutomatonBuilder) concreteLabelBytes(label *sequenceAutomatonLabel) (values [256]bool, concrete bool, err error) {
	switch label.kind {
	case sequenceLabelAny:
		for i := range values {
			values[i] = true
		}
		return values, true, nil
	case sequenceLabelCharacter:
		value, numeral, err := b.rewriter.manager.SequenceUnitByteElementValue(label.character)
		if err != nil || !numeral {
			return values, false, err
		}
		values[value] = true
		return values, true, nil
	case sequenceLabelRange:
		lo, loNumeral, err := b.rewriter.manager.SequenceUnitByteElementValue(label.lowerBound)
		if err != nil {
			return values, false, err
		}
		hi, hiNumeral, err := b.rewriter.manager.SequenceUnitByteElementValue(label.upperBound)
		if err != nil || !loNumeral || !hiNumeral {
			return values, false, err
		}
		for value := int(lo); value <= int(hi); value++ {
			values[value] = true
		}
		return values, true, nil
	case sequenceLabelIntersection:
		for i := range values {
			values[i] = true
		}
		for _, operand := range label.operands {
			operandValues, operandConcrete, err := b.concreteLabelBytes(operand)
			if err != nil || !operandConcrete {
				return [256]bool{}, false, err
			}
			for i := range values {
				values[i] = values[i] && operandValues[i]
			}
		}
		return values, true, nil
	case sequenceLabelNegation:
		if len(label.operands) != 1 {
			return values, false, ast.ErrASTDomain
		}
		operandValues, operandConcrete, err := b.concreteLabelBytes(label.operands[0])
		if err != nil || !operandConcrete {
			return values, false, err
		}
		for i := range values {
			values[i] = !operandValues[i]
		}
		return values, true, nil
	default:
		return values, false, ast.ErrASTDomain
	}
}

func (b *sequenceAutomatonBuilder) labelSatisfiable(label *sequenceAutomatonLabel) (satisfiable, defined bool, err error) {
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
	if label.kind == sequenceLabelAny || label.kind == sequenceLabelCharacter {
		return true, true, nil
	}
	if label.kind == sequenceLabelRange {
		lo, loNumeral, err := b.rewriter.manager.SequenceUnitByteElementValue(label.lowerBound)
		if err != nil {
			return false, false, err
		}
		hi, hiNumeral, err := b.rewriter.manager.SequenceUnitByteElementValue(label.upperBound)
		if err != nil {
			return false, false, err
		}
		if loNumeral && hiNumeral {
			return lo <= hi, true, nil
		}
	}
	if b.rewriter.solver == nil {
		if b.allowUnknownLabels {
			return true, true, nil
		}
		return false, false, nil
	}
	sort, err := b.labelSort(label)
	if err != nil {
		return false, false, err
	}
	variable, err := b.rewriter.manager.MkFreshConst("x", sort) // sym_expr_boolean_algebra::is_sat
	if err != nil {
		return false, false, err
	}
	if err = b.hold(variable); err != nil {
		return false, false, err
	}
	formula, err := b.accept(label, variable)
	if err != nil {
		return false, false, err
	}
	if b.rewriter.manager.IsTrue(formula) {
		return true, true, nil
	}
	if b.rewriter.manager.IsFalse(formula) {
		return false, true, nil
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

func (b *sequenceAutomatonBuilder) product(left, right *sequenceAutomaton) (*sequenceAutomaton, error) {
	type pair struct{ left, right int }
	initial := pair{left.initial, right.initial}
	ids := map[pair]int{initial: 0}
	todo := []pair{initial}
	result := newSequenceAutomaton(1)
	if left.closureIsFinal(initial.left) && right.closureIsFinal(initial.right) {
		result.addFinal(0)
	}
	for len(todo) > 0 {
		current := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		source := ids[current]
		for _, leftMove := range left.movesFrom(current.left, true) {
			for _, rightMove := range right.movesFrom(current.right, true) {
				label := b.intersectLabels(leftMove.label, rightMove.label)
				satisfiable, defined, err := b.labelSatisfiable(label)
				if err != nil {
					return nil, err
				}
				if !defined {
					return nil, nil
				}
				if !satisfiable {
					continue
				}
				targetPair := pair{leftMove.destination, rightMove.destination}
				target, present := ids[targetPair]
				if !present {
					target = len(ids)
					ids[targetPair] = target
					result.moves = append(result.moves, nil)
					if left.closureIsFinal(targetPair.left) && right.closureIsFinal(targetPair.right) {
						result.addFinal(target)
					}
					todo = append(todo, targetPair)
				}
				result.moves[source] = append(result.moves[source], sequenceAutomatonMove{source: source, destination: target, label: label})
			}
		}
	}
	return result, nil
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

func (b *sequenceAutomatonBuilder) deterministicMinterms(labels []*sequenceAutomatonLabel) (assignments [][]bool, predicates []*sequenceAutomatonLabel, defined bool, err error) {
	defined = true
	bits := make([]bool, 0, len(labels))
	var visit func(int, *sequenceAutomatonLabel) error
	visit = func(index int, predicate *sequenceAutomatonLabel) error {
		satisfiable, known, err := b.labelSatisfiable(predicate)
		if err != nil {
			return err
		}
		if !known {
			defined = false
			return nil
		}
		if !satisfiable {
			return nil
		}
		if index == len(labels) {
			copyBits := append([]bool(nil), bits...)
			assignments = append(assignments, copyBits)
			predicates = append(predicates, predicate)
			return nil
		}
		bits = append(bits, true)
		if err = visit(index+1, b.intersectLabels(predicate, labels[index])); err != nil || !defined {
			return err
		}
		bits[len(bits)-1] = false
		if err = visit(index+1, b.intersectLabels(predicate, b.negateLabel(labels[index]))); err != nil || !defined {
			return err
		}
		bits = bits[:len(bits)-1]
		return nil
	}
	if err = visit(0, &sequenceAutomatonLabel{kind: sequenceLabelAny}); err != nil {
		return nil, nil, false, err
	}
	return assignments, predicates, defined, nil
}

func (b *sequenceAutomatonBuilder) determinize(input *sequenceAutomaton, flipAcceptance bool) (*sequenceAutomaton, error) {
	initial, initialKey := sequenceAutomatonStateSet(input.epsilonClosure(input.initial))
	states := [][]int{initial}
	ids := map[string]int{initialKey: 0}
	todo := []int{0}
	result := newSequenceAutomaton(1)
	setFinal := func(id int, set []int) {
		final := false
		for _, state := range set {
			final = final || input.finals[state]
		}
		if final != flipAcceptance {
			result.addFinal(id)
		}
	}
	setFinal(0, initial)
	for len(todo) > 0 {
		stateID := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
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
				result.moves = append(result.moves, nil)
				setFinal(targetID, targetSet)
				todo = append(todo, targetID)
			}
			result.moves[stateID] = append(result.moves[stateID], sequenceAutomatonMove{source: stateID, destination: targetID, label: predicates[i]})
		}
	}
	return result, nil
}

func (b *sequenceAutomatonBuilder) regex(expression ast.ASTHandle) (*sequenceAutomaton, error) {
	if argument, matched, err := b.rewriter.manager.SequenceUnaryOperationArgument(expression, 11); err != nil {
		return nil, err
	} else if matched {
		return b.sequence(argument)
	}
	for _, row := range []struct {
		kind int32
		make func(*sequenceAutomaton, *sequenceAutomaton) *sequenceAutomaton
	}{{17, sequenceConcatAutomaton}, {18, sequenceUnionAutomaton}} {
		left, right, matched, err := b.rewriter.manager.SequenceBinaryOperationArguments(expression, row.kind)
		if err != nil {
			return nil, err
		}
		if matched {
			leftAutomaton, err := b.regex(left)
			if err != nil || leftAutomaton == nil {
				return nil, err
			}
			rightAutomaton, err := b.regex(right)
			if err != nil || rightAutomaton == nil {
				return nil, err
			}
			return row.make(leftAutomaton, rightAutomaton), nil
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
		argument, matched, err := b.rewriter.manager.SequenceUnaryOperationArgument(expression, row.kind)
		if err != nil {
			return nil, err
		}
		if matched {
			automaton, err := b.regex(argument)
			if err != nil || automaton == nil {
				return nil, err
			}
			return row.make(automaton), nil
		}
	}
	if lower, upper, matched, err := b.rewriter.manager.SequenceBinaryOperationArguments(expression, 16); err != nil {
		return nil, err
	} else if matched {
		lo, loString, err := b.rewriter.manager.SequenceStringValue(lower)
		if err != nil {
			return nil, err
		}
		hi, hiString, err := b.rewriter.manager.SequenceStringValue(upper)
		if err != nil {
			return nil, err
		}
		if !loString || !hiString || len(lo) != 1 || len(hi) != 1 {
			return nil, nil
		}
		loValue, err := b.character(lo[0])
		if err != nil {
			return nil, err
		}
		hiValue, err := b.character(hi[0])
		if err != nil {
			return nil, err
		}
		return sequenceLabelAutomaton(&sequenceAutomatonLabel{kind: sequenceLabelRange, lowerBound: loValue, upperBound: hiValue}), nil
	}
	body, lower, upper, bounded, loop, err := b.rewriter.manager.RegexLoopParameters(expression)
	if err != nil {
		return nil, err
	}
	if loop {
		if lower < 0 || bounded && upper < 0 {
			return nil, nil
		}
		a, err := b.regex(body)
		if err != nil || a == nil {
			return nil, err
		}
		lo, hi := lower, upper
		if bounded {
			eps := sequenceEpsilonAutomaton()
			result := sequenceEpsilonAutomaton()
			for hi > lo {
				c := sequenceConcatAutomaton(a, result)
				result = sequenceUnionAutomaton(eps, c)
				hi--
			}
			for lo > 0 {
				result = sequenceConcatAutomaton(a, result)
				lo--
			}
			return result, nil
		}
		result := a.clone()
		result.addFinalToInitMoves()
		result.addInitToFinalStates()
		for lo > 0 {
			result = sequenceConcatAutomaton(a, result)
			lo--
		}
		return result, nil
	}
	if left, right, intersection, err := b.rewriter.manager.SequenceBinaryOperationArguments(expression, 19); err != nil {
		return nil, err
	} else if intersection {
		leftAutomaton, err := b.regex(left)
		if err != nil || leftAutomaton == nil {
			return nil, err
		}
		rightAutomaton, err := b.regex(right)
		if err != nil || rightAutomaton == nil {
			return nil, err
		}
		return b.product(leftAutomaton, rightAutomaton)
	}
	if body, complement, err := b.rewriter.manager.SequenceUnaryOperationArgument(expression, 21); err != nil {
		return nil, err
	} else if complement {
		automaton, err := b.regex(body)
		if err != nil || automaton == nil {
			return nil, err
		}
		return b.determinize(automaton, true)
	}
	for _, row := range []struct {
		kind int32
		make func() *sequenceAutomaton
	}{{22, sequenceEmptyAutomaton}, {23, func() *sequenceAutomaton {
		return sequenceLoopAutomaton(&sequenceAutomatonLabel{kind: sequenceLabelAny})
	}},
		{24, func() *sequenceAutomaton {
			return sequenceLabelAutomaton(&sequenceAutomatonLabel{kind: sequenceLabelAny})
		}}} {
		matched, err := b.rewriter.sequenceOperation(expression, row.kind)
		if err != nil {
			return nil, err
		}
		if matched {
			return row.make(), nil
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
	defer func() { result = errors.Join(result, builder.close()) }()
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
	defer func() { result = errors.Join(result, builder.close()) }()
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

func (b *sequenceAutomatonBuilder) automatonSequence(a *sequenceAutomaton) ([]ast.ASTHandle, bool) {
	state := a.initial
	visited := make(map[int]bool)
	result := make([]ast.ASTHandle, 0, 4)
	for {
		closure := a.epsilonClosure(state)
		final := false
		moves := make([]sequenceAutomatonMove, 0, 2)
		for _, source := range closure {
			final = final || a.finals[source]
			for _, move := range a.moves[source] {
				if move.label != nil {
					moves = append(moves, move)
				}
			}
		}
		if final {
			return result, len(moves) == 0
		}
		if len(moves) != 1 || visited[state] || moves[0].label.kind != sequenceLabelCharacter {
			return nil, false
		}
		visited[state] = true
		result = append(result, moves[0].label.character)
		state = moves[0].destination
	}
}

func (b *sequenceAutomatonBuilder) expressionSequence(expression ast.ASTHandle) ([]ast.ASTHandle, bool, error) {
	leaves, err := b.rewriter.manager.SequenceFlattenConcat(expression)
	if err != nil {
		return nil, false, err
	}
	result := make([]ast.ASTHandle, 0, len(leaves))
	for _, leaf := range leaves {
		text, stringValue, err := b.rewriter.manager.SequenceStringValue(leaf)
		if err != nil {
			return nil, false, err
		}
		if stringValue {
			for i := 0; i < len(text); i++ {
				character, err := b.character(text[i])
				if err != nil {
					return nil, false, err
				}
				result = append(result, character)
			}
			continue
		}
		element, unit, err := b.rewriter.manager.SequenceUnitArgument(leaf)
		if err != nil {
			return nil, false, err
		}
		if !unit {
			return nil, false, nil
		}
		result = append(result, element)
	}
	return result, true, nil
}

func (b *sequenceAutomatonBuilder) booleanAnd(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	if b.rewriter.manager.IsFalse(left) || b.rewriter.manager.IsFalse(right) {
		return b.rewriter.manager.MkFalse()
	}
	if b.rewriter.manager.IsTrue(left) {
		return right, nil
	}
	if b.rewriter.manager.IsTrue(right) || left == right {
		return left, nil
	}
	result, err := b.rewriter.manager.MkAndArray([]ast.ASTHandle{left, right})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = b.hold(result); err != nil {
		return ast.ASTHandle{}, err
	}
	return result, nil
}

func (b *sequenceAutomatonBuilder) booleanOr(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	if b.rewriter.manager.IsTrue(left) || b.rewriter.manager.IsTrue(right) {
		return b.rewriter.manager.MkTrue()
	}
	if b.rewriter.manager.IsFalse(left) {
		return right, nil
	}
	if b.rewriter.manager.IsFalse(right) || left == right {
		return left, nil
	}
	result, err := b.rewriter.manager.MkOrArray([]ast.ASTHandle{left, right})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = b.hold(result); err != nil {
		return ast.ASTHandle{}, err
	}
	return result, nil
}

func (b *sequenceAutomatonBuilder) accept(label *sequenceAutomatonLabel, character ast.ASTHandle) (ast.ASTHandle, error) {
	if label.kind == sequenceLabelAny {
		return b.rewriter.manager.MkTrue()
	}
	if label.kind == sequenceLabelIntersection {
		result, err := b.rewriter.manager.MkTrue()
		if err != nil {
			return ast.ASTHandle{}, err
		}
		for _, operand := range label.operands {
			condition, err := b.accept(operand, character)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			result, err = b.booleanAnd(result, condition)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		return result, nil
	}
	if label.kind == sequenceLabelNegation {
		if len(label.operands) != 1 {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		condition, err := b.accept(label.operands[0], character)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		result, err := b.rewriter.manager.MkNotSimplified(condition)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if err = b.hold(result); err != nil {
			return ast.ASTHandle{}, err
		}
		return result, nil
	}
	if label.kind == sequenceLabelCharacter {
		labelValue, err := b.rewriter.manager.IsValue(label.character)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		characterValue, err := b.rewriter.manager.IsValue(character)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if labelValue && characterValue {
			return b.rewriter.manager.MkBoolVal(label.character == character)
		}
		result, err := b.rewriter.manager.MkEq(character, label.character)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if err = b.hold(result); err != nil {
			return ast.ASTHandle{}, err
		}
		return result, nil
	}
	lo, loNumeral, err := b.rewriter.manager.SequenceUnitByteElementValue(label.lowerBound)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	value, valueNumeral, err := b.rewriter.manager.SequenceUnitByteElementValue(character)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	hi, hiNumeral, err := b.rewriter.manager.SequenceUnitByteElementValue(label.upperBound)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if loNumeral && valueNumeral && hiNumeral {
		return b.rewriter.manager.MkBoolVal(lo <= value && value <= hi)
	}
	lower, err := b.rewriter.manager.MkBitVectorULE(label.lowerBound, character)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = b.hold(lower); err != nil {
		return ast.ASTHandle{}, err
	}
	upper, err := b.rewriter.manager.MkBitVectorULE(character, label.upperBound)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = b.hold(upper); err != nil {
		return ast.ASTHandle{}, err
	}
	return b.booleanAnd(lower, upper)
}

func (r *SequenceRewriter) rewriteStringInRegex(sequence, regex ast.ASTHandle, out ast.ASTRef) (status RewriteStatus, err error) {
	if err = r.open(); err != nil {
		return RewriteFailed, err
	}
	empty, err := r.sequenceOperation(regex, 22)
	if err != nil {
		return RewriteFailed, err
	}
	if empty {
		return r.assignBoolean(false, out)
	}
	full, err := r.sequenceOperation(regex, 23)
	if err != nil {
		return RewriteFailed, err
	}
	if full {
		return r.assignBoolean(true, out)
	}
	builder := sequenceAutomatonBuilder{rewriter: r}
	defer func() { err = errors.Join(err, builder.close()) }()
	automaton, err := builder.regex(regex)
	if err != nil || automaton == nil {
		return RewriteFailed, err
	}
	if elements, singleton := builder.automatonSequence(automaton); singleton {
		var value ast.ASTHandle
		if len(elements) == 0 {
			sort, e := r.manager.GetSort(sequence)
			if e != nil {
				return RewriteFailed, e
			}
			value, e = r.manager.MkSequenceEmpty(sort)
			if e != nil {
				return RewriteFailed, e
			}
			if e = builder.hold(value); e != nil {
				return RewriteFailed, e
			}
		} else {
			units := make([]ast.ASTHandle, 0, len(elements))
			for _, element := range elements {
				unit, e := r.manager.MkSequenceUnit(element)
				if e != nil {
					return RewriteFailed, e
				}
				if e = builder.hold(unit); e != nil {
					return RewriteFailed, e
				}
				units = append(units, unit)
			}
			valueRef, h, e := r.makeSequencePartsRef(units, sequence)
			if e != nil {
				return RewriteFailed, e
			}
			builder.refs = append(builder.refs, valueRef)
			value = h
		}
		result, e := r.manager.MkEq(sequence, value)
		if e != nil {
			return RewriteFailed, e
		}
		if err = r.manager.SetRef(out, result); err != nil {
			return RewriteFailed, err
		}
		return RewriteFull, nil
	}
	elements, concrete, err := builder.expressionSequence(sequence)
	if err != nil || !concrete {
		return RewriteFailed, err
	}
	truth, err := r.manager.MkTrue()
	if err != nil {
		return RewriteFailed, err
	}
	frontier := map[int]ast.ASTHandle{automaton.initial: truth}
	for _, character := range elements {
		next := make(map[int]ast.ASTHandle)
		for state, accumulated := range frontier {
			for _, move := range automaton.movesFrom(state, false) {
				condition, e := builder.accept(move.label, character)
				if e != nil {
					return RewriteFailed, e
				}
				if r.manager.IsFalse(condition) {
					continue
				}
				condition, e = builder.booleanAnd(condition, accumulated)
				if e != nil {
					return RewriteFailed, e
				}
				if previous, present := next[move.destination]; present && !r.manager.IsTrue(condition) {
					condition, e = builder.booleanOr(condition, previous)
					if e != nil {
						return RewriteFailed, e
					}
				}
				next[move.destination] = condition
			}
		}
		frontier = next
	}
	disjuncts := make([]ast.ASTHandle, 0, len(frontier))
	for state, condition := range frontier {
		if automaton.closureIsFinal(state) {
			disjuncts = append(disjuncts, condition)
		}
	}
	result, err := r.manager.MkOrArray(disjuncts)
	if err != nil {
		return RewriteFailed, err
	}
	if err = r.manager.SetRef(out, result); err != nil {
		return RewriteFailed, err
	}
	return RewriteFull, nil
}
