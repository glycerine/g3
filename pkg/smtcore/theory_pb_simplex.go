package smtcore

// Source: src/smt/theory_pb.cpp:84-92,252-415,543-600,788-839,1333-1342.

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// smtPBSimplexRow is row_info; m_bound is a rational of the AST manager's
// rational manager, freed when the row info is erased.
type smtPBSimplexRow struct {
	slack          uint32
	bound          numeral.Rational
	representative string
}

type smtPBSimplex struct {
	solver          *simplex.Solver
	rows            map[int32]smtPBSimplexRow
	representatives map[string]int32
	variables       map[uint32]bool
	lower, upper    map[uint32]SMTLiteral
}

// Simplex columns are Boolean IDs, as in the source: a variable's 0-1
// column and a predicate's row slack column share the predicate's ID
// (theory_pb.cpp:569,592). Columns of IDs freed by pop keep their values,
// which steers later pivots. Two fork fixes keep the sharing sound:
// constraints over PB predicates get no row (simplexInternalize), and only
// 0-1 columns receive literal bounds (assignNative).

func smtPBExplain(explains map[uint32]SMTLiteral, v uint32) SMTLiteral {
	if l, found := explains[v]; found {
		return l
	}
	return SMTNullLiteral
}

// simplexUpdateBound is update_bound(v, explain, is_lower, bound); bound is
// borrowed (a value of the AST manager's rational manager). The replaced
// bound is kept for the trail's undo, which restores and frees it.
func (t *SMTPseudoBooleanTheory) simplexUpdateBound(v uint32, explain SMTLiteral, lower bool, bound *simplex.Infinitesimal) (result bool, err error) {
	s := t.simplex
	q := t.q()
	old, opposite := simplex.ZeroInfinitesimal(), simplex.ZeroInfinitesimal()
	keepOld := false
	defer func() {
		err = smtErrJoin(err, opposite.Del(q))
		if !keepOld {
			err = smtErrJoin(err, old.Del(q))
		}
	}()
	var valid, oppositeValid bool
	explains, oppositeExplains := s.lower, s.upper
	if lower {
		if valid, err = s.solver.Lower(v, &old); err != nil {
			return false, err
		}
		if oppositeValid, err = s.solver.Upper(v, &opposite); err != nil {
			return false, err
		}
		if valid {
			if c, err := bound.Cmp(q, &old); err != nil || c <= 0 {
				return err == nil, err
			}
		}
	} else {
		if valid, err = s.solver.Upper(v, &old); err != nil {
			return false, err
		}
		if oppositeValid, err = s.solver.Lower(v, &opposite); err != nil {
			return false, err
		}
		explains, oppositeExplains = s.upper, s.lower
		if valid {
			if c, err := bound.Cmp(q, &old); err != nil || c >= 0 {
				return err == nil, err
			}
		}
	}
	if oppositeValid {
		c, err := bound.Cmp(q, &opposite)
		if err != nil {
			return false, err
		}
		if lower && c > 0 || !lower && c < 0 {
			lit := smtPBExplain(oppositeExplains, v)
			line := 325
			if !lower {
				line = 346
			}
			if err := t.traceClause("update_bound", line, []SMTLiteral{lit.Negated(), explain.Negated()}); err != nil {
				return false, err
			}
			return false, t.compilationClause([]SMTLiteral{lit.Negated(), explain.Negated()}, SMTClauseAux)
		}
	}
	if lower {
		err = s.solver.SetLower(v, bound)
	} else {
		err = s.solver.SetUpper(v, bound)
	}
	if err != nil {
		return false, err
	}
	previousExplain := smtPBExplain(explains, v)
	explains[v] = explain
	err = t.trail.PushUndo(func() {
		// The trailed bound is weaker than the installed one. Its opposite
		// bound has already been restored by the reverse trail walk.
		if lower {
			if valid {
				_ = s.solver.SetLower(v, &old)
			} else {
				s.solver.UnsetLower(v)
			}
		} else {
			if valid {
				_ = s.solver.SetUpper(v, &old)
			} else {
				s.solver.UnsetUpper(v)
			}
		}
		explains[v] = previousExplain
		_ = old.Del(q)
	})
	keepOld = err == nil
	return err == nil, err
}

