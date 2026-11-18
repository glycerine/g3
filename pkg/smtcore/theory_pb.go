package smtcore

// Source: src/smt/theory_pb.cpp and src/smt/theory_pb.h retained exact
// pseudo-Boolean atom semantics.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/backtrack"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/statistics"
)

type smtPseudoBooleanStatistics struct {
	predicates                         uint32
	compiled                           uint32
	compiledVariables, compiledClauses uint32
	propagations, conflicts            uint32
}

// SMTPseudoBooleanTheory keeps concrete normalized constraints and native
// variable/literal watches, weighted conflict resolution and restart-triggered
// sorting-network compilation, with the optional shared-row simplex relaxation.
type SMTPseudoBooleanTheory struct {
	*SMTTheoryBase
	statistics     smtPseudoBooleanStatistics
	watches        map[int32]*smtPBVariableWatches
	constraints    []*smtPBConstraint
	toCompile      []*smtPBConstraint
	limits         []int
	trail          *backtrack.TrailStack
	controls       SMTPBState
	configured     bool
	parameterOwner *SMTPBParams // borrowed only until the source setup boundary
	simplex        *smtPBSimplex
	trace          *diagnostic.TraceTags // optional borrowed stream; nil uses source process tags
	qErr           error                 // first numeral failure of a step that cannot return it
}

func NewSMTPseudoBooleanTheory(family ast.FamilyID) (*SMTPseudoBooleanTheory, error) {
	base, err := NewSMTTheoryBase(family)
	if err != nil {
		return nil, err
	}
	return &SMTPseudoBooleanTheory{SMTTheoryBase: base, trail: backtrack.NewTrailStack(), controls: SMTPBState{ConflictFrequency: 1000, LearnComplements: true, EnableCompilation: true}}, nil
}

func (t *SMTPseudoBooleanTheory) operation(expression ast.ASTHandle) (int32, bool, error) {
	state, err := t.State()
	if err != nil {
		return 0, false, err
	}
	family, err := state.manager.GetFamilyID(expression)
	if err != nil || family != t.FamilyID() {
		return 0, false, err
	}
	op, err := state.manager.GetDeclKind(expression)
	return op, err == nil, err
}

// smtPseudoBooleanRational moves a pb_util rational (owned by the AST
// manager's numeral service) into out, a rational of the same manager, and
// frees the service value.
func smtPseudoBooleanRational(manager *ast.ASTManager, value unumeral.OwnedRational, out *unumeral.Rational) error {
	service := manager.NumeralService()
	err := service.Manager.Copy(out, &value.Raw)
	return smtErrJoin(err, service.Del(&value))
}

func (t *SMTPseudoBooleanTheory) InternalizeAtom(expression ast.ASTHandle, _ bool) (bool, error) {
	return t.internalizeAtom(expression, false)
}

