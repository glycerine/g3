package qe

// Source: src/qe/qsat.cpp qsat::maximize_model/maximize_core.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// qsatOptimumString is inf_eps::to_string.
func qsatOptimumString(q *unumeral.RationalManager, v *simplex.Optimum) (string, error) {
	s, err := q.String(&v.Value)
	if err != nil {
		return "", err
	}
	epsilon, err := q.Sign(&v.Epsilon)
	if err != nil {
		return "", err
	}
	if epsilon != 0 {
		sign := " +e*"
		if epsilon < 0 {
			sign = " -e*"
		}
		// abs(m_infinitesimal) is a temporary of the expression.
		a := unumeral.NewRational()
		err = q.Copy(&a, &v.Epsilon)
		if err == nil {
			err = q.Abs(&a)
		}
		var text string
		if err == nil {
			text, err = q.String(&a)
		}
		if err = errors.Join(err, q.Del(&a)); err != nil {
			return "", err
		}
		s = "(" + s + sign + text + ")"
	}
	if v.Infinite {
		value, err := q.Sign(&v.Value)
		if err != nil {
			return "", err
		}
		if value == 0 && epsilon == 0 {
			return "oo", nil
		}
		return "(oo + " + s + ")", nil
	}
	return s, nil
}

func (s *qsatSearch) verboseOptimum(prefix string, v *simplex.Optimum) error {
	text, err := qsatOptimumString(s.manager.NumeralService().Manager, v)
	if err != nil {
		return err
	}
	return s.verbose(prefix + text + ")\n")
}
func (s *qsatSearch) maximizeCore(core *[]ast.ASTRef) error {
	if s.value == nil || s.objective.IsNull() {
		return ast.ErrASTHandle
	}
	if err := s.preparation.abstraction.emit("maximize_core", 1383, func(w io.Writer) error {
		_, _ = io.WriteString(w, "maximize: ")
		if e := writeExpressionVector(s.manager, s.config, w, *core); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	}); err != nil {
		return err
	}
	s.wasSat = s.wasSat || len(*core) > 0
	if err := s.value.Set(s.manager.NumeralService().Manager, &s.valueSaved); err != nil {
		return err
	}
	if err := s.verboseOptimum("(maximize ", s.value); err != nil {
		return err
	}
	h, err := s.manager.RefGet(s.improvement)
	if err != nil {
		return err
	}
	return s.assertBoth(h)
}

// maximizeModel is qsat::maximize_model; its locals (ge, asms, defs) end
// when it returns, not with the check_sat round.
func (s *qsatSearch) maximizeModel() error {
	c := s.call()
	if err := s.maximizeModelCore(c); err != nil {
		// Failed-call owners stay on pending for the outer cleanup receipt.
		return err
	}
	return s.closeRound(c)
}

