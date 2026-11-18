package smtcore

// Source: src/smt/theory_str.cpp final_check_eh regex automata stage.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type smtStringRegexStage struct {
	theory *SMTStringTheory
	locals smtStringLengthPremises
	added  bool
}

func (f *smtStringRegexStage) pin(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return value, err
	}
	return value, f.locals.add(value)
}

// number is m_autil.mk_numeral(value, true).
func (f *smtStringRegexStage) number(value *numeral.Rational) (ast.ASTHandle, error) {
	return f.pin(f.theory.state.manager.ArithmeticNumeralFromRational(value, true))
}

// numberInt is m_autil.mk_numeral(rational(value), true).
func (f *smtStringRegexStage) numberInt(value int64) (ast.ASTHandle, error) {
	return f.pin(smtMkArithNumeralInt(f.theory.state.manager, value, true))
}

func (f *smtStringRegexStage) publish(expression ast.ASTHandle) error {
	if err := f.theory.assertAxiom(expression); err != nil {
		return err
	}
	f.added = true
	return nil
}

func (f *smtStringRegexStage) imply(guard, consequence ast.ASTHandle) error {
	p := smtStringLengthPremises{theory: f.theory, values: []ast.ASTHandle{guard}}
	if err := p.imply(consequence); err != nil {
		return err
	}
	f.added = true
	return nil
}

func (f *smtStringRegexStage) assignment(member ast.ASTHandle) (lbool.LBool, error) {
	variable, exists := f.theory.state.BoolVar(member)
	if !exists {
		return lbool.Undef, nil
	}
	return f.theory.state.Assignment(NewSMTLiteral(variable, false))
}

// stringRegexUnsignedLength is rational::get_unsigned for a length.
func stringRegexUnsignedLength(q *numeral.RationalManager, value *numeral.Rational) (uint32, error) {
	if value == nil || value.IsNeg() {
		return 0, ast.ErrASTDomain
	}
	ok, err := q.IsUnsigned(value)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ast.ErrASTDomain
	}
	return q.Unsigned(value)
}

func (f *smtStringRegexStage) registerLengths(member, word, regex ast.ASTHandle, assignment lbool.LBool) (result error) {
	t, m := f.theory, f.theory.state.manager
	if _, done := t.regexStage.lengthDone[member]; done || assignment != lbool.True {
		return nil
	}
	linear, err := t.checkRegexLengthLinearity(regex, false)
	if err != nil || !linear {
		return err
	}
	formula, cached := t.regexStage.lengthFormulas[member]
	variables := t.regexStage.lengthVariables[member]
	if !cached {
		length, err := f.pin(m.MkSequenceLength(word))
		if err != nil {
			return err
		}
		root, err := t.inferStringRegexLengths(length, regex, &variables)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, m.DelRef(&root)) }()
		formula, err = m.RefGet(root)
		if err != nil {
			return err
		}
		// expr_ref top_level_length_constraint(_top_level_length_constraint, m)
		// is rewritten; the original stays alive to the end of the block,
		// after the th_rewriter is destroyed.
		rewritten, err := m.NewRef(formula)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, m.DelRef(&rewritten)) }()
		if t.state.theoryRewriter == nil {
			return ast.ErrASTHandle
		}
		rw, err := t.state.theoryRewriter.FreshDefault()
		if rw != nil {
			defer func() { result = errors.Join(result, rw.Close()) }()
		}
		if err != nil {
			return err
		}
		if err = rw.Rewrite(formula, rewritten); err != nil {
			return err
		}
		formula, err = m.RefGet(rewritten)
		if err != nil {
			return err
		}
		if err = f.locals.add(formula); err != nil {
			return err
		}
	}
	if err = f.publish(formula); err != nil {
		return err
	}
	for _, variable := range variables {
		if cached {
			if err = t.refreshStringVariable(variable); err != nil {
				return err
			}
		}
		zero, err := f.numberInt(0)
		if err != nil {
			return err
		}
		bound, err := f.pin(m.MkArithmeticGE(variable, zero))
		if err != nil {
			return err
		}
		if err = f.publish(bound); err != nil {
			return err
		}
	}
	if !cached {
		t.regexStage.lengthFormulas[member] = formula
		t.regexStage.lengthVariables[member] = append([]ast.ASTHandle(nil), variables...)
	}
	t.regexStage.lengthDone[member] = t.level
	return nil
}