func (t *SMTPseudoBooleanTheory) internalizeAtom(expression ast.ASTHandle, retainAssignments bool) (_ bool, err error) {
	if err := t.Setup(); err != nil {
		return false, err
	}
	op, own, err := t.operation(expression)
	if err != nil || !own || op < ast.ASTPBOpAtMost || op > ast.ASTPBOpAuxBool {
		return false, err
	}
	state, _ := t.State()
	if _, found := state.boolVar.lookup(expression); found {
		return false, nil
	}
	variable, err := state.MkBoolVar(expression)
	if err != nil {
		return false, err
	}
	if err = state.SetVariableTheory(variable, t.FamilyID()); err != nil {
		return false, err
	}
	t.statistics.predicates++
	if op == ast.ASTPBOpAuxBool {
		return true, nil
	}
	c := newSMTPBConstraint(NewSMTLiteral(variable, false), op == ast.ASTPBOpEQ)
	q := t.q()
	// c is freed unless it is retained (m_var_infos[abv].m_ineq = c).
	retained := false
	defer func() {
		if !retained {
			err = smtErrJoin(err, c.del(q))
		}
	}()
	bound, err := state.manager.PseudoBooleanBound(expression)
	if err != nil {
		return false, err
	}
	if err = smtPseudoBooleanRational(state.manager, bound, &c.args[0].bound); err != nil {
		return false, err
	}
	count, err := state.manager.GetNumArgs(expression)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < count; i++ {
		h, err := state.manager.GetArg(expression, i)
		if err != nil {
			return false, err
		}
		literal, err := t.compileArgument(h)
		if err != nil {
			return false, err
		}
		owned, err := state.manager.PseudoBooleanCoefficient(expression, i)
		if err != nil {
			return false, err
		}
		// numeral c = m_util.get_coeff(atom, i), destroyed each iteration.
		coefficient := smtQZero()
		if err = smtPseudoBooleanRational(state.manager, owned, &coefficient); err != nil {
			return false, smtErrJoin(err, smtQDel(q, &coefficient))
		}
		value, err := state.Assignment(literal)
		if err != nil {
			return false, smtErrJoin(err, smtQDel(q, &coefficient))
		}
		// A strengthened consequence must keep its scoped false inputs in
		// the definition. Folding them into an axiom can erase assumption
		// premises and produce an invalid learned Boolean unit.
		if retainAssignments {
			value = lbool.Undef
		}
		switch value {
		case lbool.True:
			err = q.Sub(&c.args[0].bound, &coefficient, &c.args[0].bound)
		case lbool.Undef:
			err = c.args[0].pushBack(q, literal, &coefficient)
		}
		if e := smtQDel(q, &coefficient); e != nil && err == nil {
			err = e
		}
		if err != nil {
			return false, err
		}
	}
	a := &c.args[0]
	if op == ast.ASTPBOpAtMost || op == ast.ASTPBOpLE {
		for i := range a.terms {
			if err = q.Neg(&a.terms[i].coeff); err != nil {
				return false, err
			}
		}
		if err = q.Neg(&a.bound); err != nil {
			return false, err
		}
	}
	if err = t.traceConstraint("internalize_atom", 470, c, true); err != nil {
		return false, err
	}
	if err = a.unique(q); err != nil {
		return false, err
	}
	truth, err := a.normalize(q, c.equality)
	if err != nil {
		return false, err
	}
	// z3 4.7.1 bug (fork fix in theory_pb::internalize_atom): an equality
	// whose arguments were all assigned keeps none and a negative bound;
	// normalize leaves it undecided, but a constraint with no arguments is
	// never watched, so z3 never falsifies it and answers sat on unsat
	// problems. Fold it to false, as pb_rewriter::mk_app_core does.
	if truth == lbool.Undef && c.equality && len(a.terms) == 0 {
		truth = lbool.False
	}
	if err = a.prune(q, c.equality); err != nil {
		return false, err
	}
	if truth == lbool.Undef && !c.equality {
		if err = a.negate(q); err != nil {
			return false, err
		}
		if err = a.negate(q); err != nil {
			return false, err
		}
		// m_args[1].reset(); m_args[1].m_k = m_args[0].m_k; append; negate
		if err = c.args[1].reset(q); err != nil {
			return false, err
		}
		if err = q.Copy(&c.args[1].bound, &a.bound); err != nil {
			return false, err
		}
		for i := range a.terms {
			if err = c.args[1].pushBack(q, a.terms[i].literal, &a.terms[i].coeff); err != nil {
				return false, err
			}
		}
		if err = c.args[1].negate(q); err != nil {
			return false, err
		}
		if smtcoreSourceDebug {
			if len(a.terms) != len(c.args[1].terms) {
				return false, ast.ErrASTDomain
			}
			if err = a.validateWellFormed(t.q()); err != nil {
				return false, err
			}
			if err = c.args[1].validateWellFormed(t.q()); err != nil {
				return false, err
			}
		}
	}
	if err = t.emitPB("internalize_atom", 484, func(out io.Writer) error {
		if err := t.displayInequality(out, c, false); err != nil {
			return err
		}
		_, _ = io.WriteString(out, " := ")
		if err := smtPBWriteLiteral(out, c.literal); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	}); err != nil {
		return false, err
	}
	if truth != lbool.Undef {
		lit := c.literal
		if truth == lbool.False {
			lit = lit.Negated()
		}
		return true, state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{lit})
	}
	if !c.equality {
		// The source does not dealloc c on this path (4.7.1 leaks it); the
		// deferred del frees it.
		if a.bound.IsOne() && !t.controls.EnableSimplex {
			literals := []SMTLiteral{c.literal.Negated()}
			for _, term := range a.terms {
				if smtcoreSourceDebug && !term.coeff.IsOne() {
					return false, ast.ErrASTDomain
				}
				literals = append(literals, term.literal)
				if err = state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{c.literal, term.literal.Negated()}); err != nil {
					return false, err
				}
			}
			return true, state.addTheoryAxiom(t.FamilyID(), literals)
		}
	}
	t.initCompilation(c)
	if c.compilationThreshold != ^uint32(0) {
		if err = smtPBVerbose(2, func(out io.Writer) error {
			_, err := fmt.Fprintf(out, "(smt.pb setting compilation threshold to %d)\n", c.compilationThreshold)
			return err
		}); err != nil {
			return false, err
		}
		if err = t.emitPB("internalize_atom", 533, func(out io.Writer) error {
			_, err := fmt.Fprintf(out, "compilation threshold: %d\n", c.compilationThreshold)
			return err
		}); err != nil {
			return false, err
		}
	}
	t.initVariableWatch(c)
	if err = t.takeQErr(nil); err != nil {
		return false, err
	}
	t.info(variable).constraint = c
	t.constraints = append(t.constraints, c)
	retained = true
	if err = t.simplexInternalize(c); err != nil {
		return false, err
	}
	if err = t.traceConstraint("internalize_atom", 602, c, false); err != nil {
		return false, err
	}
	return true, nil
}

