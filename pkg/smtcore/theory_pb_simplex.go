package smtcore

// Source: src/smt/theory_pb.cpp:84-92,252-415,543-600,788-839,1333-1342.

import (
	"fmt"
	"io"
	"math/big"
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

func (t *SMTPseudoBooleanTheory) simplexUpdateBound(v uint32, explain SMTLiteral, lower bool, bound simplex.Infinitesimal) (bool, error) {
	s := t.simplex
	var old, opposite simplex.Infinitesimal
	var valid, oppositeValid bool
	explains, oppositeExplains := s.lower, s.upper
	if lower {
		old, valid = s.solver.Lower(v)
		opposite, oppositeValid = s.solver.Upper(v)
		if valid && bound.Cmp(&old) <= 0 {
			return true, nil
		}
	} else {
		old, valid = s.solver.Upper(v)
		opposite, oppositeValid = s.solver.Lower(v)
		explains, oppositeExplains = s.upper, s.lower
		if valid && bound.Cmp(&old) >= 0 {
			return true, nil
		}
	}
	if oppositeValid && (lower && bound.Cmp(&opposite) > 0 || !lower && bound.Cmp(&opposite) < 0) {
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
	var err error
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
				_ = s.solver.SetLower(v, old)
			} else {
				s.solver.UnsetLower(v)
			}
		} else {
			if valid {
				_ = s.solver.SetUpper(v, old)
			} else {
				s.solver.UnsetUpper(v)
			}
		}
		explains[v] = previousExplain
	})
	return err == nil, err
}

func (t *SMTPseudoBooleanTheory) simplexInternalize(c *smtPBConstraint) (err error) {
	if !t.controls.EnableSimplex {
		return nil
	}
	if t.simplex == nil {
		t.simplex = &smtPBSimplex{solver: simplex.NewSolver(t.state.manager.ResourceLimit()), rows: make(map[int32]smtPBSimplexRow), representatives: make(map[string]int32), variables: make(map[uint32]bool), lower: make(map[uint32]SMTLiteral), upper: make(map[uint32]SMTLiteral)}
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
		var coefficients []big.Int
		for _, term := range rep.terms {
			v := uint32(term.literal.Var())
			s.solver.EnsureVariable(v)
			variables = append(variables, v)
			if !s.variables[v] {
				zero := simplex.RationalInfinitesimal(big.NewRat(0, 1))
				one := simplex.RationalInfinitesimal(big.NewRat(1, 1))
				value, err := t.state.Assignment(term.literal)
				if err != nil {
					return err
				}
				switch value {
				case lbool.True:
					if _, err = t.simplexUpdateBound(v, term.literal, true, one); err != nil {
						return err
					}
					if err = s.solver.SetLower(v, one); err != nil {
						return err
					}
				case lbool.False:
					if _, err = t.simplexUpdateBound(v, term.literal.Negated(), false, zero); err != nil {
						return err
					}
					if err = s.solver.SetUpper(v, zero); err != nil {
						return err
					}
				default:
					if err = s.solver.SetLower(v, zero); err != nil {
						return err
					}
					if err = s.solver.SetUpper(v, one); err != nil {
						return err
					}
				}
				s.variables[v] = true
				if err = t.trail.PushUndo(func() { delete(s.variables, v); s.solver.UnsetLower(v); s.solver.UnsetUpper(v) }); err != nil {
					return err
				}
			}
			if !term.coeff.IsInt() {
				return ast.ErrASTDomain
			}
			// The simplex solver takes math/big rows until it moves to
			// mpq_inf numerals.
			b, err := smtQBig(q, &term.coeff)
			if err != nil {
				return err
			}
			var coefficient big.Int
			coefficient.Set(b.Num())
			coefficients = append(coefficients, coefficient)
		}
		s.solver.EnsureVariable(slack)
		variables = append(variables, slack)
		coefficients = append(coefficients, *big.NewInt(-1))
		if _, err := s.solver.AddIntegerRow(slack, variables, coefficients); err != nil {
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
	coefficient, err := s.solver.BaseCoefficient(r)
	if err != nil {
		return false, err
	}
	base, err := s.solver.BaseVariable(r)
	if err != nil {
		return false, err
	}
	value := s.solver.Value(base)
	bound, valid := s.solver.Lower(base)
	below := valid && value.Cmp(&bound) < 0
	if smtcoreSourceDebug && !below {
		upper, valid := s.solver.Upper(base)
		if !valid || value.Cmp(&upper) <= 0 {
			return false, ast.ErrASTDomain
		}
	}
	cantIncrease := below && coefficient.Sign() > 0 || !below && coefficient.Sign() < 0
	entries, err := s.solver.RowEntries(r)
	if err != nil {
		return false, err
	}
	var literals []SMTLiteral
	for _, entry := range entries {
		useLower := cantIncrease == (entry.Coefficient.Sign() > 0)
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

func (t *SMTPseudoBooleanTheory) simplexAssignArgument(v int32, isTrue bool) (bool, error) {
	value := int64(0)
	if isTrue {
		value = 1
	}
	ok, err := t.simplexUpdateBound(uint32(v), NewSMTLiteral(v, !isTrue), isTrue, simplex.RationalInfinitesimal(big.NewRat(value, 1)))
	if err != nil || !ok {
		return ok, err
	}
	return t.simplexCheckFeasible()
}

func (t *SMTPseudoBooleanTheory) simplexAssignPredicate(c *smtPBConstraint, isTrue bool) (bool, error) {
	info, found := t.simplex.rows[c.literal.Var()]
	if !found {
		return false, ast.ErrASTDomain
	}
	main, err := smtQBig(t.q(), &info.bound)
	if err != nil {
		return false, err
	}
	bound := simplex.RationalInfinitesimal(main)
	explain := NewSMTLiteral(c.literal.Var(), !isTrue)
	if isTrue {
		if _, err := t.simplexUpdateBound(info.slack, explain, true, bound); err != nil {
			return false, err
		}
		if c.equality {
			if _, err := t.simplexUpdateBound(info.slack, explain, false, bound); err != nil {
				return false, err
			}
		}
	} else if !c.equality {
		bound.Main.Sub(&bound.Main, big.NewRat(1, 1))
		if _, err := t.simplexUpdateBound(info.slack, explain, false, bound); err != nil {
			return false, err
		}
	}
	return t.simplexCheckFeasible()
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
