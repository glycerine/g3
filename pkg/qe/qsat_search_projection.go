package qe

// Source: src/qe/qsat.cpp qsat::get_core/add_assumption/project_qe/project.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *qsatSearchCall) conjunction(forms []ast.ASTRef) (ast.ASTHandle, error) {
	s := c.owner
	hs := make([]ast.ASTHandle, len(forms))
	for i, r := range forms {
		h, err := s.manager.RefGet(r)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		hs[i] = h
	}
	return c.hold(s.manager.MkAndArray(hs))
}
func (s *qsatSearch) assertBoth(h ast.ASTHandle) error {
	if err := s.exists.assertExpression(h); err != nil {
		return err
	}
	return s.forall.assertExpression(h)
}
func (s *qsatSearch) prepareKernels(c *qsatSearchCall, formula ast.ASTRef) error {
	h, err := s.manager.RefGet(formula)
	if err != nil {
		return err
	}
	if err = s.releaseOperatorDefs(); err != nil {
		return err
	}
	defs := &s.operatorDefs
	level := EmptyMaxLevel()
	if err = s.preparation.abstraction.AbstractAtoms(h, &level, defs); err != nil {
		return err
	}
	if err = s.preparation.abstraction.MakeAbstract(h, formula); err != nil {
		return err
	}
	// m_ex.assert_expr(mk_and(defs)); m_fa.assert_expr(mk_and(defs)): each
	// conjunction is a temporary of its statement.
	hs := make([]ast.ASTHandle, len(*defs))
	for i, r := range *defs {
		if hs[i], err = s.manager.RefGet(r); err != nil {
			return err
		}
	}
	temporary := func(build func() (ast.ASTHandle, error), k *qsatKernel) error {
		t, err := build()
		if err != nil {
			return err
		}
		ref, err := s.manager.NewRef(t)
		if err != nil {
			return err
		}
		return errors.Join(k.assertExpression(t), s.manager.DelRef(&ref))
	}
	conjunction := func() (ast.ASTHandle, error) { return s.manager.MkAndArray(hs) }
	if err = temporary(conjunction, s.exists); err != nil {
		return err
	}
	if err = temporary(conjunction, s.forall); err != nil {
		return err
	}
	h, err = s.manager.RefGet(formula)
	if err != nil {
		return err
	}
	if err = s.exists.assertExpression(h); err != nil {
		return err
	}
	// m_fa.assert_expr(m.mk_not(fml)): the negation is a raw app*, owned
	// only by what the solver keeps of it.
	negated, err := s.manager.MkNot(h)
	if err != nil {
		return err
	}
	return s.forall.assertExpression(negated)
}

// initialize is the retained top-level SAT/QE setup preceding check_sat.
// The goal/tactic caller keeps its original formula until the search finishes.
func (s *qsatSearch) initialize(formula ast.ASTRef) (err error) {
	if err = s.open(); err != nil {
		return err
	}
	if err = s.reset(); err != nil {
		return err
	}
	c, err := s.begin()
	if err != nil {
		return err
	}
	defer s.finish(c, &err)
	if err = s.preparation.abstraction.emit("operator()", 1267, func(w io.Writer) error {
		h, e := s.manager.RefGet(formula)
		if e != nil {
			return e
		}
		if e = writeExpression(s.manager, s.config, w, h); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	}); err != nil {
		return err
	}
	if s.mode != qsatSAT {
		h, e := s.manager.RefGet(formula)
		if e != nil {
			return e
		}
		// fml = push_not(fml): only fml holds the result.
		h, err = s.manager.PushNot(h)
		if err != nil {
			return err
		}
		if err = s.manager.SetRef(formula, h); err != nil {
			return err
		}
	}
	if err = s.preparation.hoist(s.mode, formula); err != nil {
		return err
	}
	if err = s.prepareKernels(c, formula); err != nil {
		return err
	}
	return s.preparation.abstraction.emit("operator()", 1278, func(w io.Writer) error {
		_, _ = io.WriteString(w, "ex: ")
		h, e := s.manager.RefGet(formula)
		if e != nil {
			return e
		}
		if e = writeExpression(s.manager, s.config, w, h); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	})
}
func (s *qsatSearch) getCore(core *[]ast.ASTRef, level uint32) error {
	if err := s.requireDefinitions("get_core"); err != nil {
		return err
	}
	if err := s.kernel(level).getCore(core); err != nil {
		return err
	}
	return s.preparation.abstraction.PredicatesToLiterals(core)
}