func (*SMTPseudoBooleanTheory) InternalizeTerm(ast.ASTHandle) (bool, error) {
	if smtcoreSourceDebug {
		return false, ast.ErrASTDomain
	}
	return false, nil
}
func (*SMTPseudoBooleanTheory) InternalizeEquality(ast.ASTHandle, int32) error     { return nil }
func (*SMTPseudoBooleanTheory) ApplySortConstraint(*SMTEnode, ast.ASTHandle) error { return nil }
func (*SMTPseudoBooleanTheory) UseDisequalities() bool                             { return false }
func (*SMTPseudoBooleanTheory) NewEquality(SMTTheoryVar, SMTTheoryVar) error       { return ast.ErrASTDomain }
func (*SMTPseudoBooleanTheory) NewDisequality(SMTTheoryVar, SMTTheoryVar) error {
	return ast.ErrASTDomain
}
func (t *SMTPseudoBooleanTheory) Assign(v int32, value bool) error { return t.assignNative(v, value) }
func (t *SMTPseudoBooleanTheory) FinalCheck() (SMTFinalCheckStatus, error) {
	if err := t.emitPB("final_check_eh", 776, func(out io.Writer) error { return t.Display(out) }); err != nil {
		return SMTFinalCheckDone, err
	}
	if smtcoreSourceDebug {
		for _, v := range t.watchVariables() {
			if c := t.watches[v].constraint; c != nil {
				if err := t.validateFinalConstraint(c); err != nil {
					return SMTFinalCheckDone, err
				}
			}
		}
	}
	return SMTFinalCheckDone, nil
}

// Name is theory::get_name; theory_pb does not override it (AUDIT.md FUZZ-F5).
func (*SMTPseudoBooleanTheory) Name() string { return "unknown" }

func (t *SMTPseudoBooleanTheory) PushScope() error {
	if err := t.SMTTheoryBase.PushScope(); err != nil {
		return err
	}
	t.trail.PushScope()
	t.limits = append(t.limits, len(t.constraints))
	return nil
}

func (t *SMTPseudoBooleanTheory) PopScopes(count uint32) error {
	if count == 0 || count > uint32(len(t.limits)) {
		return ast.ErrASTDomain
	}
	if err := t.trail.PopScope(count); err != nil {
		return err
	}
	level := len(t.limits) - int(count)
	limit := t.limits[level]
	for index := len(t.constraints) - 1; index >= limit; index-- {
		c := t.constraints[index]
		t.clearWatch(c)
		t.info(c.literal.Var()).constraint = nil
		if err := t.simplexRemove(c); err != nil {
			return err
		}
		t.toCompile = smtPBRemove(t.toCompile, c)
		if err := c.del(t.q()); err != nil {
			return err
		}
	}
	clear(t.constraints[limit:])
	t.constraints = t.constraints[:limit]
	t.limits = t.limits[:level]
	return t.SMTTheoryBase.PopScopes(count)
}

func (t *SMTPseudoBooleanTheory) Close() error {
	if t == nil {
		return ast.ErrASTHandle
	}
	if err := t.trail.Reset(); err != nil {
		return err
	}
	// reset_eh: m_var_infos[i].reset() deallocs each retained ineq.
	if t.state != nil && t.state.manager != nil && t.state.manager.NumeralService() != nil {
		for _, c := range t.constraints {
			if err := c.del(t.q()); err != nil {
				return err
			}
		}
		// m_ineq_row_info's row bounds die with the theory.
		if t.simplex != nil {
			for v, info := range t.simplex.rows {
				if err := smtQDel(t.q(), &info.bound); err != nil {
					return err
				}
				delete(t.simplex.rows, v)
			}
		}
	}
	t.watches = nil
	t.constraints = nil
	t.toCompile = nil
	t.limits = nil
	t.variables = nil
	t.parameterOwner = nil
	if t.simplex != nil {
		// ~simplex frees its numerals and closes its manager.
		if err := t.simplex.solver.Close(); err != nil {
			return err
		}
	}
	t.simplex = nil
	t.trace = nil
	return nil
}

func (t *SMTPseudoBooleanTheory) CollectStatistics(target *statistics.Statistics) error {
	if t == nil || target == nil {
		return ast.ErrASTHandle
	}
	target.UpdateUint("pb conflicts", t.statistics.conflicts)
	target.UpdateUint("pb propagations", t.statistics.propagations)
	target.UpdateUint("pb predicates", t.statistics.predicates)
	target.UpdateUint("pb compilations", t.statistics.compiled)
	target.UpdateUint("pb compiled clauses", t.statistics.compiledClauses)
	target.UpdateUint("pb compiled vars", t.statistics.compiledVariables)
	if t.simplex != nil {
		s := t.simplex.solver
		target.UpdateUint("simplex add rows", s.Matrix().AddRowCount())
		target.UpdateUint("simplex num pivots", s.Stats.NumPivots)
		target.UpdateUint("simplex num infeasible", s.Stats.NumInfeasible)
		target.UpdateUint("simplex num checks", s.Stats.NumChecks)
	}
	return nil
}

func (t *SMTPseudoBooleanTheory) ResetStatistics() {
	if t != nil {
		t.statistics = smtPseudoBooleanStatistics{}
	}
}