// simplexIntBound is a fresh inf_eps(rational(k)) of the AST manager's
// rational manager.
func (t *SMTPseudoBooleanTheory) simplexIntBound(k int64) (simplex.Infinitesimal, error) {
	b := simplex.ZeroInfinitesimal()
	if err := t.q().SetInt64(&b.Main, k); err != nil {
		return b, smtErrJoin(err, b.Del(t.q()))
	}
	return b, nil
}

func (t *SMTPseudoBooleanTheory) simplexInternalize(c *smtPBConstraint) (err error) {
	if !t.controls.EnableSimplex {
		return nil
	}
	if t.simplex == nil {
		solver, err := simplex.NewSolver(t.state.manager.ResourceLimit(), t.state.manager.NumeralService())
		if err != nil {
			return err
		}
		t.simplex = &smtPBSimplex{solver: solver, rows: make(map[int32]smtPBSimplexRow), representatives: make(map[string]int32), variables: make(map[uint32]bool), lower: make(map[uint32]SMTLiteral), upper: make(map[uint32]SMTLiteral)}
	}
	s := t.simplex
	// z3 4.7.1 bug: a predicate's Boolean ID names both its 0-1 literal
	// column and its row's slack column, so a constraint whose argument is
	// itself a PB predicate with a row bounds that row's weighted sum by
	// [0,1] and z3 answers unsat on satisfiable nested constraints, or
	// crashes. The fork fix gives such a constraint no simplex row.
	for _, term := range c.args[0].terms {
		if _, found := s.rows[term.literal.Var()]; found {
			return nil
		}
	}
	// arg_t rep(c->args()); rep.remove_negations(); rep dies on return.
	q := t.q()
	rep, err := c.args[0].copy(q)
	defer func() { err = smtErrJoin(err, rep.del(q)) }()
	if err != nil {
		return err
	}
	var key strings.Builder
	for i := range rep.terms {
		term := &rep.terms[i]
		if term.literal.Sign() {
			term.literal = term.literal.Negated()
			if err = q.Neg(&term.coeff); err != nil {
				return err
			}
			if err = q.Add(&rep.bound, &term.coeff, &rep.bound); err != nil {
				return err
			}
		}
		text, err := q.String(&term.coeff)
		if err != nil {
			return err
		}
		key.WriteString(strconv.FormatInt(int64(term.literal.Var()), 10))
		key.WriteByte(':')
		key.WriteString(text)
		key.WriteByte(';')
	}
	representative := key.String()
	if err := t.emitPB("internalize_atom", 555, func(out io.Writer) error {
		_, _ = fmt.Fprintf(out, "%d\n", c.literal.Var())
		return t.displayPBArgs(out, &rep, false)
	}); err != nil {
		return err
	}
	slack := uint32(c.literal.Var())
	if owner, found := s.representatives[representative]; found {
		slack = uint32(owner)
		if err := t.emitPB("internalize_atom", 558, func(out io.Writer) error {
			info := s.rows[owner]
			old, err := q.String(&info.bound)
			if err != nil {
				return err
			}
			bound, err := q.String(&rep.bound)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(out, "Old row: %d |-> %d %s vs. %s\n", c.literal.Var(), owner, old, bound)
			return t.displayPBArgs(out, &rep, false)
		}); err != nil {
			return err
		}
	} else {
		s.representatives[representative] = c.literal.Var()
		var variables []uint32
		var coefficients []numeral.Rational
		for i := range rep.terms {
			term := &rep.terms[i]
			v := uint32(term.literal.Var())
			s.solver.EnsureVariable(v)
			variables = append(variables, v)
			if !s.variables[v] {
				if err = t.simplexBoundVariable(v, term.literal); err != nil {
					return err
				}
				s.variables[v] = true
				if err = t.trail.PushUndo(func() { delete(s.variables, v); s.solver.UnsetLower(v); s.solver.UnsetUpper(v) }); err != nil {
					return err
				}
			}
			if !term.coeff.IsInt() {
				return ast.ErrASTDomain
			}
			// The row borrows rep's coefficients.
			coefficients = append(coefficients, term.coeff)
		}
		s.solver.EnsureVariable(slack)
		variables = append(variables, slack)
		// m.set(coeffs.back(), -1): a temporary of the row.
		coefficients = append(coefficients, smtQZero())
		minusOne := &coefficients[len(coefficients)-1]
		err = q.SetInt32(minusOne, -1)
		if err == nil {
			_, err = s.solver.AddRationalRow(slack, variables, coefficients)
		}
		if err = smtErrJoin(err, smtQDel(q, minusOne)); err != nil {
			return err
		}
		if err := t.emitPB("internalize_atom", 597, func(out io.Writer) error {
			bound, err := q.String(&rep.bound)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(out, "New row: %d %s\n", c.literal.Var(), bound)
			return t.displayPBArgs(out, &rep, false)
		}); err != nil {
			return err
		}
	}
	bound := smtQZero()
	if err = q.Copy(&bound, &rep.bound); err != nil {
		return err
	}
	s.rows[c.literal.Var()] = smtPBSimplexRow{slack, bound, representative}
	return nil
}