func (f *smtStringRegexStage) construct(member, regex ast.ASTHandle, options SMTStrState, readFailureFirst bool) error {
	t := f.theory
	complexity, err := t.estimateStringRegexComplexity(regex, false)
	if err != nil {
		return err
	}
	allowed := complexity <= options.RegexAutomataDifficultyThreshold
	if readFailureFirst {
		failures := stringRegexCounter(t.regexStage.failures, member)
		allowed = allowed || failures >= options.RegexAutomataFailedAutomatonThreshold
	} else if !allowed {
		allowed = stringRegexCounter(t.regexStage.failures, member) >= options.RegexAutomataFailedAutomatonThreshold
	}
	if !allowed {
		incrementStringRegexCounter(t.regexStage.failures, member)
		return nil
	}
	owner, err := rewriter.NewStringRegexAutomaton(t.state.manager, regex, t.regexExpressionSolver)
	if err != nil {
		return err
	}
	graph, err := owner.Graph()
	if err != nil {
		return errors.Join(err, owner.Close())
	}
	graph.Compress()
	t.regexStage.owned = append(t.regexStage.owned, owner)
	t.regexStage.automata[regex] = owner
	f.added = true
	return t.findStringAutomatonInitialBounds(member, graph)
}

func (f *smtStringRegexStage) exactGuard(member, word ast.ASTHandle, assignment lbool.LBool, value *numeral.Rational) (ast.ASTHandle, error) {
	m := f.theory.state.manager
	polarity := member
	var err error
	if assignment == lbool.False {
		polarity, err = f.pin(m.MkNot(member))
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	length, err := f.pin(m.MkSequenceLength(word))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	number, err := f.number(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	equal, err := f.pin(f.theory.state.MkEqAtom(length, number))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return f.pin(m.MkAndArray([]ast.ASTHandle{polarity, equal}))
}

func (f *smtStringRegexStage) exactPath(member, word ast.ASTHandle, assignment lbool.LBool, owner *rewriter.StringRegexAutomaton, value *numeral.Rational) (result error) {
	t, m := f.theory, f.theory.state.manager
	if value.Sign() == 0 {
		graph, err := owner.Graph()
		if err != nil {
			return err
		}
		accepts := stringAutomatonFrontierAccepts(graph, graph.EpsilonClosure(graph.Initial()))
		if assignment == lbool.False {
			accepts = !accepts
		} // All retained cached automata have positive polarity.
		guard, err := f.exactGuard(member, word, assignment, value)
		if err != nil {
			return err
		}
		if !accepts {
			conflict, err := f.pin(m.MkNot(guard))
			if err != nil {
				return err
			}
			return f.publish(conflict)
		}
		empty, err := f.pin(t.stringLiteralChars(""))
		if err != nil {
			return err
		}
		rhs, err := f.pin(f.theory.state.MkEqAtom(word, empty))
		if err != nil {
			return err
		}
		if err = f.imply(guard, rhs); err != nil {
			return err
		}
		t.regexStage.pathDone[member] = t.level
		return nil
	}
	unsigned, err := stringRegexUnsignedLength(f.theory.q(), value)
	if err != nil {
		return err
	}
	characters, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelRef(&characters)) }()
	path, err := t.generateStringRegexPathConstraints(word, owner, unsigned, characters)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelRef(&path)) }()
	pathValue, err := m.RefGet(path)
	if err != nil {
		return err
	}
	characterValue, err := m.RefGet(characters)
	if err != nil {
		return err
	}
	guard, err := f.exactGuard(member, word, assignment, value)
	if err != nil {
		return err
	}
	if m.IsFalse(pathValue) {
		if assignment == lbool.True {
			conflict, err := f.pin(m.MkNot(guard))
			if err != nil {
				return err
			}
			return f.publish(conflict)
		}
		if err = f.imply(guard, characterValue); err != nil {
			return err
		}
	} else {
		if assignment == lbool.False {
			pathValue, err = f.pin(m.MkNot(pathValue))
			if err != nil {
				return err
			}
		}
		rhs, err := f.pin(m.MkAnd(pathValue, characterValue))
		if err != nil {
			return err
		}
		if err = f.imply(guard, rhs); err != nil {
			return err
		}
	}
	t.regexStage.pathDone[member] = t.level
	return nil
}

