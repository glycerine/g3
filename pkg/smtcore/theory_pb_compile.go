package smtcore

// Source: src/smt/theory_pb.cpp:516-539,1229-1317,1322-1354.

import (
	"fmt"
	"io"
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (t *SMTPseudoBooleanTheory) initCompilation(c *smtPBConstraint) {
	c.compiled = lbool.False
	c.compilationThreshold = math.MaxUint32
	if !t.controls.EnableCompilation || c.equality || c.args[0].bound.IsOne() {
		return
	}
	// args[i].second <= m_max_compiled_coeff (rational(8))
	maxCompiled := smtQInt(8)
	for i := range c.args[0].terms {
		cmp, err := t.q().Compare(&c.args[0].terms[i].coeff, &maxCompiled)
		t.keep(err)
		if cmp > 0 {
			return
		}
	}
	size := uint32(len(c.args[0].terms))
	log, n := uint32(1), uint32(1)
	for n <= size {
		log++
		n *= 2
	}
	c.compilationThreshold = size * log * log
}

func (t *SMTPseudoBooleanTheory) incPropagations(c *smtPBConstraint) {
	c.propagations++
	if c.compiled == lbool.False && c.propagations >= c.compilationThreshold {
		c.compiled = lbool.Undef
		t.toCompile = append(t.toCompile, c)
	}
}

func (t *SMTPseudoBooleanTheory) Restart() error {
	if _, err := t.State(); err != nil {
		return err
	}
	for _, c := range t.toCompile {
		if err := t.compileInequality(c); err != nil {
			return err
		}
	}
	clear(t.toCompile)
	t.toCompile = t.toCompile[:0]
	return nil
}

func (t *SMTPseudoBooleanTheory) compilationClause(literals []SMTLiteral, kind SMTClauseKind) error {
	var owner SMTJustification
	enabled, err := t.state.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	if enabled {
		axiom, err := NewSMTTheoryAxiomJustification(t.state.manager, t.FamilyID(), literals, nil)
		if err != nil {
			return err
		}
		owner = axiom
	}
	return t.state.addClauseKindWithJustificationAndDeleteHandler(literals, false, kind, ast.ASTHandle{}, owner, nil)
}

func (t *SMTPseudoBooleanTheory) compileInequality(c *smtPBConstraint) error {
	t.statistics.compiled++
	if smtcoreSourceDebug && c.compilationThreshold == ^uint32(0) {
		return ast.ErrASTDomain
	}
	s := t.state
	base := s.scopeLevel
	if s.searching {
		base = s.searchUserBaseLevel
	}
	a := c.current()
	q := t.q()
	// c.k().get_unsigned() and c.coeff(i).get_unsigned()
	unsigned := func(r *numeral.Rational) (uint32, error) {
		ok, err := q.IsUnsigned(r)
		if err != nil {
			return 0, err
		}
		if !ok {
			return 0, ast.ErrASTDomain
		}
		return q.Unsigned(r)
	}
	k, err := unsigned(&a.bound)
	if err != nil {
		return err
	}
	var in []SMTLiteral
	for i := range a.terms {
		term := &a.terms[i]
		m, err := unsigned(&term.coeff)
		if err != nil {
			return err
		}
		value, err := s.Assignment(term.literal)
		if err != nil {
			return err
		}
		if value != lbool.Undef && s.booleanData[term.literal.Var()].ScopeLevel() == base {
			if value == lbool.True {
				if k < m {
					return nil
				}
				k -= m
			}
			continue
		}
		for j := uint32(0); j < m; j++ {
			in = append(in, term.literal)
		}
	}
	if err := t.emitPB("compile_ineq", 1279, func(out io.Writer) error {
		if err := smtPBWriteLiterals(out, in, false); err != nil {
			return err
		}
		_, err := fmt.Fprintf(out, " >= %d\n", k)
		return err
	}); err != nil {
		return err
	}
	thl := c.lit()
	value, err := s.Assignment(thl)
	if err != nil {
		return err
	}
	full := value != lbool.True || s.booleanData[thl.Var()].ScopeLevel() != base
	network := smtPBSort{theory: t}
	atLeast := network.ge(full, k, in)
	if network.err != nil {
		return network.err
	}
	line := 1289
	if full {
		line = 1294
	}
	if err = t.traceClause("compile_ineq", line, []SMTLiteral{thl.Negated(), atLeast}); err != nil {
		return err
	}
	if err = t.compilationClause([]SMTLiteral{thl.Negated(), atLeast}, SMTClauseAux); err != nil {
		return err
	}
	if full {
		if err = t.compilationClause([]SMTLiteral{atLeast.Negated(), thl}, SMTClauseAux); err != nil {
			return err
		}
	}
	t.statistics.compiledVariables += network.variables
	t.statistics.compiledClauses += network.clauses
	if err = smtPBVerbose(2, func(out io.Writer) error {
		_, err := fmt.Fprintf(out, "(smt.pb compile sorting network bound: %d literals: %d clauses: %d vars: %d)\n", k, uint32(len(in)), network.clauses, network.variables)
		return err
	}); err != nil {
		return err
	}
	// Clauses disappear on scope pop; the surviving native constraint must
	// become eligible for another compilation after that same rollback.
	c.compiled = lbool.False
	if err = t.trail.PushUndo(func() { c.compiled = lbool.False }); err != nil {
		return err
	}
	c.compiled = lbool.True
	return nil
}