// simplexBoundVariable gives a new 0-1 column its bounds from the
// literal's current assignment (zero and one are temporaries).
func (t *SMTPseudoBooleanTheory) simplexBoundVariable(v uint32, literal SMTLiteral) (err error) {
	s := t.simplex
	q := t.q()
	zero := simplex.ZeroInfinitesimal()
	one, err := t.simplexIntBound(1)
	defer func() { err = smtErrJoin(err, one.Del(q), zero.Del(q)) }()
	if err != nil {
		return err
	}
	value, err := t.state.Assignment(literal)
	if err != nil {
		return err
	}
	switch value {
	case lbool.True:
		if _, err = t.simplexUpdateBound(v, literal, true, &one); err != nil {
			return err
		}
		return s.solver.SetLower(v, &one)
	case lbool.False:
		if _, err = t.simplexUpdateBound(v, literal.Negated(), false, &zero); err != nil {
			return err
		}
		return s.solver.SetUpper(v, &zero)
	default:
		if err = s.solver.SetLower(v, &zero); err != nil {
			return err
		}
		return s.solver.SetUpper(v, &one)
	}
}

func (t *SMTPseudoBooleanTheory) simplexCheckFeasible() (bool, error) {
	s := t.simplex
	status, err := s.solver.MakeFeasible()
	if err != nil || status != lbool.False {
		return err == nil, err
	}
	r, err := s.solver.InfeasibleRow()
	if err != nil {
		return false, err
	}
	sq := s.solver.Numerals()
	coefficient, err := s.solver.BaseCoefficient(r)
	if err != nil {
		return false, err
	}
	coefficientSign, err := sq.Sign(coefficient)
	if err != nil {
		return false, err
	}
	base, err := s.solver.BaseVariable(r)
	if err != nil {
		return false, err
	}
	below, err := t.simplexBelowLower(base)
	if err != nil {
		return false, err
	}
	cantIncrease := below && coefficientSign > 0 || !below && coefficientSign < 0
	entries, err := s.solver.RowEntries(r)
	if err != nil {
		return false, err
	}
	var literals []SMTLiteral
	for _, entry := range entries {
		entrySign, err := sq.Sign(entry.Coefficient)
		if err != nil {
			return false, err
		}
		useLower := cantIncrease == (entrySign > 0)
		if entry.Variable == base {
			useLower = below
		}
		explains := s.upper
		if useLower {
			explains = s.lower
		}
		lit := smtPBExplain(explains, entry.Variable)
		if lit != SMTNullLiteral {
			literals = append(literals, lit.Negated())
		}
	}
	t.statistics.conflicts++
	var owner SMTJustification
	proofs, err := t.state.manager.ProofsEnabled()
	if err != nil {
		return false, err
	}
	if proofs {
		owner, err = NewSMTTheoryLemmaJustification(t.state.manager, t.FamilyID(), literals, t.state.boolExpr, nil)
		if err != nil {
			return false, err
		}
	}
	if err = t.traceClause("check_feasible", 410, literals); err != nil {
		return false, err
	}
	err = t.state.addClauseKindWithJustificationAndDeleteHandler(literals, false, SMTClauseAuxLemma, ast.ASTHandle{}, owner, nil)
	return false, err
}