func (f *smtStringRegexStage) refineBound(member, length ast.ASTHandle, owner *rewriter.StringRegexAutomaton, value *numeral.Rational, upper bool) error {
	m := f.theory.state.manager
	unsigned, err := stringRegexUnsignedLength(f.theory.q(), value)
	if err != nil {
		return err
	}
	graph, err := owner.Graph()
	if err != nil {
		return err
	}
	var at bool
	var next int64
	if upper {
		at, next, err = refineStringAutomatonUpperBound(graph, unsigned)
	} else {
		at, next, err = refineStringAutomatonLowerBound(graph, unsigned)
	}
	if err != nil {
		return err
	}
	number, err := f.number(value)
	if err != nil {
		return err
	}
	var bound ast.ASTHandle
	if upper {
		bound, err = f.pin(m.MkArithmeticLE(length, number))
	} else {
		bound, err = f.pin(m.MkArithmeticGE(length, number))
	}
	if err != nil {
		return err
	}
	var rhs ast.ASTHandle
	if at {
		if next == -1 {
			rhs, err = f.pin(f.theory.state.MkEqAtom(length, number))
		} else {
			// The lower-bound equality-or-next case is explicitly disabled in
			// native code because it causes arithmetic nontermination.
			if !upper {
				return nil
			}
			previous, createErr := f.numberInt(next)
			if createErr != nil {
				return createErr
			}
			below, createErr := f.pin(m.MkArithmeticLE(length, previous))
			if createErr != nil {
				return createErr
			}
			exact, createErr := f.pin(f.theory.state.MkEqAtom(length, number))
			if createErr != nil {
				return createErr
			}
			rhs, err = f.pin(m.MkOr(exact, below))
		}
	} else if next == -1 {
		rhs, err = f.pin(m.MkNot(bound))
	} else {
		refined, createErr := f.numberInt(next)
		if createErr != nil {
			return createErr
		}
		if upper {
			rhs, err = f.pin(m.MkArithmeticLE(length, refined))
		} else {
			rhs, err = f.pin(m.MkArithmeticGE(length, refined))
		}
	}
	if err != nil {
		return err
	}
	// mk_and(lhs) is built only at assert_implication, after the rhs atoms.
	guard, err := f.pin(m.MkAndArray([]ast.ASTHandle{member, bound}))
	if err != nil {
		return err
	}
	return f.imply(guard, rhs)
}

