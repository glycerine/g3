package smtcore

// Source: src/smt/smt_conflict_resolution.cpp initialize_resolve(),
// process_antecedent(), resolve(), and the non-minimizing part of
// finalize_resolve().

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type SMTConflictResolution struct {
	Resolved           bool
	ConflictLevel      uint32
	NewScopeLevel      uint32
	InternalScopeLevel uint32
	Lemma              []SMTLiteral
}

type SMTConflictResolver struct {
	state        *SMTInternalizerState
	expander     *SMTAntecedentExpander
	lemmaProof   ast.ASTRef
	marks        []bool // reused across conflicts, as the source's context-owned marks
	levelsBuf    []uint32
	levelSeen    []bool
	levelTouched []uint32
	temporary    []int32
	stack        []int32
	closePhase   uint8
	closed       bool
}

// enodeLiteral reads a Boolean enode's literal in its current polarity, which is
// fixed for the duration of one conflict.
func (r *SMTConflictResolver) enodeLiteral(node *SMTEnode) (SMTLiteral, bool) {
	variable, found := r.state.boolVar.lookup(node.owner)
	if !found || !r.state.booleanData[variable].IsENode() {
		return SMTNullLiteral, false
	}
	literal := NewSMTLiteral(variable, false)
	if r.state.assignment[literal.Index()] == lbool.False {
		literal = literal.Negated()
	}
	return literal, true
}

func (r *SMTConflictResolver) resetMarks() []bool {
	n := len(r.state.booleanData)
	if cap(r.marks) < n {
		r.marks = make([]bool, n, n+n/2)
	} else {
		r.marks = r.marks[:n]
		clear(r.marks)
	}
	return r.marks
}

func (r *SMTConflictResolver) minimizeLemma(lemma []SMTLiteral, marks []bool, levels []uint32, baseLevel uint32) ([]SMTLiteral, error) {
	// Minimization overwrites lemma in place, so remember the levels it marked.
	touched := r.levelTouched[:0]
	for _, literal := range lemma {
		level := levels[literal.Var()]
		if int(level) >= len(r.levelSeen) {
			r.levelSeen = append(r.levelSeen, make([]bool, int(level)+1-len(r.levelSeen))...)
		}
		if !r.levelSeen[level] {
			r.levelSeen[level] = true
			touched = append(touched, level)
		}
	}
	r.levelTouched = touched
	defer func() {
		for _, level := range touched {
			r.levelSeen[level] = false
		}
	}()
	temporary := r.temporary[:0]
	defer func() { r.temporary = temporary[:0] }()
	var implied func(SMTLiteral) (bool, error)
	implied = func(literal SMTLiteral) (bool, error) {
		oldTemporary := len(temporary)
		stack := append(r.stack[:0], literal.Var())
		defer func() { r.stack = stack }()
		processAntecedent := func(antecedent SMTLiteral) bool {
			variable := antecedent.Var()
			level := levels[variable]
			if marks[variable] || level <= baseLevel {
				return true
			}
			if int(level) >= len(r.levelSeen) || !r.levelSeen[level] {
				return false
			}
			marks[variable] = true
			temporary = append(temporary, variable)
			stack = append(stack, variable)
			return true
		}
		fail := func() (bool, error) {
			for len(temporary) > oldTemporary {
				index := len(temporary) - 1
				marks[temporary[index]] = false
				temporary = temporary[:index]
			}
			return false, nil
		}
		for len(stack) != 0 {
			index := len(stack) - 1
			variable := stack[index]
			stack = stack[:index]
			justification := r.state.booleanData[variable].Justification()
			switch justification.Kind() {
			case SMTBooleanJustificationClause:
				clause, ok := justification.Clause()
				if !ok || clause == nil {
					return false, ast.ErrASTDomain
				}
				position := -1
				for i := 0; i < len(clause.literals) && i < 2; i++ {
					if clause.literals[i].Var() == variable {
						position = i
						break
					}
				}
				if position < 0 {
					return false, ast.ErrASTDomain
				}
				for i, item := range clause.literals {
					if i != position && !processAntecedent(item.Negated()) {
						return fail()
					}
				}
				if clause.justification != nil {
					antecedents, err := r.expander.expandReusing(clause.justification)
					if err != nil {
						return false, err
					}
					for _, antecedent := range antecedents {
						if !processAntecedent(antecedent) {
							return fail()
						}
					}
				}
			case SMTBooleanJustificationBinaryClause:
				antecedent, ok := justification.Literal()
				if !ok {
					return false, ast.ErrASTDomain
				}
				if !processAntecedent(antecedent) {
					return fail()
				}
			case SMTBooleanJustificationAxiom:
				if levels[variable] > baseLevel {
					return fail()
				}
			case SMTBooleanJustificationObject:
				if r.state.booleanData[variable].Assumption {
					return fail()
				}
				object, ok := justification.Justification()
				if !ok || object == nil {
					return false, ast.ErrASTDomain
				}
				antecedents, err := r.expander.expandReusing(object)
				if err != nil {
					return false, err
				}
				for _, antecedent := range antecedents {
					if !processAntecedent(antecedent) {
						return fail()
					}
				}
			default:
				return false, ast.ErrASTDomain
			}
		}
		return true, nil
	}
	kept := lemma[:1]
	for _, literal := range lemma[1:] {
		remove, err := implied(literal)
		if err != nil {
			return nil, err
		}
		if remove {
			marks[literal.Var()] = true
			temporary = append(temporary, literal.Var())
			r.state.statistics.NumMinimizedLits++
		} else {
			kept = append(kept, literal)
		}
	}
	for _, variable := range temporary {
		marks[variable] = false
	}
	return kept, nil
}