// simplexBelowLower is the infeasible base's value < lower test (with the
// debug check that it is otherwise above its upper bound); the values read
// are temporaries.
func (t *SMTPseudoBooleanTheory) simplexBelowLower(base uint32) (below bool, err error) {
	s := t.simplex
	q := t.q()
	value, bound := simplex.ZeroInfinitesimal(), simplex.ZeroInfinitesimal()
	defer func() { err = smtErrJoin(err, bound.Del(q), value.Del(q)) }()
	if err = s.solver.Value(base, &value); err != nil {
		return false, err
	}
	valid, err := s.solver.Lower(base, &bound)
	if err != nil {
		return false, err
	}
	if valid {
		c, err := value.Cmp(q, &bound)
		if err != nil {
			return false, err
		}
		below = c < 0
	}
	if smtcoreSourceDebug && !below {
		valid, err := s.solver.Upper(base, &bound)
		if err != nil {
			return false, err
		}
		c := 0
		if valid {
			if c, err = value.Cmp(q, &bound); err != nil {
				return false, err
			}
		}
		if !valid || c <= 0 {
			return false, ast.ErrASTDomain
		}
	}
	return below, nil
}

func (t *SMTPseudoBooleanTheory) simplexAssignArgument(v int32, isTrue bool) (bool, error) {
	value := int64(0)
	if isTrue {
		value = 1
	}
	bound, err := t.simplexIntBound(value)
	ok := false
	if err == nil {
		ok, err = t.simplexUpdateBound(uint32(v), NewSMTLiteral(v, !isTrue), isTrue, &bound)
	}
	if err = smtErrJoin(err, bound.Del(t.q())); err != nil || !ok {
		return ok, err
	}
	return t.simplexCheckFeasible()
}

func (t *SMTPseudoBooleanTheory) simplexAssignPredicate(c *smtPBConstraint, isTrue bool) (bool, error) {
	info, found := t.simplex.rows[c.literal.Var()]
	if !found {
		return false, ast.ErrASTDomain
	}
	q := t.q()
	bound, err := simplex.RationalInfinitesimal(q, &info.bound)
	if err == nil {
		err = t.simplexAssignPredicateBound(c, info.slack, isTrue, &bound)
	}
	if err = smtErrJoin(err, bound.Del(q)); err != nil {
		return false, err
	}
	return t.simplexCheckFeasible()
}

// simplexAssignPredicateBound installs the row's bounds for the predicate's
// assignment; bound is the predicate's scoped_eps_numeral.
func (t *SMTPseudoBooleanTheory) simplexAssignPredicateBound(c *smtPBConstraint, slack uint32, isTrue bool, bound *simplex.Infinitesimal) error {
	explain := NewSMTLiteral(c.literal.Var(), !isTrue)
	if isTrue {
		if _, err := t.simplexUpdateBound(slack, explain, true, bound); err != nil {
			return err
		}
		if c.equality {
			if _, err := t.simplexUpdateBound(slack, explain, false, bound); err != nil {
				return err
			}
		}
	} else if !c.equality {
		if err := t.q().Dec(&bound.Main); err != nil {
			return err
		}
		if _, err := t.simplexUpdateBound(slack, explain, false, bound); err != nil {
			return err
		}
	}
	return nil
}

func (t *SMTPseudoBooleanTheory) simplexRemove(c *smtPBConstraint) error {
	if t.simplex == nil {
		return nil
	}
	s := t.simplex
	info, found := s.rows[c.literal.Var()]
	if !found {
		// No row: the fork fix skipped a constraint over a PB predicate.
		return nil
	}
	owner, found := s.representatives[info.representative]
	if !found {
		return ast.ErrASTDomain
	}
	delete(s.rows, c.literal.Var())
	if err := smtQDel(t.q(), &info.bound); err != nil {
		return err
	}
	if owner == c.literal.Var() {
		if err := s.solver.DeleteRow(info.slack); err != nil {
			return err
		}
		delete(s.representatives, info.representative)
	}
	return nil
}

// simplexHasRow reports whether predicate v has a simplex row; the fork fix
// leaves constraints over PB predicates without one.
func (t *SMTPseudoBooleanTheory) simplexHasRow(v int32) bool {
	if t.simplex == nil {
		return false
	}
	_, found := t.simplex.rows[v]
	return found
}

// simplexIsVariable is m_vars.contains(v): v has a 0-1 column.
func (t *SMTPseudoBooleanTheory) simplexIsVariable(v int32) bool {
	return t.simplex != nil && t.simplex.variables[uint32(v)]
}