func (t *SMTStringTheory) runStringRegexAutomataStage() (added bool, result error) {
	enabled, err := t.regexAutomataEnabled()
	if err != nil || !enabled {
		return false, err
	}
	options := SMTStrState{RegexAutomataDifficultyThreshold: 1000, RegexAutomataIntersectionDifficultyThreshold: 1000, RegexAutomataFailedAutomatonThreshold: 10, RegexAutomataFailedIntersectionThreshold: 10, RegexAutomataLengthAttemptThreshold: 10}
	if t.parameterOwner != nil {
		options, err = t.stringParameterState()
		if err != nil {
			return false, err
		}
	}
	f := smtStringRegexStage{theory: t, locals: smtStringLengthPremises{theory: t}}
	defer func() { added = f.added; result = errors.Join(result, f.locals.close()) }()
	m := t.state.manager
	var terms []ast.ASTHandle
	for term := range t.regexTerms {
		terms = append(terms, term)
	}
	stringSortHandles(t, terms)
	q := t.q()
	value, lower, upper := smtQZero(), smtQZero(), smtQZero()
	defer t.qdel(&upper, &result)
	defer t.qdel(&lower, &result)
	defer t.qdel(&value, &result)
	for _, member := range terms {
		word, regex, matched, err := m.SequenceBinaryOperationArguments(member, 12)
		if err != nil {
			return false, err
		}
		if !matched {
			return false, ast.ErrASTDomain
		}
		assignment, err := f.assignment(member)
		if err != nil {
			return false, err
		}
		if assignment == lbool.Undef {
			continue
		}
		if err = f.registerLengths(member, word, regex, assignment); err != nil {
			return false, err
		}
		// rational exact_len_value, lower_bound_value, upper_bound_value
		// (the source declares them per membership; these are reset by
		// their readers each iteration).
		exact, err := t.lengthValue(word, &value)
		if err != nil {
			return false, err
		}
		owner := t.regexStage.automata[regex]
		if exact {
			if _, done := t.regexStage.pathDone[member]; done {
				continue
			}
			if owner != nil {
				if err = f.exactPath(member, word, assignment, owner, &value); err != nil {
					return false, err
				}
				incrementStringRegexCounter(t.regexStage.lengthAttempts, regex)
			} else if err = f.construct(member, regex, options, false); err != nil {
				return false, err
			}
			continue
		}
		length, err := f.pin(m.MkSequenceLength(word))
		if err != nil {
			return false, err
		}
		hasLower, err := t.stringArithmeticBound(length, true, &lower)
		if err != nil {
			return false, err
		}
		hasUpper, err := t.stringArithmeticBound(length, false, &upper)
		if err != nil {
			return false, err
		}
		if !hasLower {
			if err = q.Reset(&lower); err != nil {
				return false, err
			}
		}
		if !hasUpper {
			if err = q.Reset(&upper); err != nil {
				return false, err
			}
		} // Native unknown rationals still default to zero.
		lastLower, seenLower := t.regexLastLower[word]
		newLower := !seenLower
		if seenLower {
			eq, err := q.Equal(lastLower, &lower)
			if err != nil {
				return false, err
			}
			newLower = !eq
		}
		lastUpper, seenUpper := t.regexStage.lastUpper[word]
		newUpper := !seenUpper
		if seenUpper {
			eq, err := q.Equal(lastUpper, &upper)
			if err != nil {
				return false, err
			}
			newUpper = !eq
		}
		if newLower {
			if err = t.setRegexBound(t.regexLastLower, word, &lower); err != nil {
				return false, err
			}
		}
		if newUpper {
			if err = t.setRegexBound(t.regexStage.lastUpper, word, &upper); err != nil {
				return false, err
			}
		}
		if hasUpper && newUpper {
			if owner == nil {
				if err = f.construct(member, regex, options, true); err != nil {
					return false, err
				}
				continue
			}
			if assignment == lbool.True {
				if err = f.refineBound(member, length, owner, &upper, true); err != nil {
					return false, err
				}
			}
		} else if hasLower && !lower.IsZero() && newLower {
			if owner == nil {
				if err = f.construct(member, regex, options, true); err != nil {
					return false, err
				}
				continue
			}
			if assignment == lbool.True {
				if err = f.refineBound(member, length, owner, &lower, false); err != nil {
					return false, err
				}
			}
		} else {
			// This read happens even when an existing automaton makes the
			// following construction unnecessary; reads insert missing zero.
			stringRegexCounter(t.regexStage.failures, member)
			if owner == nil {
				if err = f.construct(member, regex, options, true); err != nil {
					return false, err
				}
			} else {
				incrementStringRegexCounter(t.regexStage.failures, member)
			}
		}
	}
	if err = f.intersections(options); err != nil {
		return false, err
	}
	// Native regex_axiom_add's FC_CONTINUE return is commented out. The
	// enclosing final checker must continue into its remaining stages.
	return f.added, nil
}

// setRegexBound is regex_last_lower_bound/upper_bound.insert(str, value):
// the map's rational is a copy; an overwritten one is freed.
func (t *SMTStringTheory) setRegexBound(bounds map[ast.ASTHandle]*numeral.Rational, word ast.ASTHandle, value *numeral.Rational) error {
	r := bounds[word]
	if r == nil {
		fresh := smtQZero()
		r = &fresh
		bounds[word] = r
	}
	return t.q().Copy(r, value)
}