// negateCore is ::push_not(::mk_and(core)): the conjunction is a temporary
// that dies once the negation is built.
func (s *qsatSearch) negateCore(c *qsatSearchCall, core []ast.ASTRef) (result ast.ASTHandle, err error) {
	hs := make([]ast.ASTHandle, len(core))
	for i, r := range core {
		if hs[i], err = s.manager.RefGet(r); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	and, err := s.manager.MkAndArray(hs)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	temporary, err := s.manager.NewRef(and)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, s.manager.DelRef(&temporary)) }()
	return c.hold(s.manager.PushNot(and))
}

// addAssumption is qsat::add_assumption; its locals (eq, b) end when it
// returns.
func (s *qsatSearch) addAssumption(formula ast.ASTHandle) error {
	c := s.call()
	if err := s.addAssumptionCore(c, formula); err != nil {
		// Failed-call owners stay on pending for the outer cleanup receipt.
		return err
	}
	return s.closeRound(c)
}

func (s *qsatSearch) addAssumptionCore(c *qsatSearchCall, formula ast.ASTHandle) error {
	r, err := c.ref(ast.ASTHandle{})
	if err != nil {
		return err
	}
	if err = s.preparation.abstraction.FreshBoolean("b", r); err != nil {
		return err
	}
	b, err := s.manager.RefGet(r)
	if err != nil {
		return err
	}
	if err = s.appendRef(&s.assumptions, b); err != nil {
		return err
	}
	eq, err := c.hold(s.manager.MkEq(b, formula))
	if err != nil {
		return err
	}
	if err = s.assertBoth(eq); err != nil {
		return err
	}
	if err = s.preparation.abstraction.AddPredicate(b, formula); err != nil {
		return err
	}
	return s.preparation.abstraction.SetExpressionLevel(b, EmptyMaxLevel())
}

// projectQE is qsat::project_qe; its local fml ends when it returns.
func (s *qsatSearch) projectQE(_ *qsatSearchCall, core *[]ast.ASTRef) error {
	c := s.call()
	if err := s.projectQECore(c, core); err != nil {
		// Failed-call owners stay on pending for the outer cleanup receipt.
		return err
	}
	return s.closeRound(c)
}

func (s *qsatSearch) projectQECore(c *qsatSearchCall, core *[]ast.ASTRef) error {
	if s.level != 1 {
		return ast.ErrASTDomain
	}
	md, err := s.current.Get()
	if err != nil {
		return err
	}
	if md == nil {
		return ast.ErrASTHandle
	}
	if err = s.getCore(core, s.level); err != nil {
		return err
	}
	if err = s.preparation.getVariables(s.level, &s.projectionVariables); err != nil {
		return err
	}
	if ast.ASTSourceDebug {
		valid, e := c.validateAssumptionsDebug(md, *core)
		if e != nil {
			return e
		}
		if !valid {
			return ast.ErrASTDomain
		}
	}
	if err = s.mbp.Project(s.mode != qsatRecursiveQE, &s.projectionVariables, md, core); err != nil {
		return err
	}
	if err = s.requireDefinitions("project_qe"); err != nil {
		return err
	}
	if s.mode == qsatMaximize {
		if err = s.maximizeCore(core); err != nil {
			return err
		}
	} else {
		fml, err := s.negateCore(c, *core)
		if err != nil {
			return err
		}
		if err = s.addAssumption(fml); err != nil {
			return err
		}
		if err = s.appendRef(&s.answers, fml); err != nil {
			return err
		}
		if err = s.appendRefs(&s.freeVariables, s.projectionVariables); err != nil {
			return err
		}
	}
	return s.pop(1)
}
func (s *qsatSearch) project(_ *qsatSearchCall, core *[]ast.ASTRef) (bool, error) {
	if err := s.getCore(core, s.level); err != nil {
		return false, err
	}
	// project's own locals (expr_ref fml; expr_ref_vector defs) end when
	// project returns, defs before fml, not with the check_sat round.
	c := s.call()
	ok, err := s.projectCore(c, core)
	if err != nil {
		// Failed-call owners stay on pending for the outer cleanup receipt.
		return ok, err
	}
	return ok, s.closeRound(c)
}

