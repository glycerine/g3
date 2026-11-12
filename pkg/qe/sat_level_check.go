package qe

// Source: src/qe/qe_sat_tactic.cpp, is_sat.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/messages"
)

type satLevelResult struct {
	manager *ast.ASTManager
	sat     bool
	// assignments borrows m_assignments, valid until the next check.
	assignments []ast.ASTHandle
	model       *model.ModelCore
}

// satAssignments is sat_tactic::m_assignments. It is a tactic member: each
// is_sat resets it, and only the tactic's destruction releases it.
type satAssignments struct {
	manager *ast.ASTManager
	refs    []ast.ASTRef
}

// reset is expr_ref_vector::reset.
func (a *satAssignments) reset() error {
	for i := range a.refs {
		if a.refs[i] != (ast.ASTRef{}) {
			if err := a.manager.DelRef(&a.refs[i]); err != nil {
				a.refs = a.refs[i:]
				return err
			}
		}
	}
	a.refs = a.refs[:0]
	return nil
}

func (s *satLevels) check(index uint32, context ast.ASTHandle) (result *satLevelResult, err error) {
	if s == nil || s.manager == nil {
		return nil, ast.ErrASTHandle
	}
	if index >= uint32(len(s.levels)) {
		return nil, arena.ErrBounds
	}
	result = &satLevelResult{manager: s.manager}
	defer func() {
		if err != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	solver := s.levels[index].solver
	if err = solver.Push(); err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, solver.Pop(1)) }()
	if err = solver.AssertFormula(context); err != nil {
		return result, err
	}
	status, err := solver.CheckSat(nil)
	if err != nil {
		return result, err
	}
	if err = s.assignments.reset(); err != nil {
		return result, err
	}
	assignments, err := solver.Assignments()
	if err != nil {
		return result, err
	}
	for _, h := range assignments {
		ref, e := s.manager.NewRef(h)
		if e != nil {
			return result, e
		}
		s.assignments.refs = append(s.assignments.refs, ref)
	}
	result.assignments = assignments
	if status == lbool.Undef {
		// check_success(r != l_undef) throws TACTIC_CANCELED_MSG.
		return result, tactic.NewTacticException(messages.Canceled)
	}
	result.sat = status == lbool.True
	if result.sat && index == 0 {
		result.model, err = solver.AcquireModel()
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

func (r *satLevelResult) Close() error {
	if r == nil || r.manager == nil {
		return nil
	}
	if r.model != nil {
		if err := r.model.DecRef(); err != nil {
			return err
		}
		r.model = nil
	}
	r.assignments = nil
	r.manager = nil
	return nil
}
