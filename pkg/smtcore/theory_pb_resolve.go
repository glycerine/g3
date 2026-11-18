package smtcore

// Source: theory_pb process_antecedent/process_ineq/resolve_conflict and
// hoist_maximal_values/remove_from_lemma. All state belongs to one resolution.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type smtPBResolution struct {
	theory      *SMTPseudoBooleanTheory
	base, level uint32
	marks       map[int32]int
	numMarks    uint32
	lemma       smtPBArgs
	guards      []SMTLiteral
}

// remove is remove_from_lemma: m_lemma[idx] = m_lemma[last] copies, then
// pop_back destroys the last coefficient.
func (r *smtPBResolution) remove(index int) error {
	q := r.theory.q()
	v := r.lemma.terms[index].literal.Var()
	last := len(r.lemma.terms) - 1
	if index != last {
		r.lemma.terms[index].literal = r.lemma.terms[last].literal
		if err := q.Copy(&r.lemma.terms[index].coeff, &r.lemma.terms[last].coeff); err != nil {
			return err
		}
		r.marks[r.lemma.terms[index].literal.Var()] = index
	}
	if err := r.lemma.popBack(q); err != nil {
		return err
	}
	delete(r.marks, v)
	r.numMarks--
	return nil
}

// antecedent is process_antecedent(l, coeff); coeff is the caller's value.
func (r *smtPBResolution) antecedent(literal SMTLiteral, coefficient *numeral.Rational) error {
	s := r.theory.state
	q := r.theory.q()
	value, err := s.Assignment(literal)
	if err != nil {
		return err
	}
	v := literal.Var()
	level := s.booleanData[v].ScopeLevel
	if value != lbool.False {
		if err = q.Sub(&r.lemma.bound, coefficient, &r.lemma.bound); err != nil {
			return err
		}
		if index, marked := r.marks[v]; marked && r.theory.controls.LearnComplements {
			if smtcoreSourceDebug && value != lbool.True {
				return ast.ErrASTDomain
			}
			c := &r.lemma.terms[index].coeff
			if err = q.Sub(c, coefficient, c); err != nil {
				return err
			}
			if c.Sign() <= 0 {
				if err = r.remove(index); err != nil {
					return err
				}
			}
		}
	} else if level > r.base {
		if index, marked := r.marks[v]; marked {
			if err = q.Add(&r.lemma.terms[index].coeff, coefficient, &r.lemma.terms[index].coeff); err != nil {
				return err
			}
			if smtcoreSourceDebug && r.lemma.terms[index].coeff.Sign() <= 0 {
				return ast.ErrASTDomain
			}
		} else {
			if level == r.level {
				if err := r.theory.emitPB("process_antecedent", 1529, func(out io.Writer) error {
					_, _ = io.WriteString(out, "add mark: ")
					if err := smtPBWriteLiteral(out, literal); err != nil {
						return err
					}
					text, err := q.String(coefficient)
					if err != nil {
						return err
					}
					_, err = fmt.Fprintf(out, " %s\n", text)
					return err
				}); err != nil {
					return err
				}
				r.numMarks++
			}
			r.marks[v] = len(r.lemma.terms)
			if err = r.lemma.pushBack(q, literal, coefficient); err != nil {
				return err
			}
		}
		if err := r.theory.emitPBTag("pb_verbose", "process_antecedent", 1535, func(out io.Writer) error {
			term := r.lemma.terms[r.marks[v]]
			_, _ = io.WriteString(out, "ante: ")
			if err := smtPBWriteLiteral(out, term.literal); err != nil {
				return err
			}
			text, err := q.String(&term.coeff)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(out, "*%s %d\n", text, level)
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}

// inequality is process_ineq(c, conseq, coeff1).
func (r *smtPBResolution) inequality(c *smtPBConstraint, consequent SMTLiteral, coefficient *numeral.Rational) (err error) {
	q := r.theory.q()
	// numeral coeff2, lc, g and the g*c.coeff(i) argument temporaries.
	other, lc, g, product := smtQZero(), smtQZero(), smtQZero(), smtQZero()
	defer func() {
		for _, x := range []*numeral.Rational{&product, &g, &lc, &other} {
			err = smtErrJoin(err, smtQDel(q, x))
		}
	}()
	if consequent == SMTNullLiteral {
		if err = q.SetInt64(&other, 1); err != nil {
			return err
		}
	} else {
		for i := range c.current().terms {
			if c.current().terms[i].literal == consequent {
				if err = q.Copy(&other, &c.current().terms[i].coeff); err != nil {
					return err
				}
				break
			}
		}
	}
	if other.Sign() <= 0 || coefficient.Sign() <= 0 || !other.IsInt() || !coefficient.IsInt() {
		return ast.ErrASTDomain
	}
	if err = q.LCM(coefficient, &other, &lc); err != nil {
		return err
	}
	if err = q.Div(&lc, coefficient, &g); err != nil {
		return err
	}
	if g.Sign() > 0 && !g.IsOne() {
		for i := range r.lemma.terms {
			if err = q.Mul(&r.lemma.terms[i].coeff, &g, &r.lemma.terms[i].coeff); err != nil {
				return err
			}
		}
		if err = q.Mul(&r.lemma.bound, &g, &r.lemma.bound); err != nil {
			return err
		}
	}
	if err = q.Div(&lc, &other, &g); err != nil {
		return err
	}
	if err = q.AddMul(&r.lemma.bound, &g, &c.current().bound, &r.lemma.bound); err != nil {
		return err
	}
	for i := range c.current().terms {
		term := &c.current().terms[i]
		if err = q.Mul(&g, &term.coeff, &product); err != nil {
			return err
		}
		if err = r.antecedent(term.literal, &product); err != nil {
			return err
		}
	}
	if smtcoreSourceDebug {
		value, err := r.theory.state.Assignment(c.lit())
		if err != nil {
			return err
		}
		if value != lbool.True {
			return ast.ErrASTDomain
		}
	}
	if r.theory.state.booleanData[c.lit().Var()].ScopeLevel > r.base {
		r.guards = append(r.guards, c.lit())
	}
	return nil
}

// hoist is hoist_maximal_values.
func (r *smtPBResolution) hoist() error {
	q := r.theory.q()
	for i := 0; i < len(r.lemma.terms); {
		cmp, err := q.Compare(&r.lemma.terms[i].coeff, &r.lemma.bound)
		if err != nil {
			return err
		}
		if cmp >= 0 {
			r.guards = append(r.guards, r.lemma.terms[i].literal.Negated())
			last := len(r.lemma.terms) - 1
			r.lemma.terms[i], r.lemma.terms[last] = r.lemma.terms[last], r.lemma.terms[i]
			if err = r.lemma.popBack(q); err != nil {
				return err
			}
		} else {
			i++
		}
	}
	return nil
}

func (t *SMTPseudoBooleanTheory) resolutionExpression(a *smtPBArgs) (expression ast.ASTHandle, result error) {
	s := t.state
	service := s.manager.NumeralService()
	var values []numeral.OwnedRational
	defer func() {
		for i := range values {
			result = smtErrJoin(result, service.Del(&values[i]))
		}
	}()
	makeValue := func(q *numeral.Rational) (numeral.OwnedRational, error) {
		v, err := service.NewValue()
		if err != nil {
			return v, err
		}
		values = append(values, v)
		err = service.Manager.Copy(&values[len(values)-1].Raw, q)
		return values[len(values)-1], err
	}
	var coefficients []numeral.OwnedRational
	var arguments []ast.ASTHandle
	for i := range a.terms {
		term := &a.terms[i]
		q, err := makeValue(&term.coeff)
		if err != nil {
			return expression, err
		}
		coefficients = append(coefficients, q)
		h, err := s.BoolExpression(term.literal.Var())
		if err != nil {
			return expression, err
		}
		if term.literal.Sign() {
			h, err = s.manager.MkNot(h)
			if err != nil {
				return expression, err
			}
		}
		arguments = append(arguments, h)
	}
	k, err := makeValue(&a.bound)
	if err != nil {
		return expression, err
	}
	return s.manager.MkPseudoBooleanGE(coefficients, arguments, &k)
}

func (t *SMTPseudoBooleanTheory) resolutionClause(literals []SMTLiteral) error {
	var owner SMTJustification
	enabled, err := t.state.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	if enabled {
		// FORK FIX (z3 4.7.1 bug; upstream removed this code in its pb rewrite,
		// first in 4.8.5): source justify() region-allocated a
		// theory_axiom_justification that the CLS_AUX_LEMMA clause later
		// dealloc'ed from the heap ("free(): invalid pointer" at exit). The
		// fixed fork heap-allocates a theory_lemma_justification, like the
		// other pb lemma sites (add_clause, the simplex conflict).
		owner, err = NewSMTTheoryLemmaJustification(t.state.manager, t.FamilyID(), literals, t.state.boolExpr, nil)
		if err != nil {
			return err
		}
	}
	return t.state.addClauseKindWithJustificationAndDeleteHandler(literals, false, SMTClauseAuxLemma, ast.ASTHandle{}, owner, nil)
}

func (t *SMTPseudoBooleanTheory) resolveConflict(c *smtPBConstraint) (_ bool, err error) {
	if c.equality {
		return false, nil
	}
	if err := t.traceConstraint("resolve_conflict", 1595, c, true); err != nil {
		return false, err
	}
	s := t.state
	base := s.scopeLevel
	if s.searching {
		base = s.searchUserBaseLevel
	}
	// m_lemma.reset(); m_lemma.m_k.reset(): the lemma's rationals die when
	// resolve_conflict's work is done (the source reuses its member).
	r := smtPBResolution{theory: t, base: base, marks: make(map[int32]int), lemma: newSMTPBArgs()}
	q := t.q()
	defer func() {
		if e := r.lemma.del(q); e != nil && err == nil {
			err = e
		}
	}()
	for _, term := range c.current().terms {
		value, err := s.Assignment(term.literal)
		if err != nil {
			return false, err
		}
		if value == lbool.False && s.booleanData[term.literal.Var()].ScopeLevel > r.level {
			r.level = s.booleanData[term.literal.Var()].ScopeLevel
		}
	}
	if r.level < s.booleanData[c.lit().Var()].ScopeLevel || r.level == base {
		return false, nil
	}
	one := smtQInt(1)
	if err := r.inequality(c, SMTNullLiteral, &one); err != nil {
		return false, err
	}
	index := len(s.assignedLiterals) - 1
	if smtcoreSourceDebug && index < 0 {
		return false, ast.ErrASTDomain
	}
	for r.numMarks > 0 {
		if err := r.traceLemma("pb_verbose", 1624, "lemma ", false); err != nil {
			return false, err
		}
		truth, err := r.lemma.normalize(q, false)
		if err != nil {
			return false, err
		}
		if truth == lbool.False {
			break
		}
		if truth == lbool.True {
			if err := smtPBVerbose(0, func(out io.Writer) error { _, err := io.WriteString(out, "lemma already evaluated\n"); return err }); err != nil {
				return false, err
			}
			if err := t.emitPB("resolve_conflict", 1632, func(out io.Writer) error { _, err := io.WriteString(out, "lemma already evaluated\n"); return err }); err != nil {
				return false, err
			}
			return false, nil
		}
		if err := r.traceLemma("pb", 1635, "", false); err != nil {
			return false, err
		}
		if err := r.lemma.validateWellFormed(t.q()); err != nil {
			return false, err
		}
		var consequent SMTLiteral
		position := -1
		for index >= 0 {
			consequent = s.assignedLiterals[index]
			index--
			if p, marked := r.marks[consequent.Var()]; marked {
				position = p
				break
			}
		}
		// Source refuses an asynchronous reason whose marked assignment
		// cannot be found earlier in the trail. Never index past the trail.
		if position < 0 {
			if err := smtPBVerbose(12, r.displayResolvedLemma); err != nil {
				return false, err
			}
			if err := t.emitPB("resolve_conflict", 1657, r.displayResolvedLemma); err != nil {
				return false, err
			}
			return false, nil
		}
		if err := t.emitPB("resolve_conflict", 1664, func(out io.Writer) error {
			if err := t.displayPBArgs(out, &r.lemma, true); err != nil {
				return err
			}
			_, _ = io.WriteString(out, "conseq: ")
			if err := smtPBWriteLiteral(out, consequent); err != nil {
				return err
			}
			_, err := fmt.Fprintf(out, " at index: %d\n", position)
			return err
		}); err != nil {
			return false, err
		}
		if r.lemma.terms[position].literal != consequent.Negated() {
			return false, ast.ErrASTDomain
		}
		// numeral conseq_coeff = m_lemma.coeff(conseq_index), destroyed at the
		// end of the iteration.
		coefficient := smtQZero()
		if err := q.Copy(&coefficient, &r.lemma.terms[position].coeff); err != nil {
			return false, err
		}
		done, err := t.resolveConsequent(&r, consequent, position, &coefficient)
		if e := smtQDel(q, &coefficient); e != nil && err == nil {
			err = e
		}
		if err != nil || done {
			return false, err
		}
	}
	return t.resolveConflictTail(c, &r)
}

// resolveConsequent resolves the lemma's consequent with its antecedents
// (resolve_conflict's loop body after conseq_coeff is taken). done reports
// a return of false from resolve_conflict.
func (t *SMTPseudoBooleanTheory) resolveConsequent(r *smtPBResolution, consequent SMTLiteral, position int, coefficient *numeral.Rational) (done bool, err error) {
	s := t.state
	base := r.base
	{
		if err := r.remove(position); err != nil {
			return false, err
		}
		reason := s.booleanData[consequent.Var()].Justification()
		switch reason.Kind() {
		case SMTBooleanJustificationClause:
			clause, ok := reason.Clause()
			if !ok || clause == nil {
				return false, ast.ErrASTDomain
			}
			if owner := clause.Justification(); owner != nil {
				if _, proof := owner.(*SMTJustificationProofWrapper); !proof {
					if err := t.emitPB("resolve_conflict", 1683, func(out io.Writer) error {
						_, _ = io.WriteString(out, "skipping justification for clause over: ")
						if err := smtPBWriteLiteral(out, consequent); err != nil {
							return err
						}
						_, err := fmt.Fprintf(out, " %s\n", smtPBJustificationTraceName(owner))
						return err
					}); err != nil {
						return false, err
					}
					r.guards = append(r.guards, consequent)
					return false, nil
				}
			}
			if len(clause.literals) < 2 {
				return false, ast.ErrASTDomain
			}
			start := 1
			if clause.literals[0] != consequent {
				if clause.literals[1] != consequent {
					return false, ast.ErrASTDomain
				}
				if err := r.antecedent(clause.literals[0], coefficient); err != nil {
					return false, err
				}
				start = 2
			}
			for _, literal := range clause.literals[start:] {
				if err := r.antecedent(literal, coefficient); err != nil {
					return false, err
				}
			}
			if err := t.emitPB("resolve_conflict", 1699, func(out io.Writer) error {
				if err := smtPBWriteLiterals(out, clause.literals, true); err != nil {
					return err
				}
				_, err := io.WriteString(out, "\n")
				return err
			}); err != nil {
				return false, err
			}
		case SMTBooleanJustificationBinaryClause:
			literal, ok := reason.Literal()
			if !ok {
				return false, ast.ErrASTDomain
			}
			if err := r.antecedent(literal.Negated(), coefficient); err != nil {
				return false, err
			}
			if err := t.traceAssignment("resolve_conflict", 1704, "binary: ", literal, nil, false); err != nil {
				return false, err
			}
		case SMTBooleanJustificationAxiom:
			if s.booleanData[consequent.Var()].ScopeLevel > base {
				r.guards = append(r.guards, consequent)
			}
			if err := t.traceAssignment("resolve_conflict", 1710, "axiom ", consequent, nil, false); err != nil {
				return false, err
			}
		case SMTBooleanJustificationObject:
			owner, ok := reason.Justification()
			if !ok || owner == nil {
				return false, ast.ErrASTDomain
			}
			pb, owned := owner.(*smtPBJustification)
			if !consequent.Sign() && owner.FromTheory() == t.FamilyID() && owned && pb.constraint != nil && !pb.constraint.equality && pb.constraint.lit() != consequent {
				if err := t.emitPB("resolve_conflict", 1729, func(out io.Writer) error {
					_, _ = io.WriteString(out, "resolve with inequality")
					return t.displayInequality(out, pb.constraint, true)
				}); err != nil {
					return false, err
				}
				if err := r.inequality(pb.constraint, consequent, coefficient); err != nil {
					return false, err
				}
			} else {
				if err := t.emitPB("resolve_conflict", 1733, func(out io.Writer) error {
					_, _ = io.WriteString(out, "skipping justification for ")
					if err := smtPBWriteLiteral(out, consequent); err != nil {
						return err
					}
					_, err := fmt.Fprintf(out, " from theory %d %s\n", owner.FromTheory(), smtPBJustificationTraceName(owner))
					return err
				}); err != nil {
					return false, err
				}
				r.guards = append(r.guards, consequent)
			}
		default:
			return false, ast.ErrASTDomain
		}
	}
	return false, nil
}

// resolveConflictTail is resolve_conflict after the resolution loop.
func (t *SMTPseudoBooleanTheory) resolveConflictTail(c *smtPBConstraint, r *smtPBResolution) (bool, error) {
	s := t.state
	q := t.q()
	if err := t.emitPB("resolve_conflict", 1745, func(out io.Writer) error {
		if err := smtPBWriteLiterals(out, r.guards, true); err != nil {
			return err
		}
		_, _ = io.WriteString(out, "=> ")
		return t.displayPBArgs(out, &r.lemma, false)
	}); err != nil {
		return false, err
	}
	if err := smtPBVerbose(14, func(out io.Writer) error {
		_, _ = io.WriteString(out, "lemma1: ")
		return t.displayPBArgs(out, &r.lemma, false)
	}); err != nil {
		return false, err
	}
	if err := r.hoist(); err != nil {
		return false, err
	}
	truth, err := r.lemma.normalize(q, false)
	if err != nil {
		return false, err
	}
	if err = r.lemma.prune(q, false); err != nil {
		return false, err
	}
	if err := smtPBVerbose(14, func(out io.Writer) error {
		_, _ = io.WriteString(out, "lemma2: ")
		return t.displayPBArgs(out, &r.lemma, false)
	}); err != nil {
		return false, err
	}
	if err := smtPBVerbose(1, func(out io.Writer) error {
		_, err := fmt.Fprintf(out, "(pb.conflict %d %d)\n", len(r.guards), len(r.lemma.terms))
		return err
	}); err != nil {
		return false, err
	}
	switch truth {
	case lbool.True:
		if smtcoreSourceDebug {
			return false, ast.ErrASTDomain
		}
		return false, nil
	case lbool.False:
		t.incPropagations(c)
		t.statistics.conflicts++
		for i := range r.guards {
			r.guards[i] = r.guards[i].Negated()
		}
		if err := t.traceClause("resolve_conflict", 1776, r.guards); err != nil {
			return false, err
		}
		return true, t.resolutionClause(r.guards)
	default:
		expression, err := t.resolutionExpression(&r.lemma)
		if err != nil {
			return false, err
		}
		if _, err = t.internalizeAtom(expression, true); err != nil {
			return false, err
		}
		if err = s.markRelevant(expression); err != nil {
			return false, err
		}
		literal, err := s.Literal(expression)
		if err != nil {
			return false, err
		}
		return true, t.addAssign(c, r.guards, literal)
	}
}