func NewSMTConflictResolver(state *SMTInternalizerState) (*SMTConflictResolver, error) {
	if state == nil || state.open() != nil {
		return nil, ast.ErrASTHandle
	}
	proof, err := state.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	resolver := &SMTConflictResolver{state: state, expander: NewSMTAntecedentExpander(), lemmaProof: proof}
	resolver.expander.literalOf = resolver.enodeLiteral
	return resolver, nil
}

// LemmaProof returns the resolver-owned proof produced by the most recent
// Resolve call. It is null when proof generation is disabled.
func (r *SMTConflictResolver) LemmaProof() (ast.ASTHandle, error) {
	if r == nil || r.state == nil || r.closed {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	return r.state.manager.RefGet(r.lemmaProof)
}

func (r *SMTConflictResolver) Close() error {
	if r == nil {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	if r.state == nil {
		return ast.ErrASTHandle
	}
	if r.closePhase == 0 {
		if err := r.state.manager.DelRef(&r.lemmaProof); err != nil {
			return err
		}
		r.closePhase = 1
	}
	r.expander = nil
	r.state = nil
	r.closed = true
	return nil
}

// UnsatCore implements mk_unsat_core's reverse assigned-literal walk.
// The source raw result can repeat an assumption and can include the failed
// consequent's polarity. The context maps and deduplicates that result.
func (r *SMTConflictResolver) UnsatCore(conflict SMTBooleanJustification, notLiteral SMTLiteral) ([]SMTLiteral, error) {
	if r == nil || r.state == nil || r.expander == nil || r.state.open() != nil {
		return nil, ast.ErrASTHandle
	}
	seen := make([]bool, len(r.state.booleanData))
	core := make([]SMTLiteral, 0)
	add := func(literal SMTLiteral) error {
		// Both constants use the reserved non-assumption variable zero.
		if literal == SMTTrueLiteral || literal == SMTFalseLiteral {
			return nil
		}
		variable := literal.Var()
		if variable < SMTFirstBoolVar || int(variable) >= len(seen) {
			return ast.ErrASTDomain
		}
		seen[variable] = true
		if r.state.booleanData[variable].Assumption {
			core = append(core, literal)
		}
		return nil
	}
	var expand func(SMTLiteral, SMTBooleanJustification) error
	expand = func(consequent SMTLiteral, justification SMTBooleanJustification) error {
		switch justification.Kind() {
		case SMTBooleanJustificationClause:
			clause, ok := justification.Clause()
			if !ok || clause == nil {
				return ast.ErrASTDomain
			}
			start := 0
			if consequent != SMTFalseLiteral {
				if clause.literals[0] == consequent {
					start = 1
				} else if len(clause.literals) > 1 && clause.literals[1] == consequent {
					if err := add(clause.literals[0].Negated()); err != nil {
						return err
					}
					start = 2
				} else {
					return ast.ErrASTDomain
				}
			}
			for _, literal := range clause.literals[start:] {
				if err := add(literal.Negated()); err != nil {
					return err
				}
			}
			if clause.justification != nil {
				antecedents, err := r.expander.ExpandJustification(clause.justification)
				if err != nil {
					return err
				}
				for _, literal := range antecedents {
					if err = add(literal); err != nil {
						return err
					}
				}
			}
		case SMTBooleanJustificationBinaryClause:
			literal, ok := justification.Literal()
			if !ok {
				return ast.ErrASTDomain
			}
			return add(literal)
		case SMTBooleanJustificationAxiom:
		case SMTBooleanJustificationObject:
			object, ok := justification.Justification()
			if !ok || object == nil {
				return ast.ErrASTDomain
			}
			antecedents, err := r.expander.ExpandJustification(object)
			if err != nil {
				return err
			}
			for _, literal := range antecedents {
				if err = add(literal); err != nil {
					return err
				}
			}
		default:
			return ast.ErrASTDomain
		}
		return nil
	}
	consequent := SMTFalseLiteral
	if notLiteral != SMTNullLiteral {
		consequent = notLiteral.Negated()
	}
	levels := r.levels()
	conflictLevel, err := SMTBooleanJustificationMaxLevel(r.expander, consequent, conflict, levels)
	if err != nil {
		return nil, err
	}
	searchLevel := r.state.scopeLevel
	if r.state.searching {
		searchLevel = r.state.searchBaseLevel
	}
	if conflictLevel > searchLevel {
		return nil, ast.ErrASTDomain
	}
	idx := len(r.state.assignedLiterals) - 1
	for idx >= 0 && levels[r.state.assignedLiterals[idx].Var()] > conflictLevel {
		idx--
	}
	if notLiteral != SMTNullLiteral {
		if err := add(consequent); err != nil {
			return nil, err
		}
	}
	if len(r.state.assignedLiterals) == 0 {
		return core, nil
	}
	reason := conflict
	for {
		if err := expand(consequent, reason); err != nil {
			return nil, err
		}
		if consequent.Var() >= SMTFirstBoolVar && r.state.booleanData[consequent.Var()].Assumption {
			core = append(core, consequent)
		}
		for idx >= 0 {
			literal := r.state.assignedLiterals[idx]
			if levels[literal.Var()] < searchLevel {
				return core, nil
			}
			if seen[literal.Var()] {
				break
			}
			idx--
		}
		if idx < 0 {
			return core, nil
		}
		consequent = r.state.assignedLiterals[idx]
		if levels[consequent.Var()] != searchLevel {
			return nil, ast.ErrASTDomain
		}
		reason = r.state.booleanData[consequent.Var()].Justification()
		idx--
	}
}

func (r *SMTConflictResolver) levels() []uint32 {
	if cap(r.levelsBuf) < len(r.state.booleanData) {
		r.levelsBuf = make([]uint32, len(r.state.booleanData), len(r.state.booleanData)*3/2)
	}
	levels := r.levelsBuf[:len(r.state.booleanData)]
	r.levelsBuf = levels
	for variable := int32(SMTFirstBoolVar); int(variable) < len(levels); variable++ {
		levels[variable] = r.state.booleanData[variable].ScopeLevel
	}
	return levels
}

func (r *SMTConflictResolver) Resolve(conflict SMTBooleanJustification, notLiteral SMTLiteral, baseLevel uint32) (SMTConflictResolution, error) {
	return r.resolve(conflict, notLiteral, baseLevel, baseLevel)
}

// The search level includes temporary assumptions, while the user base does
// not. Assumption-level antecedents must remain in learned clauses and cores.
func (r *SMTConflictResolver) resolve(conflict SMTBooleanJustification, notLiteral SMTLiteral, searchLevel, userBaseLevel uint32) (_ SMTConflictResolution, resolveErr error) {
	if r == nil || r.state == nil || r.expander == nil || r.closed || r.state.open() != nil {
		return SMTConflictResolution{}, ast.ErrASTHandle
	}
	// context::resolve_conflict counts each resolved conflict; setting a
	// conflict does not.
	r.state.statistics.NumConflicts++
	if err := r.state.manager.ResetRef(r.lemmaProof); err != nil {
		return SMTConflictResolution{}, err
	}
	originalConflict := conflict
	consequent := SMTFalseLiteral
	if notLiteral != SMTNullLiteral {
		consequent = notLiteral.Negated()
	}
	levels := r.levels()
	conflictLevel, err := SMTBooleanJustificationMaxLevel(r.expander, consequent, conflict, levels)
	if err != nil {
		return SMTConflictResolution{}, fmt.Errorf("conflict maximum level (%s, literal %d): %w", conflict.Kind(), notLiteral, err)
	}
	if err = r.state.dynamicAck.recordConflictUses(r.expander, originalConflict); err != nil {
		return SMTConflictResolution{}, err
	}
	result := SMTConflictResolution{ConflictLevel: conflictLevel}
	st := r.state
	if conflictLevel <= searchLevel {
		st.traceConflict("initialize_resolve", 403, func() string { return "problem is unsat\n" })
		proof, proofErr := r.conflictProof(originalConflict, notLiteral, nil)
		if proofErr != nil {
			return SMTConflictResolution{}, proofErr
		}
		if !proof.IsNull() {
			if err = r.state.manager.SetRef(r.lemmaProof, proof); err != nil {
				return SMTConflictResolution{}, err
			}
		}
		return result, nil
	}
	st.traceConflict("initialize_resolve", 411, func() string { return fmt.Sprintf("conflict_lvl: %d\n", conflictLevel) })
	marks := r.resetMarks()
	if err = r.expander.beginSession(); err != nil {
		return SMTConflictResolution{}, err
	}
	sessionOpen := true
	defer func() {
		if sessionOpen {
			resolveErr = errors.Join(resolveErr, r.expander.endSession())
		}
	}()
	lemma := []SMTLiteral{SMTNullLiteral}
	numMarks := 0
	processAntecedent := func(literal SMTLiteral) error {
		if literal == SMTTrueLiteral {
			return nil
		}
		if literal == SMTFalseLiteral {
			return ast.ErrASTDomain
		}
		variable := literal.Var()
		if variable < SMTFirstBoolVar || int(variable) >= len(marks) {
			return ast.ErrASTDomain
		}
		level := levels[variable]
		st.traceConflict("process_antecedent", 314, func() string {
			return fmt.Sprintf("processing antecedent (level %d):", level) + st.traceLiteralCompact(literal) + " " + st.traceLiteralVerbose(literal) + "\n"
		})
		if marks[variable] || level <= userBaseLevel {
			return nil
		}
		marks[variable] = true
		if err := r.state.incrementBooleanActivity(variable); err != nil {
			return err
		}
		if level == conflictLevel {
			st.traceConflict("process_antecedent", 335, func() string { return "marking:" + st.traceLiteralCompact(literal) + "\n" })
			numMarks++
		} else {
			lemma = append(lemma, literal.Negated())
		}
		return nil
	}
	var process func(SMTLiteral, SMTBooleanJustification) error
	process = func(consequent SMTLiteral, justification SMTBooleanJustification) error {
		switch justification.Kind() {
		case SMTBooleanJustificationClause:
			clause, ok := justification.Clause()
			if !ok || clause == nil {
				return ast.ErrASTDomain
			}
			st.traceConflict("resolve", 507, func() string { return st.traceClauseDetail(clause, levels) })
			if clause.IsLemma() {
				if err := clause.IncrementActivity(); err != nil {
					return err
				}
			}
			start := 0
			if consequent != SMTFalseLiteral {
				if clause.literals[0] == consequent {
					start = 1
				} else if len(clause.literals) > 1 && clause.literals[1] == consequent {
					if err := processAntecedent(clause.literals[0].Negated()); err != nil {
						return err
					}
					start = 2
				} else {
					return ast.ErrASTDomain
				}
			}
			for _, literal := range clause.literals[start:] {
				if err := processAntecedent(literal.Negated()); err != nil {
					return err
				}
			}
			if clause.justification != nil {
				antecedents, err := r.expander.expandInSession(clause.justification)
				if err != nil {
					return err
				}
				for _, literal := range antecedents {
					if err = processAntecedent(literal); err != nil {
						return err
					}
				}
			}
		case SMTBooleanJustificationBinaryClause:
			literal, ok := justification.Literal()
			if !ok {
				return ast.ErrASTDomain
			}
			return processAntecedent(literal)
		case SMTBooleanJustificationAxiom:
		case SMTBooleanJustificationObject:
			object, ok := justification.Justification()
			if !ok || object == nil {
				return ast.ErrASTDomain
			}
			antecedents, err := r.expander.expandInSession(object)
			if err != nil {
				return fmt.Errorf("expand %s: %w", object.Name(), err)
			}
			for _, literal := range antecedents {
				if err = processAntecedent(literal); err != nil {
					return fmt.Errorf("%s antecedent %d: %w", object.Name(), literal, err)
				}
			}
		default:
			return ast.ErrASTDomain
		}
		return nil
	}
	st.traceConflict("resolve", 479, func() string {
		// context::display(b_justification) ends its own line.
		return st.traceLiteralVerbose(notLiteral) + " " + st.traceJustification(conflict)
	})
	if notLiteral != SMTNullLiteral {
		st.traceConflict("resolve", 487, func() string { return "not_l: " + st.traceLiteralVerbose(notLiteral) + "\n" })
		if err = processAntecedent(notLiteral); err != nil {
			return SMTConflictResolution{}, err
		}
	}
	// skip_literals_above_conflict_level
	index := len(r.state.assignedLiterals) - 1
	for index >= 0 && levels[r.state.assignedLiterals[index].Var()] > conflictLevel {
		index--
	}
	for {
		st.traceConflict("resolve", 499, func() string {
			return fmt.Sprintf("processing consequent id: %d lit: %s ", index, traceLiteral(consequent)) +
				st.traceLiteralCompact(consequent) + st.traceLiteralVerbose(consequent) + "\n" +
				fmt.Sprintf("num_marks: %d, js kind: %s level: %d\n", numMarks, traceBJustificationKind(conflict.Kind()), levels[consequent.Var()])
		})
		if err = process(consequent, conflict); err != nil {
			return SMTConflictResolution{}, fmt.Errorf("resolve consequent %d (%s): %w", consequent, conflict.Kind(), err)
		}
		for index >= 0 && !marks[r.state.assignedLiterals[index].Var()] {
			index--
		}
		if index < 0 {
			return SMTConflictResolution{}, fmt.Errorf("no marked assignment at conflict level %d, remaining %d: %w", conflictLevel, numMarks, ast.ErrASTDomain)
		}
		consequent = r.state.assignedLiterals[index]
		variable := consequent.Var()
		conflict = r.state.booleanData[variable].Justification()
		marks[variable] = false
		index--
		numMarks--
		if numMarks == 0 {
			break
		}
	}
	st.traceConflict("resolve", 573, func() string { return "FUIP: " + st.traceLiteralCompact(consequent) + "\n" })
	lemma[0] = consequent.Negated()
	// finalize_resolve: unmark_justifications(0)
	sessionOpen = false
	if err = r.expander.endSession(); err != nil {
		return SMTConflictResolution{}, err
	}
	st.traceConflict("finalize_resolve", 432, func() string { return "before minimization:\n" + st.traceLiteralsCompact(lemma) + "\n" })
	lemma, err = r.minimizeLemma(lemma, marks, levels, userBaseLevel)
	if err != nil {
		return SMTConflictResolution{}, err
	}
	st.traceConflict("finalize_resolve", 439, func() string { return "after minimization:\n" + st.traceLiteralsCompact(lemma) + "\n" })
	result.Resolved, result.Lemma = true, lemma
	result.InternalScopeLevel = r.state.booleanData[lemma[0].Var()].InternalLevel()
	result.NewScopeLevel = searchLevel
	for _, literal := range lemma[1:] {
		variable := literal.Var()
		marks[variable] = false
		if levels[variable] > result.NewScopeLevel {
			result.NewScopeLevel = levels[variable]
		}
		if level := r.state.booleanData[variable].InternalLevel(); level > result.InternalScopeLevel {
			result.InternalScopeLevel = level
		}
	}
	st.traceConflict("finalize_resolve", 462, func() string {
		return fmt.Sprintf("new scope level:     %d\nintern. scope level: %d\n", result.NewScopeLevel, result.InternalScopeLevel)
	})
	proof, proofErr := r.conflictProof(originalConflict, notLiteral, lemma)
	if proofErr != nil {
		return SMTConflictResolution{}, proofErr
	}
	if !proof.IsNull() {
		if err = r.state.manager.SetRef(r.lemmaProof, proof); err != nil {
			return SMTConflictResolution{}, errors.Join(err, r.state.manager.ResetRef(r.lemmaProof))
		}
	}
	return result, nil
}