func (s *qsatSearch) projectCore(c *qsatSearchCall, core *[]ast.ASTRef) (bool, error) {
	if err := s.preparation.abstraction.emit("project", 892, func(w io.Writer) error {
		if e := s.display(w); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "core\n")
		return s.preparation.abstraction.DisplayAssumptions(w, *core)
	}); err != nil {
		return false, err
	}
	if s.level < 2 {
		return false, ast.ErrASTDomain
	}
	md, err := s.current.Get()
	if err != nil {
		return false, err
	}
	if md == nil {
		return false, ast.ErrASTHandle
	}
	if err = s.preparation.getVariables(s.level-1, &s.projectionVariables); err != nil {
		return false, err
	}
	// Source validate_core/validate_project are true stubs with disabled bodies.
	if err = s.mbp.Project(s.mode != qsatRecursiveQE, &s.projectionVariables, md, core); err != nil {
		return false, err
	}
	if err = s.appendRefs(&s.freeVariables, s.projectionVariables); err != nil {
		return false, err
	}
	// fml = negate_core(core): one expr_ref, later reassigned by mk_abstract.
	negated, err := s.negateCore(c, *core)
	if err != nil {
		return false, err
	}
	fmlRef := c.refs[len(c.refs)-1] // negateCore's hold
	fml := negated
	defs := c.vector()
	level := EmptyMaxLevel()
	if err = s.preparation.abstraction.AbstractAtoms(fml, &level, defs); err != nil {
		return false, err
	}
	// m_ex/m_fa.assert_expr(mk_and(defs)): the conjunction is a temporary
	// of each statement.
	for i := 0; i < 2; i++ {
		d, e := c.conjunction(*defs)
		if e != nil {
			return false, e
		}
		kernel := s.exists
		if i == 1 {
			kernel = s.forall
		}
		if e = kernel.assertExpression(d); e != nil {
			return false, e
		}
		if e = c.releaseHeld(d); e != nil {
			return false, e
		}
	}
	var scopes uint32
	if level.Max() == NoQuantifierLevel {
		scopes = 2 * (s.level / 2)
	} else {
		// BACKPORT upstream z3 1ce0d7512 / 8ad939a10 (fix #2974/#2990): the
		// qsat_qe_rec special case (num_scopes = 2) is removed; elim now
		// hoists like qe mode, so the ordinary level computation applies.
		if uint64(level.Max())+2 > uint64(s.level) {
			return false, nil
		}
		scopes = s.level - level.Max()
		if scopes%2 != 0 {
			scopes--
		}
	}
	if err = s.pop(scopes); err != nil {
		return false, err
	}
	if err = s.preparation.abstraction.emit("project", 926, func(w io.Writer) error {
		_, _ = fmt.Fprintf(w, "backtrack: %d new level: %d\nproject:\n", scopes, s.level)
		if e := writeExpressionVector(s.manager, s.config, w, *core); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n|->\n")
		if e := writeExpression(s.manager, s.config, w, fml); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	}); err != nil {
		return false, err
	}
	if s.level == 0 && s.mode != qsatSAT {
		err = s.addAssumption(fml)
	} else {
		// fml = m_pred_abs.mk_abstract(fml)
		if err = s.preparation.abstraction.MakeAbstract(fml, fmlRef); err != nil {
			return false, err
		}
		h, e := s.manager.RefGet(fmlRef)
		if e != nil {
			return false, e
		}
		err = s.kernel(s.level).assertExpression(h)
	}
	return err == nil, err
}