func (s *qsatSearch) maximizeModelCore(c *qsatSearchCall) error {
	if s.level != 1 || s.mode != qsatMaximize || s.objective.IsNull() {
		return ast.ErrASTDomain
	}
	md, err := s.current.Get()
	if err != nil {
		return err
	}
	if md == nil {
		return ast.ErrASTHandle
	}
	ge, err := c.ref(ast.ASTHandle{})
	if err != nil {
		return err
	}
	asms, defs := c.vector(), c.vector()
	if err = s.preparation.abstraction.GetAssumptions(md, asms); err != nil {
		return err
	}
	if err = s.preparation.abstraction.PredicatesToLiterals(asms); err != nil {
		return err
	}
	if err = s.requireDefinitions("maximize_model1"); err != nil {
		return err
	}
	// m_value = m_mbp.maximize(...): the returned inf_eps is a temporary.
	q := s.manager.NumeralService().Manager
	v, err := s.mbp.Maximize(md, *asms, s.objective, ge, s.improvement)
	if err == nil {
		err = s.valueSaved.Set(q, &v)
	}
	if err = errors.Join(err, v.Del(q)); err != nil {
		return err
	}
	if err = s.requireDefinitions("maximize_model2"); err != nil {
		return err
	}
	if err = s.verboseOptimum("(qsat-maximize-bound: ", &s.valueSaved); err != nil {
		return err
	}
	h, err := s.manager.RefGet(ge)
	if err != nil {
		return err
	}
	level := EmptyMaxLevel()
	if err = s.preparation.abstraction.AbstractAtoms(h, &level, defs); err != nil {
		return err
	}
	// m_ex/m_fa.assert_expr(mk_and(defs)): the conjunction is a temporary
	// of each statement.
	for _, kernel := range []*qsatKernel{s.exists, s.forall} {
		d, e := c.conjunction(*defs)
		if e != nil {
			return e
		}
		if e = kernel.assertExpression(d); e != nil {
			return e
		}
		if e = c.releaseHeld(d); e != nil {
			return e
		}
	}
	if err = s.preparation.abstraction.MakeAbstract(h, ge); err != nil {
		return err
	}
	h, err = s.manager.RefGet(ge)
	if err != nil {
		return err
	}
	n, err := s.manager.GetNumArgs(h)
	if err != nil {
		return err
	}
	fid, err := s.manager.GetFamilyID(h)
	if err != nil {
		return err
	}
	isConstant := n == 0 && fid == ast.NullFamilyID
	if ast.ASTSourceDebug && !isConstant {
		return ast.ErrASTDomain
	}
	if isConstant {
		decl, err := s.manager.GetDecl(h)
		if err != nil {
			return err
		}
		truth, err := s.manager.MkTrue()
		if err != nil {
			return err
		}
		if err = md.RegisterConstant(decl, truth); err != nil {
			return err
		}
	}
	return s.requireDefinitions("maximize_model3")
}

// maximize follows the retained qsat optimization entry; qmax/factory and
// command/API integration remain separate consumers.
func (s *qsatSearch) maximize(forms []ast.ASTRef, objective ast.ASTHandle, md *tactic.ModelRef, value *simplex.Optimum) (status lbool.LBool, err error) {
	c, err := s.begin()
	if err != nil {
		return lbool.Undef, err
	}
	defer s.finish(c, &err)
	if s.mode != qsatMaximize || md == nil || value == nil {
		return lbool.Undef, ast.ErrASTHandle
	}
	fml, err := c.conjunction(forms)
	if err != nil {
		return lbool.Undef, err
	}
	r, err := c.ref(fml)
	if err != nil {
		return lbool.Undef, err
	}
	if err = s.preparation.hoist(s.mode, r); err != nil {
		return lbool.Undef, err
	}
	s.objective = objective
	s.value = value
	s.wasSat = false
	if err = s.saved.Set(nil); err != nil {
		return lbool.Undef, err
	}
	if err = s.prepareKernels(c, r); err != nil {
		return lbool.Undef, err
	}
	status, err = s.checkLoop()
	if err != nil {
		return lbool.Undef, err
	}
	if err = md.SetRef(&s.current); err != nil {
		return lbool.Undef, err
	}
	switch status {
	case lbool.False:
		if !s.wasSat {
			return lbool.False, nil
		}
		if err = md.SetRef(&s.saved); err != nil {
			return lbool.Undef, err
		}
	case lbool.True:
		return lbool.Undef, ast.ErrASTDomain
	case lbool.Undef:
		ex, e := s.exists.solver()
		if e != nil {
			return lbool.Undef, e
		}
		reason, e := ex.ReasonUnknown()
		if e != nil {
			return lbool.Undef, e
		}
		if reason == "ok" {
			fa, e := s.forall.solver()
			if e != nil {
				return lbool.Undef, e
			}
			reason, e = fa.ReasonUnknown()
			if e != nil {
				return lbool.Undef, e
			}
		}
		return lbool.Undef, tactic.NewTacticException(reason)
	}
	return lbool.True, nil
}
